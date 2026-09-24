package activity

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

var testNow = time.Date(2026, time.July, 21, 18, 30, 0, 250_000_000, time.UTC)

type testAuth struct {
	principal ports.Principal
	err       error
}

func (f testAuth) Authenticate(context.Context, string) (ports.Principal, error) {
	return f.principal, f.err
}

type testProfile struct {
	zone  string
	err   error
	users *[]string
}

func (f testProfile) TimeZone(_ context.Context, userID string) (string, error) {
	if f.users != nil {
		*f.users = append(*f.users, userID)
	}
	return f.zone, f.err
}

type testAuthorizer struct {
	allowed bool
	err     error
	calls   *[]authCall
}

type authCall struct{ resourceType, resourceID, permission, userID string }

func (f testAuthorizer) Check(_ context.Context, resourceType, resourceID, permission, userID string) (bool, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, authCall{resourceType, resourceID, permission, userID})
	}
	return f.allowed, f.err
}
func (testAuthorizer) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (testAuthorizer) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}

type testRepository struct {
	startResult     StartTimerResult
	stopResult      StopTimerResult
	createResult    CreateManualActivityResult
	updateResult    UpdateActivityResult
	deleteResult    DeleteActivityResult
	timer           domain.RunningTimer
	activity        domain.RecordedActivity
	activityVersion int64
	revisions       []ActivityRevisionRecord
	activities      []ActivityListRecord
	activityHasMore bool
	revisionHasMore bool
	activityPages   *[]ActivityPageRequest
	revisionPages   *[]ActivityRevisionPageRequest
	startErr        error
	stopErr         error
	createErr       error
	updateErr       error
	deleteErr       error
	activityErr     error
	revisionsErr    error
	activitiesErr   error
	getErr          error
	total           int64
	totalErr        error
	starts          *[]StartTimerCommand
	stops           *[]StopTimerCommand
	creates         *[]CreateManualActivityCommand
	updates         *[]UpdateActivityCommand
	deletes         *[]DeleteActivityCommand
	gets            *[]struct{ userID, pathID string }
}

func (f testRepository) AccumulatedSeconds(context.Context, string, string) (int64, error) {
	return f.total, f.totalErr
}

func (f testRepository) StartTimer(_ context.Context, command StartTimerCommand) (StartTimerResult, error) {
	if f.starts != nil {
		*f.starts = append(*f.starts, command)
	}
	if f.startResult.Timer.ID == "" && !f.startResult.Replayed && f.startErr == nil {
		return StartTimerResult{Timer: command.Timer}, nil
	}
	return f.startResult, f.startErr
}
func (f testRepository) StopTimer(_ context.Context, command StopTimerCommand) (StopTimerResult, error) {
	if f.stops != nil {
		*f.stops = append(*f.stops, command)
	}
	return f.stopResult, f.stopErr
}
func (f testRepository) CreateManualActivity(_ context.Context, command CreateManualActivityCommand) (CreateManualActivityResult, error) {
	if f.creates != nil {
		*f.creates = append(*f.creates, command)
	}
	if f.createResult.Activity.ID == "" && f.createErr == nil {
		return CreateManualActivityResult{Activity: command.Activity, Version: 1}, nil
	}
	return f.createResult, f.createErr
}
func (f testRepository) UpdateActivity(_ context.Context, command UpdateActivityCommand) (UpdateActivityResult, error) {
	if f.updates != nil {
		*f.updates = append(*f.updates, command)
	}
	return f.updateResult, f.updateErr
}
func (f testRepository) DeleteActivity(_ context.Context, command DeleteActivityCommand) (DeleteActivityResult, error) {
	if f.deletes != nil {
		*f.deletes = append(*f.deletes, command)
	}
	return f.deleteResult, f.deleteErr
}
func (f testRepository) GetActivity(context.Context, string, string, string) (domain.RecordedActivity, int64, error) {
	if f.activity == (domain.RecordedActivity{}) && f.updateResult.Revision.Activity != (domain.RecordedActivity{}) {
		return f.updateResult.Revision.Activity, 1, f.activityErr
	}
	if f.activity == (domain.RecordedActivity{}) && f.updates != nil {
		entry, _ := domain.RecordManualActivity(domain.ManualActivity{ID: "activity-1", PathID: "path-1", ParticipantID: "user-1", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC"}, testNow.Add(-time.Minute))
		return entry, 1, f.activityErr
	}
	return f.activity, f.activityVersion, f.activityErr
}
func (f testRepository) ListActivityRevisions(_ context.Context, _, _, _ string, page ActivityRevisionPageRequest) (ActivityRevisionPage, error) {
	if f.revisionPages != nil {
		*f.revisionPages = append(*f.revisionPages, page)
	}
	return ActivityRevisionPage{Items: f.revisions, HasMore: f.revisionHasMore}, f.revisionsErr
}
func (f testRepository) ListActivities(_ context.Context, _, _ string, page ActivityPageRequest) (ActivityPage, error) {
	if f.activityPages != nil {
		*f.activityPages = append(*f.activityPages, page)
	}
	return ActivityPage{Items: f.activities, HasMore: f.activityHasMore}, f.activitiesErr
}
func (f testRepository) GetRunningTimer(_ context.Context, userID, pathID string) (domain.RunningTimer, error) {
	if f.gets != nil {
		*f.gets = append(*f.gets, struct{ userID, pathID string }{userID, pathID})
	}
	return f.timer, f.getErr
}

type testAudits struct {
	events *[]audit.Event
	err    error
}

func (f testAudits) AppendAuditEvent(_ context.Context, event audit.Event) error {
	if f.events != nil {
		*f.events = append(*f.events, event)
	}
	return f.err
}
func (testAudits) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type testLimiter struct{ allowed bool }

func (f testLimiter) Allow(string, time.Time) bool { return f.allowed }

type testClock struct{ now time.Time }

func (f testClock) Now() time.Time { return f.now }

func testService(repository Repository) *Service {
	ids := []string{"timer-1", "audit-1", "activity-1", "audit-2", "audit-3"}
	return New(Dependencies{
		Auth:     testAuth{principal: ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}}},
		Profiles: testProfile{zone: "America/New_York"}, Authorizer: testAuthorizer{allowed: true},
		Repository: repository, Audits: testAudits{}, AuditRateLimiter: testLimiter{allowed: true}, Clock: testClock{now: testNow},
		CursorSigningKey: []byte("0123456789abcdef0123456789abcdef"),
		NewID:            func() string { id := ids[0]; ids = ids[1:]; return id },
	})
}

func TestStartTimerAuthorizesTrackingAndPersistsCanonicalTimerWithAtomicAudit(t *testing.T) {
	var starts []StartTimerCommand
	var authorization []authCall
	var profileUsers []string
	service := testService(testRepository{starts: &starts})
	service.Authorizer = testAuthorizer{allowed: true, calls: &authorization}
	service.Profiles = testProfile{zone: "America/New_York", users: &profileUsers}

	result, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	if err != nil {
		t.Fatal(err)
	}
	if len(authorization) != 1 || authorization[0] != (authCall{"path", "path-1", "track", "user-1"}) {
		t.Fatalf("authorization calls = %+v", authorization)
	}
	if len(profileUsers) != 1 || profileUsers[0] != "user-1" || len(starts) != 1 {
		t.Fatalf("profile users = %v, starts = %d", profileUsers, len(starts))
	}
	command := starts[0]
	if result.Timer != command.Timer || command.Timer.StartedAt != testNow || command.Timer.OccurrenceTimeZone != "America/New_York" {
		t.Fatalf("timer = %+v, command = %+v", result.Timer, command.Timer)
	}
	if command.Idempotency.PrincipalID != "user-1" || command.Idempotency.Operation != StartTimerOperation || command.Idempotency.Key != "start-request-0001" || len(command.Idempotency.RequestHash) != sha256Size {
		t.Fatalf("idempotency = %+v", command.Idempotency)
	}
	if !command.Audit.Valid() || command.Audit.Action != audit.ActivityTimerStarted || command.Audit.TargetID != "timer-1" {
		t.Fatalf("audit = %+v", command.Audit)
	}
}

func TestStartTimerCanonicalizesMutationInstantToDurablePrecision(t *testing.T) {
	var starts []StartTimerCommand
	service := testService(testRepository{starts: &starts})
	service.Clock = testClock{now: testNow.Add(123 * time.Nanosecond)}

	result, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	if err != nil {
		t.Fatal(err)
	}
	want := testNow.Truncate(time.Microsecond)
	if len(starts) != 1 || starts[0].Timer.StartedAt != want || result.Timer.StartedAt != want {
		t.Fatalf("mutation instants = command %v, result %v; want %v", starts[0].Timer.StartedAt, result.Timer.StartedAt, want)
	}
}

const sha256Size = 32

func TestStartTimerRequestHashExcludesGeneratedIdentityAndClock(t *testing.T) {
	var first, second []StartTimerCommand
	serviceA := testService(testRepository{starts: &first})
	serviceB := testService(testRepository{starts: &second})
	serviceB.Clock = testClock{now: testNow.Add(time.Hour)}
	serviceB.NewID = func() string { return "different-id" }
	_, _ = serviceA.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	_, _ = serviceB.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	if len(first) != 1 || len(second) != 1 || !bytes.Equal(first[0].Idempotency.RequestHash, second[0].Idempotency.RequestHash) {
		t.Fatal("same start request did not retain a stable request hash")
	}
	if bytes.Equal(first[0].Idempotency.RequestHash, requestHash(StartTimerOperation, "path-2")) {
		t.Fatal("request hash did not bind the Path")
	}
}

func TestStartConflictSurfacesExistingTimerWithoutReset(t *testing.T) {
	existing, err := domain.StartTimer("existing", "path-1", "user-1", testNow.Add(-time.Hour), "Etc/UTC", testNow)
	if err != nil {
		t.Fatal(err)
	}
	var events []audit.Event
	service := testService(testRepository{startResult: StartTimerResult{Timer: existing, AccumulatedSeconds: 125}, startErr: ports.ErrConflict})
	service.Audits = testAudits{events: &events}
	result, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	if err != nil || result.Timer != existing || result.AccumulatedSeconds != 125 {
		t.Fatalf("StartTimer() = (%+v, %v), want existing timer state", result, err)
	}
	if len(events) != 0 {
		t.Fatalf("service duplicated repository-owned conflict audit: %+v", events)
	}
}

func TestDelayedStartReplayPreservesStoppedCurrentState(t *testing.T) {
	service := testService(testRepository{startResult: StartTimerResult{AccumulatedSeconds: 90, Replayed: true}})
	result, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001")
	if err != nil || result.Timer != (domain.RunningTimer{}) || result.AccumulatedSeconds != 90 || !result.Replayed {
		t.Fatalf("StartTimer() = (%+v, %v), want stopped replay state", result, err)
	}
}

func TestCurrentTimerReturnsReloadStateAndAccumulatedTimeWithOneAudit(t *testing.T) {
	timer, _ := domain.StartTimer("timer-1", "path-1", "user-1", testNow.Add(-time.Minute), "Etc/UTC", testNow)
	var events []audit.Event
	service := testService(testRepository{timer: timer, total: 3661})
	service.Audits = testAudits{events: &events}
	result, err := service.CurrentTimer(context.Background(), "Bearer valid", "path-1")
	if err != nil || result.Timer == nil || *result.Timer != timer || result.AccumulatedSeconds != 3661 {
		t.Fatalf("CurrentTimer() = (%+v, %v)", result, err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceViewed || events[0].TargetID != timer.ID {
		t.Fatalf("audits = %+v", events)
	}
}

func TestCurrentTimerRepresentsNoRunningTimerWithoutTreatingItAsMissingPath(t *testing.T) {
	var events []audit.Event
	service := testService(testRepository{getErr: ports.ErrNotFound, total: 90})
	service.Audits = testAudits{events: &events}
	result, err := service.CurrentTimer(context.Background(), "Bearer valid", "path-1")
	if err != nil || result.Timer != nil || result.AccumulatedSeconds != 90 {
		t.Fatalf("CurrentTimer() = (%+v, %v)", result, err)
	}
	if len(events) != 1 || events[0].TargetType != "path" || events[0].TargetID != "path-1" {
		t.Fatalf("audits = %+v", events)
	}
}

func TestAccumulatedSecondsIsAuthorizedUserScopedAndAudited(t *testing.T) {
	var authorization []authCall
	var events []audit.Event
	service := testService(testRepository{total: 7265})
	service.Authorizer = testAuthorizer{allowed: true, calls: &authorization}
	service.Audits = testAudits{events: &events}
	total, err := service.AccumulatedSeconds(context.Background(), "Bearer valid", "path-1")
	if err != nil || total != 7265 {
		t.Fatalf("AccumulatedSeconds() = %d, %v", total, err)
	}
	if len(authorization) != 1 || authorization[0] != (authCall{"path", "path-1", "track", "user-1"}) {
		t.Fatalf("authorization = %+v", authorization)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceViewed || events[0].TargetType != "path" || events[0].TargetID != "path-1" {
		t.Fatalf("audits = %+v", events)
	}
}

func TestCreateManualActivityUsesParticipantLocalOccurrenceAndAtomicEvidence(t *testing.T) {
	var creates []CreateManualActivityCommand
	var authorization []authCall
	var profileUsers []string
	service := testService(testRepository{creates: &creates})
	service.Authorizer = testAuthorizer{allowed: true, calls: &authorization}
	service.Profiles = testProfile{zone: "America/New_York", users: &profileUsers}
	service.NewID = func() string {
		if len(creates) == 0 {
			return "activity-1"
		}
		return "audit-1"
	}

	result, err := service.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", ManualActivityInput{
		LocalDate: "2026-07-21", LocalStartTime: "13:00:00", DurationSeconds: 3600, Note: "Cafe\u0301 practice",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(authorization) != 1 || authorization[0] != (authCall{"path", "path-1", "track", "user-1"}) || len(profileUsers) != 1 || profileUsers[0] != "user-1" {
		t.Fatalf("authorization=%+v profileUsers=%v", authorization, profileUsers)
	}
	if len(creates) != 1 || result.Activity != creates[0].Activity || result.Version != 1 {
		t.Fatalf("result=%+v creates=%+v", result, creates)
	}
	command := creates[0]
	wantStart := time.Date(2026, time.July, 21, 17, 0, 0, 0, time.UTC)
	if command.Activity.StartedAt != wantStart || command.Activity.EndedAt != wantStart.Add(time.Hour) || command.Activity.Note != "Café practice" {
		t.Fatalf("activity=%+v", command.Activity)
	}
	if command.Idempotency.Operation != CreateManualActivityOperation || command.Idempotency.Key != "manual-request-001" || len(command.Idempotency.RequestHash) != sha256Size {
		t.Fatalf("idempotency=%+v", command.Idempotency)
	}
	if !command.Audit.Valid() || command.Audit.Action != audit.ResourceCreated || command.Audit.TargetType != "activity" || command.Audit.TargetID != "activity-1" {
		t.Fatalf("audit=%+v", command.Audit)
	}
}

func TestManualActivityDefaultsCarryAuthoritativeInstantAndParticipantTimeZone(t *testing.T) {
	service := testService(testRepository{})
	service.Clock = testClock{now: time.Date(2026, time.November, 1, 6, 30, 0, 123_456_789, time.UTC)}
	service.Profiles = testProfile{zone: "America/New_York"}

	defaults, err := service.ManualActivityDefaults(context.Background(), "Bearer valid", "path-1")
	if err != nil {
		t.Fatal(err)
	}
	wantInstant := time.Date(2026, time.November, 1, 6, 30, 0, 123_456_000, time.UTC)
	if defaults.CurrentInstant != wantInstant || defaults.TimeZone != "America/New_York" || defaults.LocalDate != "2026-11-01" || defaults.LocalStartTime != "01:30:00" {
		t.Fatalf("ManualActivityDefaults() = %+v", defaults)
	}
}

func TestCreateManualActivityRequestHashBindsCanonicalUserInput(t *testing.T) {
	var first, second []CreateManualActivityCommand
	input := ManualActivityInput{LocalDate: "2026-07-21", LocalStartTime: "13:00:00", DurationSeconds: 3600, Note: "practice"}
	serviceA := testService(testRepository{creates: &first})
	serviceB := testService(testRepository{creates: &second})
	serviceA.NewID = func() string { return "first-generated" }
	serviceB.NewID = func() string { return "second-generated" }
	_, _ = serviceA.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", input)
	_, _ = serviceB.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", input)
	if len(first) != 1 || len(second) != 1 || !bytes.Equal(first[0].Idempotency.RequestHash, second[0].Idempotency.RequestHash) {
		t.Fatal("same manual request did not retain a stable hash")
	}
	input.Note = "different"
	var changed []CreateManualActivityCommand
	serviceC := testService(testRepository{creates: &changed})
	serviceC.NewID = func() string { return "third-generated" }
	_, _ = serviceC.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", input)
	if len(changed) != 1 || bytes.Equal(first[0].Idempotency.RequestHash, changed[0].Idempotency.RequestHash) {
		t.Fatal("manual request hash did not bind the normalized note")
	}
}

func TestCreateManualActivityRequestHashIgnoresLaterProfileTimeZone(t *testing.T) {
	input := ManualActivityInput{LocalDate: "2026-07-21", LocalStartTime: "10:00:00", DurationSeconds: 60, Note: "Cafe\u0301"}
	var first, second []CreateManualActivityCommand
	serviceA := testService(testRepository{creates: &first})
	serviceB := testService(testRepository{creates: &second})
	serviceA.Profiles = testProfile{zone: "America/New_York"}
	serviceB.Profiles = testProfile{zone: "Europe/Paris"}
	_, _ = serviceA.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", input)
	_, _ = serviceB.CreateManualActivity(context.Background(), "Bearer valid", "path-1", "manual-request-001", input)
	if len(first) != 1 || len(second) != 1 || !bytes.Equal(first[0].Idempotency.RequestHash, second[0].Idempotency.RequestHash) {
		t.Fatal("profile timezone change altered the raw manual request hash")
	}
}

func TestUpdateActivityRequestHashUsesCanonicalNoteAndNotProfileTimeZone(t *testing.T) {
	inputs := []struct {
		zone string
		note string
	}{
		{zone: "America/New_York", note: "Cafe\u0301"},
		{zone: "Europe/Paris", note: "Café"},
	}
	var commands [2][]UpdateActivityCommand
	for index, input := range inputs {
		service := testService(testRepository{updates: &commands[index]})
		service.Profiles = testProfile{zone: input.zone}
		_, _ = service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", ManualActivityInput{
			LocalDate: "2026-07-21", LocalStartTime: "10:00:00", DurationSeconds: 60, Note: input.note,
		})
	}
	if len(commands[0]) != 1 || len(commands[1]) != 1 || !bytes.Equal(commands[0][0].Idempotency.RequestHash, commands[1][0].Idempotency.RequestHash) {
		t.Fatal("canonical-equivalent update changed its request hash")
	}

	var absent, blank []UpdateActivityCommand
	for note, destination := range map[string]*[]UpdateActivityCommand{"": &absent, " \r\n ": &blank} {
		service := testService(testRepository{updates: destination})
		_, _ = service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", ManualActivityInput{
			LocalDate: "2026-07-21", LocalStartTime: "10:00:00", DurationSeconds: 60, Note: note,
		})
	}
	if len(absent) != 1 || len(blank) != 1 || !bytes.Equal(absent[0].Idempotency.RequestHash, blank[0].Idempotency.RequestHash) {
		t.Fatal("blank-equivalent update note changed its request hash")
	}
}

func TestUpdateActivityReplayReturnsStoredEditAfterProfileTimeZoneChange(t *testing.T) {
	original, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-1",
		StartedAt: testNow.Add(-2 * time.Hour), DurationSeconds: 60,
		OccurrenceTimeZone: "America/New_York", Note: "before",
	}, testNow.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	storedEdit := domain.ActivityEdit{
		StartedAt: time.Date(2026, time.July, 21, 14, 0, 0, 0, time.UTC), DurationSeconds: 60,
		OccurrenceTimeZone: "America/New_York", Note: "Café",
	}
	stored, revision, err := original.EditByOwner("user-1", storedEdit, testNow)
	if err != nil {
		t.Fatal(err)
	}
	service := testService(testRepository{updateResult: UpdateActivityResult{
		Activity: stored, Revision: revision, Version: 2, AccumulatedSeconds: 60, Replayed: true,
	}})
	service.Profiles = testProfile{zone: "Europe/Paris"}
	result, err := service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", ManualActivityInput{
		LocalDate: "2026-07-21", LocalStartTime: "10:00:00", DurationSeconds: 60, Note: "Cafe\u0301",
	})
	if err != nil || result.Activity != stored || result.Revision != revision || !result.Replayed {
		t.Fatalf("timezone-changing replay = %+v, %v", result, err)
	}
}

func TestUpdateActivityIsOwnerScopedAndRetainsThePriorRevision(t *testing.T) {
	original, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-1",
		StartedAt: testNow.Add(-2 * time.Hour), DurationSeconds: 1800, OccurrenceTimeZone: "America/New_York", Note: "before",
	}, testNow.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	edit := domain.ActivityEdit{StartedAt: time.Date(2026, time.July, 21, 17, 0, 0, 0, time.UTC), DurationSeconds: 3600, OccurrenceTimeZone: "America/New_York", Note: "after"}
	edited, revision, err := original.EditByOwner("user-1", edit, testNow)
	if err != nil {
		t.Fatal(err)
	}
	var updates []UpdateActivityCommand
	var authorization []authCall
	var profileReads []string
	service := testService(testRepository{updates: &updates, updateResult: UpdateActivityResult{
		Activity: edited, Revision: revision, Version: 2, AccumulatedSeconds: 3600,
	}})
	service.Profiles = testProfile{zone: "Europe/Paris", users: &profileReads}
	service.Authorizer = testAuthorizer{allowed: true, calls: &authorization}

	result, err := service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", ManualActivityInput{
		LocalDate: "2026-07-21", LocalStartTime: "13:00:00", DurationSeconds: 3600, Note: "after",
	})
	if err != nil || result.Activity != edited || result.Revision != revision || result.Version != 2 || len(updates) != 1 || len(profileReads) != 0 {
		t.Fatalf("UpdateActivity() = (%+v, %v), updates=%+v", result, err, updates)
	}
	if len(authorization) != 1 || authorization[0] != (authCall{"path", "path-1", "track", "user-1"}) {
		t.Fatalf("authorization=%+v", authorization)
	}
	command := updates[0]
	if command.ParticipantID != "user-1" || command.ActivityID != "activity-1" || command.PathID != "path-1" || command.Edit != edit || command.UpdatedAt != testNow {
		t.Fatalf("command=%+v", command)
	}
	if command.Idempotency.Operation != UpdateActivityOperation || len(command.Idempotency.RequestHash) != sha256Size || !command.Audit.Valid() || command.Audit.Action != audit.ResourceUpdated || command.Audit.TargetType != "activity" {
		t.Fatalf("idempotency=%+v audit=%+v", command.Idempotency, command.Audit)
	}
}

func TestUpdateActivityRejectsCrossOwnerRepositoryProjection(t *testing.T) {
	other, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-2", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
	}, testNow.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	edited, revision, err := other.EditByOwner("user-2", domain.ActivityEdit{StartedAt: testNow.Add(-time.Hour), DurationSeconds: 120, OccurrenceTimeZone: "Etc/UTC"}, testNow)
	if err != nil {
		t.Fatal(err)
	}
	service := testService(testRepository{updateResult: UpdateActivityResult{Activity: edited, Revision: revision, Version: 2}})
	_, err = service.UpdateActivity(context.Background(), "Bearer valid", "path-1", "activity-1", "update-request-001", ManualActivityInput{
		LocalDate: "2026-07-21", LocalStartTime: "13:00:00", DurationSeconds: 3600,
	})
	if !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("cross-owner projection error=%v", err)
	}
}

func TestActivityDetailAndRevisionsUsePathViewAndRedactAnotherOwnerNote(t *testing.T) {
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-2", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
	}, testNow.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	revision := ActivityRevisionRecord{Version: 1, Revision: domain.ActivityRevision{Activity: entry, ReplacedAt: testNow}}
	var authorization []authCall
	var events []audit.Event
	service := testService(testRepository{activity: entry, activityVersion: 2, revisions: []ActivityRevisionRecord{revision}})
	service.Authorizer = testAuthorizer{allowed: true, calls: &authorization}
	service.Audits = testAudits{events: &events}
	got, version, err := service.GetActivity(context.Background(), "Bearer valid", "path-1", "activity-1")
	if err != nil || got != entry || version != 2 {
		t.Fatalf("GetActivity() = %+v, %d, %v", got, version, err)
	}
	history, _, err := service.ListActivityRevisions(context.Background(), "Bearer valid", "path-1", "activity-1", "", 25)
	if err != nil || len(history) != 1 || history[0] != revision {
		t.Fatalf("ListActivityRevisions() = %+v, %v", history, err)
	}
	if len(authorization) != 2 || authorization[0].permission != "view" || authorization[1].permission != "view" || len(events) != 2 || events[0].Action != audit.ResourceViewed || events[1].Action != audit.ResourceListed {
		t.Fatalf("authorization=%+v audits=%+v", authorization, events)
	}
}

func TestPathActivityHistoryUsesPathViewAndAuditsTheList(t *testing.T) {
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-1", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC", Note: "private",
	}, testNow.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	var authorization []authCall
	var events []audit.Event
	service := testService(testRepository{activities: []ActivityListRecord{{Activity: entry, Version: 2}}})
	service.Authorizer = testAuthorizer{allowed: true, calls: &authorization}
	service.Audits = testAudits{events: &events}
	history, _, err := service.ListActivities(context.Background(), "Bearer valid", "path-1", "", "", 25)
	if err != nil || len(history) != 1 || history[0].Activity != entry || history[0].Version != 2 {
		t.Fatalf("ListActivities() = %+v, %v", history, err)
	}
	if len(authorization) != 1 || authorization[0].permission != "view" || len(events) != 1 || events[0].Action != audit.ResourceListed || events[0].TargetID != "path-1" {
		t.Fatalf("authorization=%+v audits=%+v", authorization, events)
	}
}

func TestActivityDetailRejectsLeakedCrossOwnerNote(t *testing.T) {
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "user-2", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC", Note: "must stay private",
	}, testNow.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	service := testService(testRepository{activity: entry, activityVersion: 1})
	if _, _, err := service.GetActivity(context.Background(), "Bearer valid", "path-1", "activity-1"); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("leaked note error=%v", err)
	}
}

func TestActivityDetailAcceptsTimerCreatedFractionalInstants(t *testing.T) {
	startedAt := testNow.Add(-1500 * time.Millisecond)
	timer, err := domain.StartTimer("timer-1", "path-1", "user-1", startedAt, "America/New_York", startedAt)
	if err != nil {
		t.Fatal(err)
	}
	entry, saved, err := timer.Stop("activity-1", testNow, testNow)
	if err != nil || !saved || entry.DurationSeconds() != 1 || entry.EndedAt.Sub(entry.StartedAt) != 1500*time.Millisecond {
		t.Fatalf("timer Stop() = %+v, %v, %v", entry, saved, err)
	}
	service := testService(testRepository{activity: entry, activityVersion: 1})
	got, version, err := service.GetActivity(context.Background(), "Bearer valid", "path-1", "activity-1")
	if err != nil || got != entry || version != 1 {
		t.Fatalf("GetActivity() = %+v, %d, %v", got, version, err)
	}
}

func TestStopTimerUsesAuthenticatedParticipantAndStableRequestHash(t *testing.T) {
	rawNow := testNow.Add(123 * time.Nanosecond)
	durableNow := rawNow.Truncate(time.Microsecond)
	entry := domain.RecordedActivity{ID: "activity-1", PathID: "path-1", ParticipantID: "user-1", StartedAt: testNow.Add(-time.Minute), EndedAt: durableNow, OccurrenceTimeZone: "Etc/UTC", CreatedAt: durableNow, UpdatedAt: durableNow}
	var stops []StopTimerCommand
	service := testService(testRepository{stopResult: StopTimerResult{Activity: entry, Saved: true}, stops: &stops})
	service.Clock = testClock{now: rawNow}
	result, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-00001")
	if err != nil || !result.Saved || len(stops) != 1 {
		t.Fatalf("StopTimer() = (%+v, %v), stops=%d", result, err, len(stops))
	}
	command := stops[0]
	if command.ParticipantID != "user-1" || command.StoppedAt != durableNow || command.RecordedAt != durableNow || command.Idempotency.Operation != StopTimerOperation || command.Audit.Action != audit.ActivityTimerStopped {
		t.Fatalf("stop command = %+v", command)
	}
	wantHash := requestHash(StopTimerOperation, "path-1", "timer-1")
	if !bytes.Equal(command.Idempotency.RequestHash, wantHash) {
		t.Fatal("stop hash did not bind operation, Path, and timer")
	}
}

func TestStopTimerPreservesSubsecondNotSavedResult(t *testing.T) {
	service := testService(testRepository{stopResult: StopTimerResult{Saved: false, AccumulatedSeconds: 47}})
	result, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-00001")
	if err != nil || result.Saved || result.Activity != (domain.RecordedActivity{}) || result.AccumulatedSeconds != 47 || result.CurrentTimer != nil {
		t.Fatalf("StopTimer() = (%+v, %v), want saved=false", result, err)
	}
}

func TestDelayedStopReplayPreservesNewerCurrentTimer(t *testing.T) {
	current, _ := domain.StartTimer("timer-2", "path-1", "user-1", testNow, "Etc/UTC", testNow)
	service := testService(testRepository{stopResult: StopTimerResult{CurrentTimer: &current, Replayed: true}})
	result, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-00001")
	if err != nil || result.CurrentTimer == nil || *result.CurrentTimer != current || !result.Replayed {
		t.Fatalf("StopTimer() = (%+v, %v), want newer current timer", result, err)
	}
}

func TestGetRunningTimerIsAuthorizedUserScopedAndAudited(t *testing.T) {
	timer, _ := domain.StartTimer("timer-1", "path-1", "user-1", testNow, "Etc/UTC", testNow)
	var gets []struct{ userID, pathID string }
	var events []audit.Event
	service := testService(testRepository{timer: timer, gets: &gets})
	service.Audits = testAudits{events: &events}
	got, err := service.GetRunningTimer(context.Background(), "Bearer valid", "path-1")
	if err != nil || got != timer || len(gets) != 1 || gets[0].userID != "user-1" {
		t.Fatalf("GetRunningTimer() = (%+v, %v), gets=%+v", got, err, gets)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceViewed || events[0].TargetType != "timer" || events[0].TargetID != "timer-1" {
		t.Fatalf("read audit = %+v", events)
	}
}

func TestActivityServiceRequiresExactUserScopeBeforeDependencies(t *testing.T) {
	for name, principal := range map[string]ports.Principal{
		"missing": {UserID: "user-1"}, "onboarding": {UserID: "user-1", Scopes: []string{"api:onboarding"}},
		"extra": {UserID: "user-1", Scopes: []string{"api:user", "admin"}}, "no user": {Scopes: []string{"api:user"}},
	} {
		t.Run(name, func(t *testing.T) {
			service := New(Dependencies{Auth: testAuth{principal: principal}})
			if _, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001"); !errors.Is(err, platformapp.ErrUnauthenticated) {
				t.Fatalf("StartTimer() error = %v", err)
			}
		})
	}
}

func TestManualActivityBoundariesFailClosedForAuthenticationAuthorizationAndPolicyOutage(t *testing.T) {
	input := ManualActivityInput{LocalDate: "2026-07-21", LocalStartTime: "13:00:00", DurationSeconds: 60}
	operations := map[string]func(*Service) error{
		"defaults": func(service *Service) error {
			_, err := service.ManualActivityDefaults(context.Background(), "Bearer invalid", "path-1")
			return err
		},
		"create": func(service *Service) error {
			_, err := service.CreateManualActivity(context.Background(), "Bearer invalid", "path-1", "create-request-001", input)
			return err
		},
		"update": func(service *Service) error {
			_, err := service.UpdateActivity(context.Background(), "Bearer invalid", "path-1", "activity-1", "update-request-001", input)
			return err
		},
		"list": func(service *Service) error {
			_, _, err := service.ListActivities(context.Background(), "Bearer invalid", "path-1", "", "", 25)
			return err
		},
		"detail": func(service *Service) error {
			_, _, err := service.GetActivity(context.Background(), "Bearer invalid", "path-1", "activity-1")
			return err
		},
		"revisions": func(service *Service) error {
			_, _, err := service.ListActivityRevisions(context.Background(), "Bearer invalid", "path-1", "activity-1", "", 25)
			return err
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			unauthenticated := New(Dependencies{Auth: testAuth{err: ports.ErrInvalidCredential}})
			if err := operation(unauthenticated); !errors.Is(err, ports.ErrInvalidCredential) {
				t.Fatalf("authentication error = %v", err)
			}
			denied := testService(testRepository{})
			denied.Authorizer = testAuthorizer{allowed: false}
			if err := operation(denied); !errors.Is(err, platformapp.ErrForbidden) {
				t.Fatalf("authorization error = %v", err)
			}
			policyErr := errors.New("authorization dependency unavailable")
			outage := testService(testRepository{})
			outage.Authorizer = testAuthorizer{err: policyErr}
			if err := operation(outage); !errors.Is(err, policyErr) {
				t.Fatalf("policy outage error = %v", err)
			}
		})
	}
}

func TestActivityServiceFailsClosedBeforeRepository(t *testing.T) {
	dependencyErr := errors.New("SpiceDB unavailable")
	var starts []StartTimerCommand
	var stops []StopTimerCommand
	service := testService(testRepository{starts: &starts, stops: &stops})
	service.Authorizer = testAuthorizer{allowed: false}
	if _, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001"); !errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("denied StartTimer() error = %v", err)
	}
	if _, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-00001"); !errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("denied StopTimer() error = %v", err)
	}
	service.Authorizer = testAuthorizer{err: dependencyErr}
	if _, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001"); !errors.Is(err, dependencyErr) {
		t.Fatalf("failed authorizer error = %v", err)
	}
	if _, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-00001"); !errors.Is(err, dependencyErr) {
		t.Fatalf("failed stop authorizer error = %v", err)
	}
	service.Authorizer = testAuthorizer{allowed: true}
	service.Profiles = testProfile{err: dependencyErr}
	if _, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001"); !errors.Is(err, dependencyErr) {
		t.Fatalf("failed profile error = %v", err)
	}
	if len(starts) != 0 || len(stops) != 0 {
		t.Fatalf("repository called before authorization/profile succeeded: starts=%d stops=%d", len(starts), len(stops))
	}
}

func TestActivityServiceRejectsCrossUserRepositoryResults(t *testing.T) {
	otherTimer, err := domain.StartTimer("timer-2", "path-1", "other-user", testNow, "Etc/UTC", testNow)
	if err != nil {
		t.Fatal(err)
	}
	service := testService(testRepository{startResult: StartTimerResult{Timer: otherTimer, Replayed: true}})
	if _, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "start-request-0001"); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("cross-user start result error = %v", err)
	}

	service = testService(testRepository{timer: otherTimer})
	if _, err := service.GetRunningTimer(context.Background(), "Bearer valid", "path-1"); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("cross-user get result error = %v", err)
	}

	service = testService(testRepository{stopResult: StopTimerResult{CurrentTimer: &otherTimer, Replayed: true}})
	if _, err := service.StopTimer(context.Background(), "Bearer valid", "path-1", "timer-1", "stop-request-00001"); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("cross-user stop replay error = %v", err)
	}
}

func TestActivityServiceRejectsWeakIdempotencyKeysBeforeAuthorization(t *testing.T) {
	for _, key := range []string{"short", " leading-key-0001", "control-key-0001\n", string(bytes.Repeat([]byte{'x'}, 129))} {
		service := testService(testRepository{})
		if _, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", key); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("key %q error = %v", key, err)
		}
	}
}
