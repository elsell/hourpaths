package activity

import (
	"context"
	"crypto/sha256"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

var errInvalidDependencies = errors.New("activity service dependencies are invalid")

type Dependencies struct {
	Auth             ports.Authenticator
	Profiles         ProfileReader
	Authorizer       ports.Authorizer
	Repository       Repository
	Audits           ports.Audits
	AuditRateLimiter ports.AuditRateLimiter
	Clock            ports.Clock
	NewID            func() string
	CursorSigningKey []byte
}

type Service struct{ Dependencies }

func New(dependencies Dependencies) *Service { return &Service{Dependencies: dependencies} }

func (s *Service) StartTimer(ctx context.Context, authorization, pathID, idempotencyKey string) (StartTimerResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return StartTimerResult{}, err
	}
	if !validPathID(pathID) || !validIdempotencyKey(idempotencyKey) {
		return StartTimerResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return StartTimerResult{}, err
	}
	if s.Profiles == nil {
		return StartTimerResult{}, errInvalidDependencies
	}
	timeZone, err := s.Profiles.TimeZone(ctx, principal.UserID)
	if err != nil {
		return StartTimerResult{}, err
	}
	intervalProgress, err := s.intervalProgressRequest(ctx, principal.UserID, pathID, now, timeZone)
	if err != nil {
		return StartTimerResult{}, err
	}
	timer, err := domain.StartTimer(s.NewID(), pathID, principal.UserID, now, timeZone, now)
	if err != nil {
		return StartTimerResult{}, errInvalidDependencies
	}
	result, err := s.Repository.StartTimer(ctx, StartTimerCommand{
		Timer: timer,
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: StartTimerOperation, Key: idempotencyKey,
			RequestHash: requestHash(StartTimerOperation, pathID)},
		Audit:            s.auditEvent(ctx, principal.UserID, audit.ActivityTimerStarted, timer.ID),
		IntervalProgress: intervalProgress,
	})
	if errors.Is(err, ports.ErrConflict) {
		if !validTimerFor(result.Timer, principal.UserID, pathID) || result.AccumulatedSeconds < 0 || !validIntervalProgress(intervalProgress, result.IntervalProgress) {
			return StartTimerResult{}, errors.Join(err, errInvalidDependencies)
		}
		return result, nil
	}
	if err != nil {
		return StartTimerResult{}, err
	}
	if result.AccumulatedSeconds < 0 || !validIntervalProgress(intervalProgress, result.IntervalProgress) {
		return StartTimerResult{}, errInvalidDependencies
	}
	if result.Replayed && result.Timer == (domain.RunningTimer{}) {
		return result, nil
	}
	if !validTimerFor(result.Timer, principal.UserID, pathID) || (!result.Replayed && result.Timer != timer) {
		return StartTimerResult{}, errInvalidDependencies
	}
	return result, nil
}

func (s *Service) CurrentTimer(ctx context.Context, authorization, pathID string) (CurrentTimerResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return CurrentTimerResult{}, err
	}
	if !validPathID(pathID) {
		return CurrentTimerResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return CurrentTimerResult{}, err
	}
	intervalProgress, err := s.intervalProgressRequest(ctx, principal.UserID, pathID, now, "")
	if err != nil {
		return CurrentTimerResult{}, err
	}
	result, err := s.Repository.CurrentProjection(ctx, principal.UserID, pathID, intervalProgress)
	if err != nil {
		return CurrentTimerResult{}, err
	}
	if result.AccumulatedSeconds < 0 || !validIntervalProgress(intervalProgress, result.IntervalProgress) {
		return CurrentTimerResult{}, errInvalidDependencies
	}
	targetID := pathID
	if result.Timer != nil {
		if !validTimerFor(*result.Timer, principal.UserID, pathID) {
			return CurrentTimerResult{}, errInvalidDependencies
		}
		targetID = result.Timer.ID
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceViewed, targetID)
	if result.Timer == nil {
		event.TargetType = "path"
	}
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return CurrentTimerResult{}, err
	}
	return result, nil
}

func (s *Service) intervalProgressRequest(ctx context.Context, participantID, pathID string, now time.Time, knownTimeZone string) (*IntervalProgressRequest, error) {
	goal, err := s.Repository.IntervalGoal(ctx, participantID, pathID)
	if err != nil {
		return nil, err
	}
	if !goal.Present {
		if goal != (pathdomain.IntervalGoal{}) {
			return nil, errInvalidDependencies
		}
		return nil, nil
	}
	if knownTimeZone == "" {
		if s.Profiles == nil {
			return nil, errInvalidDependencies
		}
		knownTimeZone, err = s.Profiles.TimeZone(ctx, participantID)
		if err != nil {
			return nil, err
		}
	}
	window, err := currentIntervalWindow(goal, knownTimeZone, now)
	if err != nil {
		return nil, errInvalidDependencies
	}
	return &IntervalProgressRequest{TargetSeconds: goal.TargetSeconds, Window: window}, nil
}

func validIntervalProgress(request *IntervalProgressRequest, progress *IntervalProgress) bool {
	if request == nil {
		return progress == nil
	}
	return progress != nil && request.TargetSeconds > 0 && progress.TargetSeconds == request.TargetSeconds &&
		progress.AccumulatedSeconds >= 0 && progress.Window == request.Window
}

// AccumulatedSeconds is the authorized projection boundary for completed time.
// Persistence derives it from canonical UTC instants instead of a duration
// column, and this authenticated read is audited like other domain reads.
func (s *Service) AccumulatedSeconds(ctx context.Context, authorization, pathID string) (int64, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return 0, err
	}
	if !validPathID(pathID) {
		return 0, ports.ErrInvalidArgument
	}
	if _, err := s.authorizeTracking(ctx, principal.UserID, pathID); err != nil {
		return 0, err
	}
	total, err := s.Repository.AccumulatedSeconds(ctx, principal.UserID, pathID)
	if err != nil {
		return 0, err
	}
	if total < 0 {
		return 0, errInvalidDependencies
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceViewed, pathID)
	event.TargetType = "path"
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return 0, err
	}
	return total, nil
}

func (s *Service) CreateManualActivity(ctx context.Context, authorization, pathID, idempotencyKey string, input ManualActivityInput) (CreateManualActivityResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return CreateManualActivityResult{}, err
	}
	if !validPathID(pathID) || !validIdempotencyKey(idempotencyKey) {
		return CreateManualActivityResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return CreateManualActivityResult{}, err
	}
	if s.Profiles == nil {
		return CreateManualActivityResult{}, errInvalidDependencies
	}
	timeZone, err := s.Profiles.TimeZone(ctx, principal.UserID)
	if err != nil {
		return CreateManualActivityResult{}, err
	}
	startedAt, err := manualStartInstant(input.LocalDate, input.LocalStartTime, timeZone)
	if err != nil {
		return CreateManualActivityResult{}, ports.ErrInvalidArgument
	}
	entry, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: s.NewID(), PathID: pathID, ParticipantID: principal.UserID,
		StartedAt: startedAt, DurationSeconds: input.DurationSeconds,
		OccurrenceTimeZone: timeZone, Note: input.Note,
	}, now)
	if err != nil {
		return CreateManualActivityResult{}, ports.ErrInvalidArgument
	}
	intervalProgress, err := s.intervalProgressRequest(ctx, principal.UserID, pathID, now, timeZone)
	if err != nil {
		return CreateManualActivityResult{}, err
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceCreated, entry.ID)
	event.TargetType = "activity"
	result, err := s.Repository.CreateManualActivity(ctx, CreateManualActivityCommand{
		Activity: entry,
		Idempotency: ports.Idempotency{
			PrincipalID: principal.UserID, Operation: CreateManualActivityOperation, Key: idempotencyKey,
			RequestHash: requestHash(CreateManualActivityOperation, pathID, input.LocalDate, input.LocalStartTime, strconv.FormatInt(input.DurationSeconds, 10), entry.Note),
		},
		Audit:            event,
		IntervalProgress: intervalProgress,
	})
	if err != nil {
		return CreateManualActivityResult{}, err
	}
	if !validCreatedManualResult(result, principal.UserID, pathID, entry) || !validIntervalProgress(intervalProgress, result.IntervalProgress) {
		return CreateManualActivityResult{}, errInvalidDependencies
	}
	return result, nil
}

func (s *Service) ManualActivityDefaults(ctx context.Context, authorization, pathID string) (ManualActivityDefaults, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return ManualActivityDefaults{}, err
	}
	if !validPathID(pathID) {
		return ManualActivityDefaults{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return ManualActivityDefaults{}, err
	}
	if s.Profiles == nil {
		return ManualActivityDefaults{}, errInvalidDependencies
	}
	timeZone, err := s.Profiles.TimeZone(ctx, principal.UserID)
	if err != nil {
		return ManualActivityDefaults{}, err
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil || timeZone == "Local" || strings.TrimSpace(timeZone) != timeZone {
		return ManualActivityDefaults{}, errInvalidDependencies
	}
	local := now.In(location)
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceViewed, pathID)
	event.TargetType = "path"
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return ManualActivityDefaults{}, err
	}
	return ManualActivityDefaults{LocalDate: local.Format("2006-01-02"), LocalStartTime: local.Format("15:04:05"), TimeZone: timeZone, CurrentInstant: now}, nil
}

func (s *Service) UpdateActivity(ctx context.Context, authorization, pathID, activityID, idempotencyKey string, input ManualActivityInput) (UpdateActivityResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return UpdateActivityResult{}, err
	}
	if !validPathID(pathID) || strings.TrimSpace(activityID) != activityID || activityID == "" || !validIdempotencyKey(idempotencyKey) {
		return UpdateActivityResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return UpdateActivityResult{}, err
	}
	current, _, err := s.Repository.GetActivity(ctx, principal.UserID, pathID, activityID)
	if err != nil {
		return UpdateActivityResult{}, err
	}
	if current.ParticipantID != principal.UserID || current.PathID != pathID || current.ID != activityID || current.ValidateAt(current.UpdatedAt) != nil {
		return UpdateActivityResult{}, errInvalidDependencies
	}
	intervalProgress, err := s.intervalProgressRequest(ctx, principal.UserID, pathID, now, "")
	if err != nil {
		return UpdateActivityResult{}, err
	}
	timeZone := current.OccurrenceTimeZone
	startedAt, err := manualStartInstant(input.LocalDate, input.LocalStartTime, timeZone)
	if err != nil || input.DurationSeconds <= 0 {
		return UpdateActivityResult{}, ports.ErrInvalidArgument
	}
	canonicalInput, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: activityID, PathID: pathID, ParticipantID: principal.UserID,
		StartedAt: startedAt, DurationSeconds: input.DurationSeconds,
		OccurrenceTimeZone: timeZone, Note: input.Note,
	}, now)
	if err != nil {
		return UpdateActivityResult{}, ports.ErrInvalidArgument
	}
	edit := domain.ActivityEdit{StartedAt: startedAt, DurationSeconds: input.DurationSeconds, OccurrenceTimeZone: timeZone, Note: canonicalInput.Note}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceUpdated, activityID)
	event.TargetType = "activity"
	result, err := s.Repository.UpdateActivity(ctx, UpdateActivityCommand{
		ActivityID: activityID, PathID: pathID, ParticipantID: principal.UserID, Edit: edit, UpdatedAt: now,
		Idempotency: ports.Idempotency{
			PrincipalID: principal.UserID, Operation: UpdateActivityOperation, Key: idempotencyKey,
			RequestHash: requestHash(UpdateActivityOperation, pathID, activityID, input.LocalDate, input.LocalStartTime, strconv.FormatInt(input.DurationSeconds, 10), canonicalInput.Note),
		},
		Audit:            event,
		IntervalProgress: intervalProgress,
	})
	if err != nil {
		return UpdateActivityResult{}, err
	}
	if !validUpdatedActivityResult(result, principal.UserID, pathID, activityID, edit, now) || !validIntervalProgress(intervalProgress, result.IntervalProgress) {
		return UpdateActivityResult{}, errInvalidDependencies
	}
	return result, nil
}

func (s *Service) DeleteActivity(ctx context.Context, authorization, pathID, activityID, idempotencyKey string) (DeleteActivityResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return DeleteActivityResult{}, err
	}
	if !validPathID(pathID) || strings.TrimSpace(activityID) != activityID || activityID == "" || !validIdempotencyKey(idempotencyKey) {
		return DeleteActivityResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return DeleteActivityResult{}, err
	}
	intervalProgress, err := s.intervalProgressRequest(ctx, principal.UserID, pathID, now, "")
	if err != nil {
		return DeleteActivityResult{}, err
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceDeleted, activityID)
	event.TargetType = "activity"
	result, err := s.Repository.DeleteActivity(ctx, DeleteActivityCommand{
		ActivityID: activityID, PathID: pathID, ParticipantID: principal.UserID,
		Idempotency: ports.Idempotency{
			PrincipalID: principal.UserID, Operation: DeleteActivityOperation, Key: idempotencyKey,
			RequestHash: requestHash(DeleteActivityOperation, pathID, activityID),
		},
		Audit:            event,
		IntervalProgress: intervalProgress,
	})
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			denied := s.auditEvent(ctx, principal.UserID, audit.ResourceAccessDenied, activityID)
			denied.TargetType = "activity"
			if auditErr := s.Audits.AppendAuditEvent(ctx, denied); auditErr != nil {
				return DeleteActivityResult{}, auditErr
			}
		}
		return DeleteActivityResult{}, err
	}
	if result.AccumulatedSeconds < 0 || result.SessionCount < 0 || result.UnreadNotificationCount < 0 || !validRemovedFeedEventIDs(result.RemovedFeedEventIDs, activityID) || !validIntervalProgress(intervalProgress, result.IntervalProgress) {
		return DeleteActivityResult{}, errInvalidDependencies
	}
	return result, nil
}

func validRemovedFeedEventIDs(ids []string, activityID string) bool {
	if len(ids) == 0 || !slices.IsSorted(ids) {
		return false
	}
	wantPracticeID := "practice:" + activityID
	practiceCount := 0
	for index, id := range ids {
		if strings.TrimSpace(id) != id || id == "" || (index > 0 && ids[index-1] == id) {
			return false
		}
		if id == wantPracticeID {
			practiceCount++
		} else if !strings.HasPrefix(id, "achievement:") || id == "achievement:" {
			return false
		}
	}
	return practiceCount == 1
}

func (s *Service) StopTimer(ctx context.Context, authorization, pathID, timerID, idempotencyKey string) (StopTimerResult, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return StopTimerResult{}, err
	}
	if !validPathID(pathID) || strings.TrimSpace(timerID) != timerID || timerID == "" || !validIdempotencyKey(idempotencyKey) {
		return StopTimerResult{}, ports.ErrInvalidArgument
	}
	now, err := s.authorizeTracking(ctx, principal.UserID, pathID)
	if err != nil {
		return StopTimerResult{}, err
	}
	intervalProgress, err := s.intervalProgressRequest(ctx, principal.UserID, pathID, now, "")
	if err != nil {
		return StopTimerResult{}, err
	}
	result, err := s.Repository.StopTimer(ctx, StopTimerCommand{
		TimerID: timerID, PathID: pathID, ParticipantID: principal.UserID,
		ActivityID: s.NewID(), StoppedAt: now, RecordedAt: now,
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: StopTimerOperation, Key: idempotencyKey,
			RequestHash: requestHash(StopTimerOperation, pathID, timerID)},
		Audit:            s.auditEvent(ctx, principal.UserID, audit.ActivityTimerStopped, timerID),
		IntervalProgress: intervalProgress,
	})
	if err != nil {
		return StopTimerResult{}, err
	}
	if result.AccumulatedSeconds < 0 || !validStopResult(result, principal.UserID, pathID, timerID, now) || !validIntervalProgress(intervalProgress, result.IntervalProgress) {
		return StopTimerResult{}, errInvalidDependencies
	}
	return result, nil
}

func (s *Service) GetRunningTimer(ctx context.Context, authorization, pathID string) (domain.RunningTimer, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return domain.RunningTimer{}, err
	}
	if !validPathID(pathID) {
		return domain.RunningTimer{}, ports.ErrInvalidArgument
	}
	if _, err := s.authorizeTracking(ctx, principal.UserID, pathID); err != nil {
		return domain.RunningTimer{}, err
	}
	timer, err := s.Repository.GetRunningTimer(ctx, principal.UserID, pathID)
	if err != nil {
		return domain.RunningTimer{}, err
	}
	if !validTimerFor(timer, principal.UserID, pathID) {
		return domain.RunningTimer{}, errInvalidDependencies
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceViewed, timer.ID)
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return domain.RunningTimer{}, err
	}
	return timer, nil
}

func (s *Service) GetActivity(ctx context.Context, authorization, pathID, activityID string) (domain.RecordedActivity, int64, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return domain.RecordedActivity{}, 0, err
	}
	if !validPathID(pathID) || strings.TrimSpace(activityID) != activityID || activityID == "" {
		return domain.RecordedActivity{}, 0, ports.ErrInvalidArgument
	}
	if _, err := s.authorizePath(ctx, principal.UserID, pathID, "view"); err != nil {
		return domain.RecordedActivity{}, 0, err
	}
	entry, version, err := s.Repository.GetActivity(ctx, principal.UserID, pathID, activityID)
	if err != nil {
		return domain.RecordedActivity{}, 0, err
	}
	if !validActivityView(entry, version, principal.UserID, pathID, activityID) {
		return domain.RecordedActivity{}, 0, errInvalidDependencies
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceViewed, activityID)
	event.TargetType = "activity"
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return domain.RecordedActivity{}, 0, err
	}
	return entry, version, nil
}

func (s *Service) ListActivities(ctx context.Context, authorization, pathID, participantID, cursor string, limit int) ([]ActivityListRecord, string, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if !validPathID(pathID) || (participantID != "" && strings.TrimSpace(participantID) != participantID) {
		return nil, "", ports.ErrInvalidArgument
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if len(s.CursorSigningKey) < 32 {
		return nil, "", errInvalidDependencies
	}
	now, err := s.authorizePath(ctx, principal.UserID, pathID, "view")
	if err != nil {
		return nil, "", err
	}
	request := ActivityPageRequest{ParticipantID: participantID, Limit: limit, Snapshot: now}
	domainName := "path-activities:" + pathID
	if participantID != "" {
		domainName += ":participant:" + participantID
	}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(s.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != domainName {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterID, request.AfterStartedAt, request.Snapshot = payload.AfterID, payload.AfterCreated, payload.Snapshot
	}
	page, err := s.Repository.ListActivities(ctx, principal.UserID, pathID, request)
	if err != nil {
		return nil, "", err
	}
	if len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) {
		return nil, "", errInvalidDependencies
	}
	for _, entry := range page.Items {
		if !validActivityView(entry.Activity, entry.Version, principal.UserID, pathID, entry.Activity.ID) || (participantID != "" && entry.Activity.ParticipantID != participantID) {
			return nil, "", errInvalidDependencies
		}
	}
	nextCursor := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		nextCursor, err = shared.EncodeCursor(s.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: domainName, AfterID: last.Activity.ID, AfterCreated: last.Activity.StartedAt, Snapshot: request.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceListed, pathID)
	event.TargetType = "activity"
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return page.Items, nextCursor, nil
}

func (s *Service) ListActivityRevisions(ctx context.Context, authorization, pathID, activityID, cursor string, limit int) ([]ActivityRevisionRecord, string, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if !validPathID(pathID) || strings.TrimSpace(activityID) != activityID || activityID == "" {
		return nil, "", ports.ErrInvalidArgument
	}
	if limit == 0 {
		limit = 25
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if len(s.CursorSigningKey) < 32 {
		return nil, "", errInvalidDependencies
	}
	now, err := s.authorizePath(ctx, principal.UserID, pathID, "view")
	if err != nil {
		return nil, "", err
	}
	request := ActivityRevisionPageRequest{Limit: limit, Snapshot: now}
	domainName := "activity-revisions:" + pathID + ":" + activityID
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(s.CursorSigningKey, cursor)
		beforeVersion, parseErr := strconv.ParseInt(payload.AfterID, 10, 64)
		if decodeErr != nil || parseErr != nil || beforeVersion < 1 || payload.Owner != principal.UserID || payload.Domain != domainName {
			return nil, "", ports.ErrInvalidArgument
		}
		request.BeforeVersion, request.Snapshot = beforeVersion, payload.Snapshot
	}
	page, err := s.Repository.ListActivityRevisions(ctx, principal.UserID, pathID, activityID, request)
	if err != nil {
		return nil, "", err
	}
	if len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) {
		return nil, "", errInvalidDependencies
	}
	for index, revision := range page.Items {
		if revision.Version < 1 || (index > 0 && page.Items[index-1].Version <= revision.Version) || !validActivityView(revision.Revision.Activity, revision.Version, principal.UserID, pathID, activityID) || revision.Revision.ReplacedAt.Before(revision.Revision.Activity.UpdatedAt) {
			return nil, "", errInvalidDependencies
		}
	}
	nextCursor := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		nextCursor, err = shared.EncodeCursor(s.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: domainName, AfterID: strconv.FormatInt(last.Version, 10), AfterCreated: last.Revision.ReplacedAt, Snapshot: request.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceListed, activityID)
	event.TargetType = "activity"
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return page.Items, nextCursor, nil
}

func (s *Service) authenticate(ctx context.Context, authorization string) (ports.Principal, error) {
	if s == nil || s.Auth == nil {
		return ports.Principal{}, errInvalidDependencies
	}
	principal, err := s.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, err
	}
	if principal.UserID == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:user" {
		return ports.Principal{}, platformapp.ErrUnauthenticated
	}
	return principal, nil
}

func (s *Service) authorizeTracking(ctx context.Context, userID, pathID string) (time.Time, error) {
	return s.authorizePath(ctx, userID, pathID, "track")
}

func (s *Service) authorizePath(ctx context.Context, userID, pathID, permission string) (time.Time, error) {
	if s.Authorizer == nil || s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil {
		return time.Time{}, errInvalidDependencies
	}
	now := durableInstant(s.Clock.Now())
	if now.IsZero() {
		return time.Time{}, errInvalidDependencies
	}
	if !s.AuditRateLimiter.Allow(userID, now) {
		return time.Time{}, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", pathID, permission, userID)
	if err != nil {
		return time.Time{}, err
	}
	if !allowed {
		event := s.auditEvent(ctx, userID, audit.ResourceAccessDenied, pathID)
		if auditErr := s.Audits.AppendAuditEvent(ctx, event); auditErr != nil {
			return time.Time{}, auditErr
		}
		return time.Time{}, platformapp.ErrForbidden
	}
	return now, nil
}

func (s *Service) auditEvent(ctx context.Context, userID string, action audit.Action, targetID string) audit.Event {
	event := shared.NewAuditEvent(ctx, s.Clock, userID, userID, action, "timer", targetID, audit.Succeeded)
	event.ID = s.NewID()
	if action == audit.ResourceAccessDenied {
		event.TargetType = "path"
		event.Outcome = audit.Denied
	}
	return event
}

func requestHash(parts ...string) []byte {
	digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return digest[:]
}

func validIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}
	return true
}

func validPathID(value string) bool { return value != "" && strings.TrimSpace(value) == value }

// Durable timer instants use PostgreSQL timestamptz precision so first
// responses and idempotent replays expose the same canonical state.
func durableInstant(value time.Time) time.Time { return value.UTC().Truncate(time.Microsecond) }

func validTimerFor(timer domain.RunningTimer, userID, pathID string) bool {
	canonical, err := domain.StartTimer(timer.ID, timer.PathID, timer.ParticipantID, timer.StartedAt, timer.OccurrenceTimeZone, timer.StartedAt)
	return err == nil && canonical == timer && timer.ParticipantID == userID && timer.PathID == pathID
}

func validStopResult(result StopTimerResult, userID, pathID, stoppedTimerID string, now time.Time) bool {
	if result.CurrentTimer != nil && (!result.Replayed || result.CurrentTimer.ID == stoppedTimerID || !validTimerFor(*result.CurrentTimer, userID, pathID)) {
		return false
	}
	if !result.Saved {
		return result.Activity == (domain.RecordedActivity{})
	}
	entry := result.Activity
	if entry.ID == "" || entry.ParticipantID != userID || entry.PathID != pathID || entry.EndedAt.After(now) || entry.CreatedAt.After(now) {
		return false
	}
	timer, err := domain.StartTimer("validation", entry.PathID, entry.ParticipantID, entry.StartedAt, entry.OccurrenceTimeZone, entry.StartedAt)
	if err != nil {
		return false
	}
	canonical, saved, err := timer.Stop(entry.ID, entry.EndedAt, entry.CreatedAt)
	return err == nil && saved && canonical == entry && (result.Replayed || (entry.EndedAt.Equal(now) && entry.CreatedAt.Equal(now)))
}

func validCreatedManualResult(result CreateManualActivityResult, userID, pathID string, requested domain.RecordedActivity) bool {
	if result.Version < 1 || result.AccumulatedSeconds < 0 || result.Activity.ID == "" || result.Activity.ParticipantID != userID || result.Activity.PathID != pathID {
		return false
	}
	entry := result.Activity
	canonical, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: entry.ID, PathID: entry.PathID, ParticipantID: entry.ParticipantID,
		StartedAt: entry.StartedAt, DurationSeconds: entry.DurationSeconds(),
		OccurrenceTimeZone: entry.OccurrenceTimeZone, Note: entry.Note,
	}, entry.CreatedAt)
	if err != nil || canonical != entry {
		return false
	}
	return result.Replayed || (result.Version == 1 && entry == requested)
}

func validUpdatedActivityResult(result UpdateActivityResult, userID, pathID, activityID string, edit domain.ActivityEdit, now time.Time) bool {
	if result.Version < 2 || result.AccumulatedSeconds < 0 || result.Activity.ID != activityID || result.Activity.ParticipantID != userID || result.Activity.PathID != pathID {
		return false
	}
	if result.Revision.Activity.ID != activityID || result.Revision.Activity.ParticipantID != userID || result.Revision.Activity.PathID != pathID || result.Revision.ReplacedAt != result.Activity.UpdatedAt {
		return false
	}
	if result.Replayed {
		edit = domain.ActivityEdit{
			StartedAt: result.Activity.StartedAt, DurationSeconds: result.Activity.DurationSeconds(),
			OccurrenceTimeZone: result.Activity.OccurrenceTimeZone, Note: result.Activity.Note,
		}
	}
	edited, revision, err := result.Revision.Activity.EditByOwner(userID, edit, result.Activity.UpdatedAt)
	if err != nil || edited != result.Activity || revision != result.Revision {
		return false
	}
	return result.Replayed || result.Activity.UpdatedAt == now
}

func validActivityView(entry domain.RecordedActivity, version int64, viewerID, pathID, activityID string) bool {
	if version < 1 || entry.ID != activityID || entry.PathID != pathID || strings.TrimSpace(entry.ParticipantID) == "" || (entry.ParticipantID != viewerID && entry.Note != "") {
		return false
	}
	return entry.ValidateAt(entry.UpdatedAt) == nil
}
