package activitystore

import "github.com/elsell/hour-paths/apps/api/internal/domain/audit"

func fromAudit(event audit.Event) *auditModel {
	return &auditModel{ID: event.ID, OwnerUserID: event.OwnerUserID, ActorUserID: event.ActorUserID, Action: event.Action, TargetType: event.TargetType, TargetID: event.TargetID, Outcome: event.Outcome, CorrelationID: event.CorrelationID, OccurredAt: event.OccurredAt}
}
