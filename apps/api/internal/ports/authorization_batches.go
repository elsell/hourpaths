package ports

import (
	"context"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

// AuthorizationBatch is one indivisible authorization transition. Updates are
// ordered because one SpiceDB WriteRelationships request applies the complete
// sequence atomically.
type AuthorizationBatch struct {
	ID, TransferID, ResourceType, ResourceID string
	OwnerUserID, ActorUserID                 string
	Updates                                  []RelationshipUpdate
	Attempts                                 int
	LockedBy                                 string
	LockedUntil                              time.Time
	Lease                                    time.Duration
}

// AuthorizationBatchOutbox keeps multi-relationship transitions retryable
// across process crashes. Claims and lease decisions use PostgreSQL time.
type AuthorizationBatchOutbox interface {
	ClaimAuthorizationBatches(context.Context, string, time.Duration, int) ([]AuthorizationBatch, error)
	ClaimAuthorizationBatch(context.Context, string, string, time.Duration) (AuthorizationBatch, error)
	AuthorizationBatchCompleted(context.Context, string) (bool, error)
	RenewAuthorizationBatch(context.Context, string, string, time.Duration) error
	CompleteAuthorizationBatchWithAudit(context.Context, string, string, audit.Event) error
	FailAuthorizationBatch(context.Context, string, string, string) error
}
