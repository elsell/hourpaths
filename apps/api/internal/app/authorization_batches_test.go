package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledBatchOutbox struct {
	batches         []ports.AuthorizationBatch
	claimErr        error
	completedStatus bool
	renewErr        error
	completed       *bool
	failed          *bool
	completion      *audit.Event
}

func (f controlledBatchOutbox) ClaimAuthorizationBatches(context.Context, string, time.Duration, int) ([]ports.AuthorizationBatch, error) {
	return f.batches, nil
}
func (f controlledBatchOutbox) ClaimAuthorizationBatch(context.Context, string, string, time.Duration) (ports.AuthorizationBatch, error) {
	if f.claimErr != nil {
		return ports.AuthorizationBatch{}, f.claimErr
	}
	if len(f.batches) == 0 {
		return ports.AuthorizationBatch{}, ports.ErrNotFound
	}
	return f.batches[0], nil
}
func (f controlledBatchOutbox) AuthorizationBatchCompleted(context.Context, string) (bool, error) {
	return f.completedStatus, nil
}
func (f controlledBatchOutbox) RenewAuthorizationBatch(context.Context, string, string, time.Duration) error {
	return f.renewErr
}
func (f controlledBatchOutbox) CompleteAuthorizationBatchWithAudit(_ context.Context, _, _ string, event audit.Event) error {
	if f.completed != nil {
		*f.completed = true
	}
	if f.completion != nil {
		*f.completion = event
	}
	return nil
}
func (f controlledBatchOutbox) FailAuthorizationBatch(context.Context, string, string, string) error {
	if f.failed != nil {
		*f.failed = true
	}
	return nil
}

type controlledBatchWriter struct {
	batches *[][]ports.RelationshipUpdate
	err     error
}

func (f controlledBatchWriter) WriteRelationships(_ context.Context, updates []ports.RelationshipUpdate) error {
	if f.batches != nil {
		*f.batches = append(*f.batches, append([]ports.RelationshipUpdate(nil), updates...))
	}
	return f.err
}

func validTestAuthorizationBatch() ports.AuthorizationBatch {
	return ports.AuthorizationBatch{
		ID: "batch-1", TransferID: "transfer-1", ResourceType: "path", ResourceID: "path-1", OwnerUserID: "recipient", ActorUserID: "recipient",
		Updates: []ports.RelationshipUpdate{
			{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: "path-1", Relation: "creator", SubjectType: "user", SubjectID: "creator"},
			{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: "path-1", Relation: "creator", SubjectType: "user", SubjectID: "recipient"},
			{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: "path-1", Relation: "administrator", SubjectType: "user", SubjectID: "creator"},
			{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: "path-1", Relation: "administrator", SubjectType: "user", SubjectID: "recipient"},
		},
	}
}

func TestAuthorizationBatchWorkerWritesExactlyOneAtomicBatchAndCompletesAuditedClaim(t *testing.T) {
	batch := validTestAuthorizationBatch()
	completed := false
	var completion audit.Event
	var writes [][]ports.RelationshipUpdate
	a := App{
		AuthorizationBatchOutbox: controlledBatchOutbox{batches: []ports.AuthorizationBatch{batch}, completed: &completed, completion: &completion},
		RelationshipWriter:       controlledBatchWriter{batches: &writes}, AuthorizationSerializer: fakeSerializer{}, Audits: fakeAudits{}, Clock: fakeClock{now: time.Now().UTC()},
	}
	if err := a.ReconcileAuthorizationBatches(context.Background(), "worker", 10); err != nil {
		t.Fatal(err)
	}
	if len(writes) != 1 || len(writes[0]) != 4 || !completed {
		t.Fatalf("writes=%+v completed=%t", writes, completed)
	}
	if completion.Action != audit.AuthorizationApplied || completion.OwnerUserID != "recipient" || completion.ActorUserID != "recipient" || completion.TargetType != "path" || completion.TargetID != "path-1" {
		t.Fatalf("completion audit=%+v", completion)
	}
}

func TestAuthorizationBatchWorkerLeavesDependencyFailureRetryable(t *testing.T) {
	failed := false
	var events []audit.Event
	a := App{
		AuthorizationBatchOutbox: controlledBatchOutbox{batches: []ports.AuthorizationBatch{validTestAuthorizationBatch()}, failed: &failed},
		RelationshipWriter:       controlledBatchWriter{err: errors.New("spicedb unavailable")}, AuthorizationSerializer: fakeSerializer{}, Audits: fakeAudits{events: &events}, Clock: fakeClock{now: time.Now().UTC()},
	}
	if err := a.ReconcileAuthorizationBatches(context.Background(), "worker", 10); err == nil || !failed {
		t.Fatalf("error=%v failed=%t", err, failed)
	}
	if len(events) != 1 || events[0].Action != audit.AuthorizationFailed {
		t.Fatalf("failure audit=%+v", events)
	}
}

func TestAuthorizationBatchWorkerRejectsStaleLeaseBeforeSpiceDB(t *testing.T) {
	wrote := false
	writer := controlledBatchWriter{}
	writes := [][]ports.RelationshipUpdate{}
	writer.batches = &writes
	a := App{
		AuthorizationBatchOutbox: controlledBatchOutbox{batches: []ports.AuthorizationBatch{validTestAuthorizationBatch()}, renewErr: ports.ErrNotFound},
		RelationshipWriter:       writer, AuthorizationSerializer: fakeSerializer{}, Audits: fakeAudits{}, Clock: fakeClock{now: time.Now().UTC()},
	}
	if err := a.ReconcileAuthorizationBatches(context.Background(), "stale", 10); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
	wrote = len(writes) > 0
	if wrote {
		t.Fatal("stale worker contacted SpiceDB")
	}
}

func TestAuthorizationBatchWorkerRejectsSemanticallyInvalidOwnershipHandoff(t *testing.T) {
	batch := validTestAuthorizationBatch()
	batch.Updates[1].SubjectID = "attacker"
	failed := false
	var writes [][]ports.RelationshipUpdate
	a := App{
		AuthorizationBatchOutbox: controlledBatchOutbox{batches: []ports.AuthorizationBatch{batch}, failed: &failed},
		RelationshipWriter:       controlledBatchWriter{batches: &writes}, AuthorizationSerializer: fakeSerializer{}, Audits: fakeAudits{}, Clock: fakeClock{now: time.Now().UTC()},
	}
	if err := a.ReconcileAuthorizationBatches(context.Background(), "worker", 10); err == nil || !failed {
		t.Fatalf("error=%v failed=%t", err, failed)
	}
	if len(writes) != 0 {
		t.Fatalf("invalid handoff reached SpiceDB: %+v", writes)
	}
}

func TestAuthorizationBatchRequestReplayAcceptsOnlyDurablyCompletedBatch(t *testing.T) {
	a := App{AuthorizationBatchOutbox: controlledBatchOutbox{claimErr: ports.ErrNotFound, completedStatus: true}}
	if err := a.ReconcileAuthorizationBatch(context.Background(), "batch-1", "worker"); err != nil {
		t.Fatalf("completed replay error = %v", err)
	}
	a.AuthorizationBatchOutbox = controlledBatchOutbox{claimErr: ports.ErrNotFound}
	if err := a.ReconcileAuthorizationBatch(context.Background(), "batch-1", "worker"); !errors.Is(err, ports.ErrAuthorizationPending) {
		t.Fatalf("locked incomplete replay error = %v", err)
	}
}
