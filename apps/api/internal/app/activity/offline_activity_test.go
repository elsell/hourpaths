package activity

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type recordedMemory struct {
	testRepository
	commands map[string]OfflineActivityCommand
}

func (r *recordedMemory) SynchronizeActivity(_ context.Context, c OfflineActivityCommand) (OfflineActivityResult, error) {
	if r.commands == nil {
		r.commands = map[string]OfflineActivityCommand{}
	}
	key := c.Activity.ParticipantID + ":" + c.Idempotency.Key
	if prior, ok := r.commands[key]; ok {
		if string(prior.Idempotency.RequestHash) != string(c.Idempotency.RequestHash) {
			return OfflineActivityResult{}, ports.ErrIdempotencyConflict
		}
		return OfflineActivityResult{Outcome: "accepted", Activity: &prior.Activity, Order: &prior.Order, Replayed: true}, nil
	}
	r.commands[key] = c
	return OfflineActivityResult{Outcome: "accepted", Activity: &c.Activity, Order: &c.Order}, nil
}
func TestOfflineActivityPreservesOccurrenceAndRetryIdentity(t *testing.T) {
	repository := &recordedMemory{}
	service := testService(testRepository{})
	service.Repository = repository
	input := OfflineActivityInput{Kind: "create", ActivityID: "device-entry", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "Asia/Tokyo", Note: "draft", AuthoredAt: testNow.Add(-time.Minute)}
	first, err := service.SynchronizeActivity(context.Background(), "Bearer valid", "path-1", "offline-entry-operation-1", input)
	if err != nil || first.Activity.ID != input.ActivityID || first.Activity.OccurrenceTimeZone != input.OccurrenceTimeZone {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	command := repository.commands["user-1:offline-entry-operation-1"]
	if command.Activity.ParticipantID != "user-1" || !command.Audit.Valid() || command.Audit.TargetID != input.ActivityID || command.Order.OperationID != "offline-entry-operation-1" {
		t.Fatalf("command=%+v", command)
	}
	service.Clock = testClock{now: testNow.Add(time.Minute)}
	retry, err := service.SynchronizeActivity(context.Background(), "Bearer valid", "path-1", "offline-entry-operation-1", input)
	if err != nil || !retry.Replayed || len(repository.commands) != 1 {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	input.Counter = 1
	if _, err := service.SynchronizeActivity(context.Background(), "Bearer valid", "path-1", "offline-entry-operation-1", input); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("changed stamp=%v", err)
	}
}
func TestOfflineActivityDenialNeverPersists(t *testing.T) {
	repository := &recordedMemory{}
	service := testService(testRepository{})
	service.Repository = repository
	service.Authorizer = testAuthorizer{allowed: false}
	input := OfflineActivityInput{Kind: "edit", ActivityID: "entry", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "UTC", AuthoredAt: testNow}
	if _, err := service.SynchronizeActivity(context.Background(), "Bearer valid", "path-1", "offline-entry-operation-2", input); err == nil || len(repository.commands) != 0 {
		t.Fatalf("denied persisted: %v", err)
	}
}

type foreignRecordedRepository struct{ testRepository }

func (foreignRecordedRepository) SynchronizeActivity(_ context.Context, c OfflineActivityCommand) (OfflineActivityResult, error) {
	c.Activity.ParticipantID = "another-owner"
	return OfflineActivityResult{Outcome: "accepted", Activity: &c.Activity, Order: &c.Order}, nil
}
func TestOfflineActivityRejectsForeignRepositoryResult(t *testing.T) {
	service := testService(testRepository{})
	service.Repository = foreignRecordedRepository{}
	input := OfflineActivityInput{Kind: "edit", ActivityID: "entry", StartedAt: testNow.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "UTC", AuthoredAt: testNow}
	if _, err := service.SynchronizeActivity(context.Background(), "Bearer valid", "path-1", "offline-entry-operation-3", input); !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("foreign result=%v", err)
	}
}

func (testRepository) SynchronizeActivity(context.Context, OfflineActivityCommand) (OfflineActivityResult, error) {
	return OfflineActivityResult{}, errInvalidDependencies
}
