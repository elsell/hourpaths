package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledAuthenticator struct {
	principal ports.Principal
	err       error
}

func (f controlledAuthenticator) Authenticate(_ context.Context, authorization string) (ports.Principal, error) {
	if f.err != nil {
		return ports.Principal{}, f.err
	}
	if authorization != "Bearer valid" {
		return ports.Principal{}, ports.ErrInvalidCredential
	}
	return f.principal, nil
}

type controlledAuthorizer struct {
	allowed    bool
	err        error
	calls      *[]authorizationCall
	writes     *[]relationshipWrite
	deletes    *[]relationshipWrite
	operations *[]string
	batches    *[][]ports.RelationshipUpdate
}

type authorizationCall struct{ resourceType, resourceID, permission, userID string }
type relationshipWrite struct{ resourceType, resourceID, relation, subjectType, subjectID string }

func (f controlledAuthorizer) Check(_ context.Context, resourceType, resourceID, permission, userID string) (bool, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, authorizationCall{resourceType, resourceID, permission, userID})
	}
	return f.allowed, f.err
}
func (f controlledAuthorizer) WriteRelationship(_ context.Context, resourceType, resourceID, relation, subjectType, subjectID string) error {
	if f.operations != nil {
		*f.operations = append(*f.operations, "touch:"+relation+":"+subjectID)
	}
	if f.writes != nil {
		*f.writes = append(*f.writes, relationshipWrite{resourceType, resourceID, relation, subjectType, subjectID})
	}
	return f.err
}
func (f controlledAuthorizer) DeleteRelationship(_ context.Context, resourceType, resourceID, relation, subjectType, subjectID string) error {
	if f.operations != nil {
		*f.operations = append(*f.operations, "delete:"+relation+":"+subjectID)
	}
	if f.deletes != nil {
		*f.deletes = append(*f.deletes, relationshipWrite{resourceType, resourceID, relation, subjectType, subjectID})
	}
	return f.err
}
func (f controlledAuthorizer) WriteRelationships(_ context.Context, updates []ports.RelationshipUpdate) error {
	if f.batches != nil {
		copyOfUpdates := append([]ports.RelationshipUpdate(nil), updates...)
		*f.batches = append(*f.batches, copyOfUpdates)
	}
	return f.err
}

type controlledRepository struct {
	entity            domain.Entity
	page              Page
	err               error
	replayed          bool
	creates           *[]repositoryCreate
	gets              *[]repositoryGet
	lists             *[]repositoryList
	updates           *[]repositoryUpdate
	renames           *[]RenameCommand
	renameResult      RenameResult
	visibilityChanges *[]SetVisibilityCommand
	visibilityResult  SetVisibilityResult
	visibilityReplay  *SetVisibilityResult
	goalUpdates       *[]UpdateGoalsCommand
	goalResult        UpdateGoalsResult
	archiveChanges    *[]SetArchiveStateCommand
	archiveResult     SetArchiveStateResult
}
type repositoryCreate struct {
	entity      domain.Entity
	change      ports.AuthorizationChange
	idempotency ports.Idempotency
	event       audit.Event
}

type repositoryGet struct {
	userID string
	id     domain.ID
}

type repositoryList struct {
	userID string
	page   PageRequest
}

type repositoryUpdate struct {
	owner  string
	entity domain.Entity
	event  audit.Event
}

func (f controlledRepository) Create(_ context.Context, entity domain.Entity, change ports.AuthorizationChange, idempotency ports.Idempotency, event audit.Event) (domain.Entity, bool, error) {
	if f.creates != nil {
		*f.creates = append(*f.creates, repositoryCreate{entity: entity, change: change, idempotency: idempotency, event: event})
	}
	if f.err != nil {
		return domain.Entity{}, false, f.err
	}
	if f.replayed {
		return f.entity, true, nil
	}
	return entity, false, nil
}
func (f controlledRepository) List(_ context.Context, userID string, page PageRequest) (Page, error) {
	if f.lists != nil {
		*f.lists = append(*f.lists, repositoryList{userID: userID, page: page})
	}
	return f.page, f.err
}
func (f controlledRepository) Get(_ context.Context, userID string, id domain.ID) (domain.Entity, error) {
	if f.gets != nil {
		*f.gets = append(*f.gets, repositoryGet{userID: userID, id: id})
	}
	return f.entity, f.err
}
func (f controlledRepository) Update(_ context.Context, owner string, entity domain.Entity, event audit.Event) error {
	if f.updates != nil {
		*f.updates = append(*f.updates, repositoryUpdate{owner: owner, entity: entity, event: event})
	}
	return f.err
}
func (f controlledRepository) Rename(_ context.Context, command RenameCommand) (RenameResult, error) {
	if f.renames != nil {
		*f.renames = append(*f.renames, command)
	}
	return f.renameResult, f.err
}
func (f controlledRepository) SetVisibility(_ context.Context, command SetVisibilityCommand) (SetVisibilityResult, error) {
	if f.visibilityChanges != nil {
		*f.visibilityChanges = append(*f.visibilityChanges, command)
	}
	return f.visibilityResult, f.err
}
func (f controlledRepository) SetVisibilityReplay(context.Context, string, domain.ID, ports.Idempotency) (*SetVisibilityResult, error) {
	return f.visibilityReplay, f.err
}
func (f controlledRepository) UpdateGoals(_ context.Context, command UpdateGoalsCommand) (UpdateGoalsResult, error) {
	if f.goalUpdates != nil {
		*f.goalUpdates = append(*f.goalUpdates, command)
	}
	return f.goalResult, f.err
}

type controlledAudits struct {
	events *[]audit.Event
	err    error
}

func (f controlledAudits) AppendAuditEvent(_ context.Context, event audit.Event) error {
	if f.events != nil {
		*f.events = append(*f.events, event)
	}
	return f.err
}
func (controlledAudits) ListAuditEvents(context.Context, string, ports.PageRequest) (audit.Page, error) {
	return audit.Page{}, nil
}

type controlledLimiter struct{ denied bool }

func (f controlledLimiter) Allow(string, time.Time) bool { return !f.denied }

type controlledClock struct{ now time.Time }

func (f controlledClock) Now() time.Time { return f.now }

type controlledProfiles struct {
	visibility     identity.ProfileVisibility
	firstDayOfWeek identity.FirstDayOfWeek
	timeZone       string
	err            error
	calls          *[]string
	timeZoneCalls  *[]string
}

func (f controlledProfiles) PathCreationProfile(_ context.Context, userID string) (PathCreationProfile, error) {
	if f.calls != nil {
		*f.calls = append(*f.calls, userID)
	}
	return PathCreationProfile{ProfileVisibility: f.visibility, FirstDayOfWeek: f.firstDayOfWeek}, f.err
}

func (f controlledProfiles) TimeZone(_ context.Context, userID string) (string, error) {
	if f.timeZoneCalls != nil {
		*f.timeZoneCalls = append(*f.timeZoneCalls, userID)
	}
	return f.timeZone, f.err
}

type controlledOutbox struct {
	change      ports.AuthorizationChange
	changes     *[]ports.AuthorizationChange
	claimIndex  *int
	claimErr    error
	renewErr    error
	completeErr error
	failErr     error
	claims      *[]string
	renewed     *bool
	completed   *audit.Event
	failed      *bool
}

func (f controlledOutbox) ClaimAuthorizationChanges(context.Context, string, time.Duration, int) ([]ports.AuthorizationChange, error) {
	return nil, nil
}
func (f controlledOutbox) ClaimAuthorizationChange(context.Context, string, string, time.Duration) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, ports.ErrNotFound
}
func (f controlledOutbox) ClaimAuthorizationChangeForResource(_ context.Context, resourceType, resourceID, _ string, _ time.Duration) (ports.AuthorizationChange, error) {
	if f.claims != nil {
		*f.claims = append(*f.claims, resourceType+"/"+resourceID)
	}
	if f.change.ID == "" && f.claimErr == nil {
		if f.changes != nil && f.claimIndex != nil && *f.claimIndex < len(*f.changes) {
			change := (*f.changes)[*f.claimIndex]
			(*f.claimIndex)++
			return change, nil
		}
		return ports.AuthorizationChange{}, ports.ErrNotFound
	}
	return f.change, f.claimErr
}
func (f controlledOutbox) RenewAuthorizationChange(context.Context, string, string, time.Duration) error {
	if f.renewed != nil {
		*f.renewed = true
	}
	return f.renewErr
}
func (f controlledOutbox) CompleteAuthorizationChangeWithAudit(_ context.Context, _ string, _ string, event audit.Event) error {
	if f.completed != nil {
		*f.completed = event
	}
	return f.completeErr
}
func (f controlledOutbox) FailAuthorizationChange(context.Context, string, string, int, string) (bool, error) {
	if f.failed != nil {
		*f.failed = true
	}
	return false, f.failErr
}
func (controlledOutbox) ListAuthorizationDeadLetters(context.Context, string, ports.PageRequest) (ports.AuthorizationDeadLetterPage, error) {
	return ports.AuthorizationDeadLetterPage{}, nil
}
func (controlledOutbox) RequeueAuthorizationDeadLetter(context.Context, string, string, string, time.Duration, audit.Event) (ports.AuthorizationChange, error) {
	return ports.AuthorizationChange{}, ports.ErrNotFound
}

type controlledSerializer struct{ err error }

func (f controlledSerializer) WithinResource(ctx context.Context, _, _ string, operation func(context.Context) error) error {
	if f.err != nil {
		return f.err
	}
	return operation(ctx)
}

func configuredDependencies() Dependencies {
	return Dependencies{
		Auth:                    controlledAuthenticator{principal: ports.Principal{UserID: "member", Scopes: []string{"api:user"}}},
		Profiles:                controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: identity.FirstDayMonday},
		Authorizer:              controlledAuthorizer{allowed: true},
		AuthorizationOutbox:     controlledOutbox{},
		AuthorizationSerializer: controlledSerializer{},
		Repository:              controlledRepository{entity: domain.Entity{ID: "path-id", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Guitar", Visibility: "private"}}},
		Audits:                  controlledAudits{},
		AuditRateLimiter:        controlledLimiter{},
		Clock:                   controlledClock{now: time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)},
		NewID:                   sequentialIDs("path-id", "change-id"),
		AuthorizationWorker:     "path-api",
		AuthorizationLease:      time.Minute,
		CursorSigningKey:        []byte("0123456789abcdef0123456789abcdef"),
	}
}

func TestCreatePathDefaultsVisibilityAndAtomicallyCreatesCreatorRelationshipAndAudit(t *testing.T) {
	var creates []repositoryCreate
	var writes []relationshipWrite
	var completed audit.Event
	renewed := false
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{creates: &creates}
	dependencies.Authorizer = controlledAuthorizer{writes: &writes}
	dependencies.AuthorizationOutbox = controlledOutbox{renewed: &renewed, completed: &completed}

	created, err := New(dependencies).Create(platformapp.WithCorrelationID(context.Background(), "request-id"), "Bearer valid", "request-key-0001", domain.Attributes{Name: "  Guitar practice  "})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "path-id" || created.OwnerUserID != "member" || created.Name != "Guitar practice" || created.Visibility != "public" || !created.CreatedAt.Equal(dependencies.Clock.Now()) || !created.UpdatedAt.Equal(dependencies.Clock.Now()) {
		t.Fatalf("created path = %+v", created)
	}
	if len(creates) != 1 {
		t.Fatalf("repository creates = %+v", creates)
	}
	request := creates[0]
	if request.entity != created {
		t.Fatalf("persisted entity = %+v", request.entity)
	}
	if request.change.ID != "change-id" || request.change.ResourceType != "path" || request.change.ResourceID != "path-id" || request.change.Relation != "creator" || request.change.SubjectType != "user" || request.change.SubjectID != "member" || request.change.OwnerUserID != "member" || request.change.ActorUserID != "member" || request.change.Operation != ports.AuthorizationTouch || request.change.LockedBy != "path-api" || request.change.Lease != time.Minute {
		t.Fatalf("creator authorization change = %+v", request.change)
	}
	if request.idempotency.PrincipalID != "member" || request.idempotency.Operation != "path.create" || request.idempotency.Key != "request-key-0001" || len(request.idempotency.RequestHash) != 32 {
		t.Fatalf("idempotency = %+v", request.idempotency)
	}
	if request.event.Action != audit.ResourceCreated || request.event.OwnerUserID != "member" || request.event.ActorUserID != "member" || request.event.TargetType != "path" || request.event.TargetID != "path-id" || request.event.Outcome != audit.Succeeded || request.event.CorrelationID != "request-id" || !request.event.OccurredAt.Equal(dependencies.Clock.Now()) {
		t.Fatalf("atomic create audit = %+v", request.event)
	}
	if len(writes) != 1 || writes[0] != (relationshipWrite{"path", "path-id", "creator", "user", "member"}) || !renewed {
		t.Fatalf("authorization reconciliation: writes=%+v renewed=%v", writes, renewed)
	}
	if completed.Action != audit.AuthorizationApplied || completed.TargetType != "path" || completed.TargetID != "path-id" || completed.OwnerUserID != "member" || completed.ActorUserID != "member" {
		t.Fatalf("authorization completion audit = %+v", completed)
	}
}

func TestCreatePathEnforcesProfileBoundedVisibility(t *testing.T) {
	tests := []struct {
		name      string
		profile   identity.ProfileVisibility
		requested string
		want      string
		wantErr   error
	}{
		{"public defaults public", identity.ProfileVisibilityPublic, "", "public", nil},
		{"public allows followers", identity.ProfileVisibilityPublic, "followers", "followers", nil},
		{"public allows private", identity.ProfileVisibilityPublic, "private", "private", nil},
		{"private defaults followers", identity.ProfileVisibilityPrivate, "", "followers", nil},
		{"private allows private", identity.ProfileVisibilityPrivate, "private", "private", nil},
		{"private rejects public", identity.ProfileVisibilityPrivate, "public", "", ports.ErrInvalidArgument},
		{"rejects unknown path visibility", identity.ProfileVisibilityPublic, "friends", "", ports.ErrInvalidArgument},
		{"rejects absent profile visibility", "", "private", "", errInvalidPathDependencies},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var creates []repositoryCreate
			dependencies := configuredDependencies()
			dependencies.Profiles = controlledProfiles{visibility: test.profile, firstDayOfWeek: identity.FirstDayMonday}
			dependencies.Repository = controlledRepository{creates: &creates}
			created, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read", Visibility: test.requested})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && created.Visibility != test.want {
				t.Fatalf("visibility = %q, want %q", created.Visibility, test.want)
			}
			if test.wantErr != nil && len(creates) != 0 {
				t.Fatalf("invalid visibility reached repository: %+v", creates)
			}
		})
	}
}

func TestCreatePathRejectsInvalidProfileWeekdayBeforePersistence(t *testing.T) {
	for _, firstDayOfWeek := range []identity.FirstDayOfWeek{0, 8} {
		var creates []repositoryCreate
		dependencies := configuredDependencies()
		dependencies.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: firstDayOfWeek}
		dependencies.Repository = controlledRepository{creates: &creates}
		if _, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"}); !errors.Is(err, errInvalidPathDependencies) {
			t.Fatalf("first day %d returned %v, want invalid dependencies", firstDayOfWeek, err)
		}
		if len(creates) != 0 {
			t.Fatalf("invalid first day %d reached repository: %+v", firstDayOfWeek, creates)
		}
	}
}

func TestCreatePathRejectsInvalidSessionAndIdempotencyBeforeSideEffects(t *testing.T) {
	for _, authorization := range []string{"", "Bearer malformed", "Bearer oidc.header.payload"} {
		var profiles []string
		var creates []repositoryCreate
		dependencies := configuredDependencies()
		dependencies.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: identity.FirstDayMonday, calls: &profiles}
		dependencies.Repository = controlledRepository{creates: &creates}
		_, err := New(dependencies).Create(context.Background(), authorization, "request-key-0001", domain.Attributes{Name: "Read"})
		if !errors.Is(err, ports.ErrInvalidCredential) || len(profiles) != 0 || len(creates) != 0 {
			t.Fatalf("invalid session result: err=%v profiles=%+v creates=%+v", err, profiles, creates)
		}
	}
	for _, key := range []string{"short", "request-key-0001\n", "réquest-key-0001", string(make([]byte, 129))} {
		var profiles []string
		dependencies := configuredDependencies()
		dependencies.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: identity.FirstDayMonday, calls: &profiles}
		_, err := New(dependencies).Create(context.Background(), "Bearer valid", key, domain.Attributes{Name: "Read"})
		if !errors.Is(err, ports.ErrInvalidArgument) || len(profiles) != 0 {
			t.Fatalf("key %q result: err=%v profiles=%+v", key, err, profiles)
		}
	}
	for _, scopes := range [][]string{{"api:onboarding"}, {"api:user", "api:onboarding"}, nil} {
		dependencies := configuredDependencies()
		dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "member", Scopes: scopes}}
		if _, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"}); !errors.Is(err, platformapp.ErrUnauthenticated) {
			t.Fatalf("scopes %v returned %v", scopes, err)
		}
	}
}

func TestCreatePathReplayReturnsStoredEntityAndReconcilesOnlyPendingAuthorization(t *testing.T) {
	stored := domain.Entity{ID: "original-path", OwnerUserID: "member", Attributes: domain.Attributes{Name: "Read", Visibility: "followers"}, CreatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)}
	change := ports.AuthorizationChange{ID: "original-change", ResourceType: "path", ResourceID: "original-path", Relation: "creator", SubjectType: "user", SubjectID: "member", OwnerUserID: "member", ActorUserID: "member", Operation: ports.AuthorizationTouch}
	for _, test := range []struct {
		name       string
		claimErr   error
		wantErr    error
		wantWrites int
	}{
		{"already applied", ports.ErrNotFound, nil, 0},
		{"pending relationship", nil, nil, 1},
		{"concurrent claim", ports.ErrAuthorizationPending, ports.ErrAuthorizationPending, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			var claims []string
			var writes []relationshipWrite
			dependencies := configuredDependencies()
			dependencies.Repository = controlledRepository{entity: stored, replayed: true}
			dependencies.Authorizer = controlledAuthorizer{writes: &writes}
			dependencies.AuthorizationOutbox = controlledOutbox{change: change, claimErr: test.claimErr, claims: &claims}
			created, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"})
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, test.wantErr)
			}
			if test.wantErr == nil && created != stored {
				t.Fatalf("replay = %+v, want %+v", created, stored)
			}
			if len(claims) != 1 || claims[0] != "path/original-path" || len(writes) != test.wantWrites {
				t.Fatalf("reconciliation claims=%+v writes=%+v", claims, writes)
			}
		})
	}
}

func TestCreatePathDefaultVisibilityDoesNotDestabilizeIdempotencyHash(t *testing.T) {
	var publicCreates []repositoryCreate
	publicDependencies := configuredDependencies()
	publicDependencies.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPublic, firstDayOfWeek: identity.FirstDayMonday}
	publicDependencies.Repository = controlledRepository{creates: &publicCreates}
	if _, err := New(publicDependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: " Read "}); err != nil {
		t.Fatal(err)
	}

	var privateCreates []repositoryCreate
	privateDependencies := configuredDependencies()
	privateDependencies.Profiles = controlledProfiles{visibility: identity.ProfileVisibilityPrivate, firstDayOfWeek: identity.FirstDayMonday}
	privateDependencies.Repository = controlledRepository{creates: &privateCreates}
	if _, err := New(privateDependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"}); err != nil {
		t.Fatal(err)
	}
	if len(publicCreates) != 1 || len(privateCreates) != 1 || string(publicCreates[0].idempotency.RequestHash) != string(privateCreates[0].idempotency.RequestHash) {
		t.Fatalf("default visibility changed request hash: public=%x private=%x", publicCreates[0].idempotency.RequestHash, privateCreates[0].idempotency.RequestHash)
	}
}
func TestCreatePathRejectsMalformedReplayedCreatorChange(t *testing.T) {
	stored := domain.Entity{ID: "original-path", OwnerUserID: "member", Attributes: domain.Attributes{Name: "Read", Visibility: "private"}, CreatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)}
	malformed := ports.AuthorizationChange{ID: "wrong-change", ResourceType: "path", ResourceID: "different-path", Relation: "creator", SubjectType: "user", SubjectID: "other", OwnerUserID: "other", ActorUserID: "other", Operation: ports.AuthorizationTouch}
	var writes []relationshipWrite
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{entity: stored, replayed: true}
	dependencies.AuthorizationOutbox = controlledOutbox{change: malformed}
	dependencies.Authorizer = controlledAuthorizer{writes: &writes}
	if _, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"}); !errors.Is(err, errInvalidPathDependencies) {
		t.Fatalf("malformed replay returned %v", err)
	}
	if len(writes) != 0 {
		t.Fatalf("malformed replay wrote relationship: %+v", writes)
	}
}

func TestCreatePathAuthorizationFailureIsAuditedAndRemainsRetryable(t *testing.T) {
	failure := errors.New("spicedb unavailable")
	failed := false
	var events []audit.Event
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{err: failure}
	dependencies.AuthorizationOutbox = controlledOutbox{failed: &failed}
	dependencies.Audits = controlledAudits{events: &events}
	if _, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"}); !errors.Is(err, failure) {
		t.Fatalf("authorization failure returned %v", err)
	}
	if !failed || len(events) != 1 || events[0].Action != audit.AuthorizationFailed || events[0].TargetType != "path" || events[0].TargetID != "path-id" || events[0].Outcome != audit.Failed {
		t.Fatalf("retry/audit state: failed=%v events=%+v", failed, events)
	}
}

func TestCreatePathFailsClosedOnDependencyFailures(t *testing.T) {
	failure := errors.New("dependency unavailable")
	tests := []struct {
		name string
		set  func(*Dependencies)
	}{
		{"profile", func(d *Dependencies) { d.Profiles = controlledProfiles{err: failure} }},
		{"repository", func(d *Dependencies) { d.Repository = controlledRepository{err: failure} }},
		{"authorization", func(d *Dependencies) {
			d.Authorizer = controlledAuthorizer{err: failure}
			d.AuthorizationOutbox = controlledOutbox{failed: new(bool)}
		}},
		{"serializer", func(d *Dependencies) { d.AuthorizationSerializer = controlledSerializer{err: failure} }},
		{"completion", func(d *Dependencies) { d.AuthorizationOutbox = controlledOutbox{completeErr: failure} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dependencies := configuredDependencies()
			test.set(&dependencies)
			if _, err := New(dependencies).Create(context.Background(), "Bearer valid", "request-key-0001", domain.Attributes{Name: "Read"}); !errors.Is(err, failure) {
				t.Fatalf("dependency failure returned %v", err)
			}
		})
	}
}

func TestListPathsReturnsAuditedOwnerBoundPage(t *testing.T) {
	var checks []authorizationCall
	var lists []repositoryList
	var events []audit.Event
	created := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	dependencies := configuredDependencies()
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.Repository = controlledRepository{page: Page{
		Items: []domain.Entity{
			{ID: "first", OwnerUserID: "member", CreatedAt: created},
			{ID: "second", OwnerUserID: "creator", CreatedAt: created.Add(time.Hour)},
		},
		HasMore: true,
	}, lists: &lists}
	dependencies.Audits = controlledAudits{events: &events}

	items, cursor, err := New(dependencies).List(platformapp.WithCorrelationID(context.Background(), "request-id"), "Bearer valid", "", 0)
	if err != nil || len(items) != 2 || cursor == "" {
		t.Fatalf("List() = %+v, %q, %v", items, cursor, err)
	}
	if len(lists) != 1 || lists[0].userID != "member" || lists[0].page.Limit != 25 || !lists[0].page.Snapshot.Equal(dependencies.Clock.Now()) {
		t.Fatalf("repository list = %+v", lists)
	}
	if len(checks) != 2 || checks[0] != (authorizationCall{"path", "first", "view", "member"}) || checks[1] != (authorizationCall{"path", "second", "view", "member"}) {
		t.Fatalf("authorization checks = %+v", checks)
	}
	payload, err := shared.DecodeCursor(dependencies.CursorSigningKey, cursor)
	if err != nil || payload.Owner != "member" || payload.Domain != "path" || payload.AfterID != "second" || !payload.AfterCreated.Equal(created.Add(time.Hour)) || !payload.Snapshot.Equal(dependencies.Clock.Now()) {
		t.Fatalf("cursor payload = %+v, %v", payload, err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceListed || events[0].OwnerUserID != "member" || events[0].ActorUserID != "member" || events[0].TargetType != "path" || events[0].TargetID != "path" || events[0].Outcome != audit.Succeeded || events[0].CorrelationID != "request-id" {
		t.Fatalf("list audit = %+v", events)
	}
}

func TestListPathsAcceptsOnlyCurrentUsersOwnerBoundCursor(t *testing.T) {
	var lists []repositoryList
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{lists: &lists}
	validCursor, err := shared.EncodeCursor(dependencies.CursorSigningKey, shared.CursorPayload{
		Version: 1, Owner: "member", Domain: "path", AfterID: "previous",
		AfterCreated: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC), Snapshot: time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", validCursor, 100); err != nil {
		t.Fatal(err)
	}
	if len(lists) != 1 || lists[0].page.AfterID != "previous" || lists[0].page.Limit != 100 || !lists[0].page.Snapshot.Equal(time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC)) {
		t.Fatalf("decoded page = %+v", lists)
	}

	for _, cursor := range []string{"malformed", mustCursor(t, dependencies.CursorSigningKey, "other", "path"), mustCursor(t, dependencies.CursorSigningKey, "member", "other")} {
		if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", cursor, 25); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("cursor %q returned %v", cursor, err)
		}
	}
	for _, limit := range []int{-1, 101} {
		if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", "", limit); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("limit %d returned %v", limit, err)
		}
	}
}

func mustCursor(t *testing.T, key []byte, owner, domainName string) string {
	t.Helper()
	value, err := shared.EncodeCursor(key, shared.CursorPayload{
		Version: 1, Owner: owner, Domain: domainName, AfterID: "previous",
		AfterCreated: time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC), Snapshot: time.Date(2026, 7, 20, 11, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestListPathsRejectsRestrictedSessionsBeforeDependencies(t *testing.T) {
	for _, authorization := range []string{"", "Bearer malformed", "Bearer oidc.header.payload"} {
		var lists []repositoryList
		var checks []authorizationCall
		dependencies := configuredDependencies()
		dependencies.Repository = controlledRepository{lists: &lists}
		dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
		if _, _, err := New(dependencies).List(context.Background(), authorization, "", 25); !errors.Is(err, ports.ErrInvalidCredential) {
			t.Fatalf("credential %q returned %v", authorization, err)
		}
		if len(lists) != 0 || len(checks) != 0 {
			t.Fatalf("credential %q reached dependencies: lists=%+v checks=%+v", authorization, lists, checks)
		}
	}

	for _, scopes := range [][]string{{"api:onboarding"}, {"openid"}, {"api:user", "api:onboarding"}, nil} {
		var lists []repositoryList
		dependencies := configuredDependencies()
		dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "provisional", Scopes: scopes}}
		dependencies.Repository = controlledRepository{lists: &lists}
		if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", "", 25); !errors.Is(err, platformapp.ErrUnauthenticated) {
			t.Fatalf("scopes %v returned %v", scopes, err)
		}
		if len(lists) != 0 {
			t.Fatalf("scopes %v reached repository: %+v", scopes, lists)
		}
	}
}

func TestListPathsFailsClosedOnAuthorizationAuditAndRepositoryFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		set  func(*Dependencies, error)
	}{
		{"authorization", func(d *Dependencies, err error) { d.Authorizer = controlledAuthorizer{err: err} }},
		{"repository", func(d *Dependencies, err error) { d.Repository = controlledRepository{err: err} }},
		{"audit", func(d *Dependencies, err error) { d.Audits = controlledAudits{err: err} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			failure := errors.New(test.name + " unavailable")
			dependencies := configuredDependencies()
			dependencies.Repository = controlledRepository{page: Page{Items: []domain.Entity{{ID: "path-id", OwnerUserID: "member", CreatedAt: dependencies.Clock.Now()}}}}
			test.set(&dependencies, failure)
			if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", "", 25); !errors.Is(err, failure) {
				t.Fatalf("failure returned %v", err)
			}
		})
	}

	var events []audit.Event
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{page: Page{Items: []domain.Entity{{ID: "stale-path", OwnerUserID: "member", CreatedAt: dependencies.Clock.Now()}}}}
	dependencies.Authorizer = controlledAuthorizer{allowed: false}
	dependencies.Audits = controlledAudits{events: &events}
	if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", "", 25); !errors.Is(err, platformapp.ErrForbidden) {
		t.Fatalf("authorization disagreement returned %v", err)
	}
	if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied || events[0].TargetID != "list" || events[0].TargetID == "stale-path" {
		t.Fatalf("generic denial audit = %+v", events)
	}
}

func TestListPathsRateLimitRunsBeforeRepository(t *testing.T) {
	var lists []repositoryList
	dependencies := configuredDependencies()
	dependencies.AuditRateLimiter = controlledLimiter{denied: true}
	dependencies.Repository = controlledRepository{lists: &lists}
	if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", "", 25); !errors.Is(err, platformapp.ErrRateLimited) {
		t.Fatalf("rate limit returned %v", err)
	}
	if len(lists) != 0 {
		t.Fatalf("rate-limited list reached repository: %+v", lists)
	}
}

func TestListPathsRejectsImpossibleRepositoryPageWithoutSuccessAudit(t *testing.T) {
	var events []audit.Event
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{page: Page{HasMore: true}}
	dependencies.Audits = controlledAudits{events: &events}
	if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", "", 25); !errors.Is(err, errInvalidPathDependencies) {
		t.Fatalf("impossible repository page returned %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("failed list emitted success audit: %+v", events)
	}
}
