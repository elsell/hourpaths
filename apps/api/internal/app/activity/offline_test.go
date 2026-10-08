package activity

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type offlineMemory struct {
	testRepository
	commands map[string]OfflineTimerCommand
}

func (f *offlineMemory) SynchronizeTimer(_ context.Context, c OfflineTimerCommand) (OfflineTimerResult, error) {
	if f.commands == nil {
		f.commands = map[string]OfflineTimerCommand{}
	}
	identity := c.Timer.ParticipantID + ":" + c.Idempotency.Key
	if old, ok := f.commands[identity]; ok {
		if string(old.Idempotency.RequestHash) != string(c.Idempotency.RequestHash) {
			return OfflineTimerResult{}, ports.ErrIdempotencyConflict
		}
		timer := old.Timer
		return OfflineTimerResult{Outcome: "accepted", Timer: &timer, Replayed: true}, nil
	}
	f.commands[identity] = c
	timer := c.Timer
	return OfflineTimerResult{Outcome: "accepted", Timer: &timer}, nil
}
func TestOfflineReplayPreservesDeviceOccurrenceAndBindsImmutableContent(t *testing.T) {
	repository := &offlineMemory{}
	input := OfflineTimerInput{StillRunning: true, TimerID: "device-timer", Kind: "start", StartedAt: testNow.Add(-time.Hour), OccurrenceTimeZone: "Asia/Tokyo"}
	first := testService(testRepository{})
	first.Repository = repository
	result, err := first.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "offline-request-0001", input)
	if err != nil || result.Timer.StartedAt != input.StartedAt || result.Timer.OccurrenceTimeZone != input.OccurrenceTimeZone {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	command := repository.commands["user-1:offline-request-0001"]
	if !command.NotifyStart || command.Timer.ParticipantID != "user-1" || !command.Audit.Valid() || command.Audit.TargetID != input.TimerID {
		t.Fatalf("command=%+v", command)
	}
	retry := testService(testRepository{})
	retry.Repository = repository
	retry.Clock = testClock{now: testNow.Add(time.Minute)}
	input.StillRunning = false // Current device state must not rewrite durable replay identity.
	result, err = retry.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "offline-request-0001", input)
	if err != nil || !result.Replayed || len(repository.commands) != 1 {
		t.Fatalf("replay=%+v err=%v", result, err)
	}
	changed := testService(testRepository{})
	changed.Repository = repository
	input.StartedAt = input.StartedAt.Add(time.Second)
	if _, err = changed.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "offline-request-0001", input); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("changed payload=%v", err)
	}
}
func TestOfflineReplayRequiresAuthorizationBeforePersistence(t *testing.T) {
	repository := &offlineMemory{}
	service := testService(testRepository{})
	service.Repository = repository
	service.Authorizer = testAuthorizer{allowed: false}
	input := OfflineTimerInput{TimerID: "device-timer", Kind: "start", StartedAt: testNow.Add(-time.Hour), OccurrenceTimeZone: "Etc/UTC"}
	if _, err := service.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "offline-denied-0001", input); err == nil || len(repository.commands) != 0 {
		t.Fatalf("denied replay persisted: %v", err)
	}
}

// A response from a faulty persistence adapter must not disclose another owner.
type foreignOffline struct{ testRepository }

func (foreignOffline) SynchronizeTimer(_ context.Context, c OfflineTimerCommand) (OfflineTimerResult, error) {
	timer := domain.RunningTimer{ID: "foreign", PathID: c.Timer.PathID, ParticipantID: "other", StartedAt: c.Timer.StartedAt, OccurrenceTimeZone: c.Timer.OccurrenceTimeZone}
	return OfflineTimerResult{Outcome: "accepted", Timer: &timer}, nil
}
func TestOfflineReplayRejectsForeignPersistenceResult(t *testing.T) {
	service := testService(testRepository{})
	service.Repository = foreignOffline{}
	_, err := service.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "offline-foreign-01", OfflineTimerInput{TimerID: "timer", Kind: "start", StartedAt: testNow.Add(-time.Hour), OccurrenceTimeZone: "Etc/UTC"})
	if !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("foreign result=%v", err)
	}
}

func (testRepository) SynchronizeTimer(context.Context, OfflineTimerCommand) (OfflineTimerResult, error) {
	return OfflineTimerResult{}, errInvalidDependencies
}

func TestOfflineCorrectionSeparatesOriginalIdentityFromReviewedOccurrence(t *testing.T) {
	repository := &offlineMemory{}
	service := testService(testRepository{})
	service.Repository = repository
	original := testNow.Add(time.Hour)
	reviewed := testNow.Add(-2 * time.Minute)
	ended := testNow.Add(-time.Minute)
	input := OfflineTimerInput{TimerID: "clock-changed", Kind: "correct", StartedAt: original, CorrectedStartedAt: &reviewed, EndedAt: &ended, OccurrenceTimeZone: "Asia/Tokyo"}
	if _, err := service.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "correction-request-001", input); err != nil {
		t.Fatal(err)
	}
	command := repository.commands["user-1:correction-request-001"]
	if !command.Timer.StartedAt.Equal(original) || command.CorrectedStartedAt == nil || !command.CorrectedStartedAt.Equal(reviewed) || command.Audit.Action != audit.ActivityTimerStopped {
		t.Fatalf("command=%+v", command)
	}
	changed := reviewed.Add(-time.Minute)
	input.CorrectedStartedAt = &changed
	if _, err := service.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "correction-request-001", input); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("changed correction=%v", err)
	}
	input.CorrectedStartedAt = &original
	if _, err := service.SynchronizeTimer(context.Background(), "Bearer valid", "path-1", "correction-request-002", input); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("future correction=%v", err)
	}
}
