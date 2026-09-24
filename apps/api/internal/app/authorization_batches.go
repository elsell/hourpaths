package app

import (
	"context"
	"errors"
	"strings"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// ReconcileAuthorizationBatches delivers each claimed batch through exactly one
// relationship write. Failed and abandoned leases remain durable and retryable.
func (a App) ReconcileAuthorizationBatches(ctx context.Context, worker string, limit int) error {
	if a.AuthorizationBatchOutbox == nil || a.RelationshipWriter == nil || a.AuthorizationSerializer == nil || a.Audits == nil || a.Clock == nil {
		return errors.New("authorization batch dependencies are required")
	}
	batches, err := a.AuthorizationBatchOutbox.ClaimAuthorizationBatches(ctx, worker, authorizationLease, limit)
	if err != nil {
		return err
	}
	var failures []error
	for _, batch := range batches {
		if err := a.reconcileClaimedAuthorizationBatch(ctx, batch, worker); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// ReconcileAuthorizationBatch is the request-path fast path for a durably
// persisted batch. Background reconciliation uses the same delivery function.
func (a App) ReconcileAuthorizationBatch(ctx context.Context, id, worker string) error {
	if a.AuthorizationBatchOutbox == nil {
		return errors.New("authorization batch outbox is required")
	}
	batch, err := a.AuthorizationBatchOutbox.ClaimAuthorizationBatch(ctx, id, worker, authorizationLease)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			completed, statusErr := a.AuthorizationBatchOutbox.AuthorizationBatchCompleted(ctx, id)
			if statusErr != nil {
				return statusErr
			}
			if completed {
				return nil
			}
			return ports.ErrAuthorizationPending
		}
		return err
	}
	return a.reconcileClaimedAuthorizationBatch(ctx, batch, worker)
}

func (a App) reconcileClaimedAuthorizationBatch(ctx context.Context, batch ports.AuthorizationBatch, worker string) error {
	if a.AuthorizationBatchOutbox == nil || a.RelationshipWriter == nil || a.AuthorizationSerializer == nil || a.Audits == nil || a.Clock == nil {
		return errors.New("authorization batch dependencies are required")
	}
	var operationErr error
	err := a.AuthorizationSerializer.WithinResource(ctx, batch.ResourceType, batch.ResourceID, func(locked context.Context) error {
		if err := a.AuthorizationBatchOutbox.RenewAuthorizationBatch(locked, batch.ID, worker, authorizationLease); err != nil {
			return err
		}
		if !validAuthorizationBatch(batch) {
			operationErr = errors.New("authorization batch is invalid")
		} else {
			callContext, cancel := context.WithTimeout(locked, authorizationCallTimeout)
			defer cancel()
			operationErr = a.RelationshipWriter.WriteRelationships(callContext, batch.Updates)
		}
		if operationErr != nil {
			auditErr := a.Audits.AppendAuditEvent(locked, a.auditEvent(locked, batch.OwnerUserID, batch.ActorUserID, audit.AuthorizationFailed, batch.ResourceType, batch.ResourceID, audit.Failed))
			return errors.Join(operationErr, auditErr)
		}
		event := a.auditEvent(locked, batch.OwnerUserID, batch.ActorUserID, audit.AuthorizationApplied, batch.ResourceType, batch.ResourceID, audit.Succeeded)
		return a.AuthorizationBatchOutbox.CompleteAuthorizationBatchWithAudit(locked, batch.ID, worker, event)
	})
	if operationErr != nil {
		failErr := a.AuthorizationBatchOutbox.FailAuthorizationBatch(ctx, batch.ID, worker, "dependency_failure")
		return errors.Join(err, failErr)
	}
	return err
}

func validAuthorizationBatch(batch ports.AuthorizationBatch) bool {
	if strings.TrimSpace(batch.ID) == "" || strings.TrimSpace(batch.TransferID) == "" ||
		strings.TrimSpace(batch.ResourceType) == "" || strings.TrimSpace(batch.ResourceID) == "" ||
		strings.TrimSpace(batch.OwnerUserID) == "" || strings.TrimSpace(batch.ActorUserID) == "" || len(batch.Updates) != 4 {
		return false
	}
	for _, update := range batch.Updates {
		if update.ResourceType != batch.ResourceType || update.ResourceID != batch.ResourceID ||
			(update.Operation != ports.AuthorizationTouch && update.Operation != ports.AuthorizationDelete) ||
			strings.TrimSpace(update.Relation) == "" || strings.TrimSpace(update.SubjectType) == "" || strings.TrimSpace(update.SubjectID) == "" {
			return false
		}
	}
	formerCreator := batch.Updates[0].SubjectID
	return formerCreator != batch.OwnerUserID &&
		batch.Updates[0] == (ports.RelationshipUpdate{Operation: ports.AuthorizationDelete, ResourceType: batch.ResourceType, ResourceID: batch.ResourceID, Relation: "creator", SubjectType: "user", SubjectID: formerCreator}) &&
		batch.Updates[1] == (ports.RelationshipUpdate{Operation: ports.AuthorizationTouch, ResourceType: batch.ResourceType, ResourceID: batch.ResourceID, Relation: "creator", SubjectType: "user", SubjectID: batch.OwnerUserID}) &&
		batch.Updates[2] == (ports.RelationshipUpdate{Operation: ports.AuthorizationTouch, ResourceType: batch.ResourceType, ResourceID: batch.ResourceID, Relation: "administrator", SubjectType: "user", SubjectID: formerCreator}) &&
		batch.Updates[3] == (ports.RelationshipUpdate{Operation: ports.AuthorizationDelete, ResourceType: batch.ResourceType, ResourceID: batch.ResourceID, Relation: "administrator", SubjectType: "user", SubjectID: batch.OwnerUserID})
}
