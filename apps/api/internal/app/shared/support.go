package shared

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"

	"github.com/google/uuid"
)

type correlationContextKey struct{}

func WithCorrelationID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, correlationContextKey{}, id)
}

func CorrelationID(ctx context.Context) string {
	if id, ok := ctx.Value(correlationContextKey{}).(string); ok && id != "" {
		return id
	}
	return uuid.NewString()
}

func NewAuditEvent(ctx context.Context, clock ports.Clock, owner, actor string, action audit.Action, targetType, targetID string, outcome audit.Outcome) audit.Event {
	return audit.Event{ID: uuid.NewString(), OwnerUserID: owner, ActorUserID: actor, Action: action, TargetType: targetType, TargetID: targetID, Outcome: outcome, CorrelationID: CorrelationID(ctx), OccurredAt: clock.Now().UTC()}
}
