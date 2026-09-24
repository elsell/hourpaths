package gormstore

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type authorizationBatchOutboxModel struct {
	ID, PathOwnershipTransferID, ResourceType, ResourceID string
	OwnerUserID, ActorUserID                              string
	RelationshipUpdates                                   []byte `gorm:"type:jsonb"`
	Attempts                                              int
	LockedBy                                              *string
	LockedUntil, CompletedAt                              *time.Time
	FailureCode                                           string
	CreatedAt                                             time.Time
}

func (authorizationBatchOutboxModel) TableName() string {
	return "authorization_batch_outbox_models"
}

type persistedRelationshipUpdate struct {
	Operation    ports.AuthorizationOperation `json:"operation"`
	ResourceType string                       `json:"resourceType"`
	ResourceID   string                       `json:"resourceId"`
	Relation     string                       `json:"relation"`
	SubjectType  string                       `json:"subjectType"`
	SubjectID    string                       `json:"subjectId"`
}

func encodeRelationshipUpdates(updates []ports.RelationshipUpdate) ([]byte, error) {
	if len(updates) != 4 {
		return nil, ports.ErrInvalidArgument
	}
	persisted := make([]persistedRelationshipUpdate, len(updates))
	for index, update := range updates {
		if !validRelationshipUpdate(update) {
			return nil, ports.ErrInvalidArgument
		}
		persisted[index] = persistedRelationshipUpdate{
			Operation: update.Operation, ResourceType: update.ResourceType, ResourceID: update.ResourceID,
			Relation: update.Relation, SubjectType: update.SubjectType, SubjectID: update.SubjectID,
		}
	}
	return json.Marshal(persisted)
}

func decodeRelationshipUpdates(value []byte) ([]ports.RelationshipUpdate, error) {
	var persisted []persistedRelationshipUpdate
	if err := json.Unmarshal(value, &persisted); err != nil || len(persisted) != 4 {
		return nil, errors.New("persisted authorization batch is invalid")
	}
	updates := make([]ports.RelationshipUpdate, len(persisted))
	for index, update := range persisted {
		updates[index] = ports.RelationshipUpdate{
			Operation: update.Operation, ResourceType: update.ResourceType, ResourceID: update.ResourceID,
			Relation: update.Relation, SubjectType: update.SubjectType, SubjectID: update.SubjectID,
		}
		if !validRelationshipUpdate(updates[index]) {
			return nil, errors.New("persisted authorization batch is invalid")
		}
	}
	return updates, nil
}

func validRelationshipUpdate(update ports.RelationshipUpdate) bool {
	return (update.Operation == ports.AuthorizationTouch || update.Operation == ports.AuthorizationDelete) &&
		strings.TrimSpace(update.ResourceType) != "" && strings.TrimSpace(update.ResourceID) != "" &&
		strings.TrimSpace(update.Relation) != "" && strings.TrimSpace(update.SubjectType) != "" &&
		strings.TrimSpace(update.SubjectID) != ""
}

func (s *Store) ClaimAuthorizationBatches(ctx context.Context, worker string, lease time.Duration, limit int) ([]ports.AuthorizationBatch, error) {
	if strings.TrimSpace(worker) == "" || lease <= 0 || limit < 1 {
		return nil, ports.ErrInvalidArgument
	}
	var rows []authorizationBatchOutboxModel
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		earlierBatch := tx.Table("authorization_batch_outbox_models").Select("1").Where("authorization_batch_outbox_models.resource_type = candidate.resource_type AND authorization_batch_outbox_models.resource_id = candidate.resource_id AND authorization_batch_outbox_models.completed_at IS NULL AND (authorization_batch_outbox_models.created_at < candidate.created_at OR (authorization_batch_outbox_models.created_at = candidate.created_at AND authorization_batch_outbox_models.id < candidate.id))")
		earlierSingle := tx.Table("authorization_outbox_models").Select("1").Where("authorization_outbox_models.resource_type = candidate.resource_type AND authorization_outbox_models.resource_id = candidate.resource_id AND authorization_outbox_models.completed_at IS NULL AND (authorization_outbox_models.created_at < candidate.created_at OR (authorization_outbox_models.created_at = candidate.created_at AND authorization_outbox_models.id < candidate.id))")
		if err := tx.Table("authorization_batch_outbox_models AS candidate").Where("candidate.completed_at IS NULL AND (candidate.locked_until IS NULL OR candidate.locked_until <= CURRENT_TIMESTAMP)").Where("NOT EXISTS (?)", earlierBatch).Where("NOT EXISTS (?)", earlierSingle).Order("candidate.created_at ASC, candidate.id ASC").Limit(limit).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]string, len(rows))
		for index := range rows {
			ids[index] = rows[index].ID
		}
		if err := tx.Model(&authorizationBatchOutboxModel{}).Where("id IN ?", ids).Updates(map[string]any{"locked_by": worker, "locked_until": leaseExpiry(lease.Milliseconds())}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", ids).Order("created_at ASC, id ASC").Find(&rows).Error
	})
	if err != nil {
		return nil, err
	}
	result := make([]ports.AuthorizationBatch, len(rows))
	for index, row := range rows {
		batch, err := authorizationBatch(row)
		if err != nil {
			return nil, err
		}
		result[index] = batch
	}
	return result, nil
}

func (s *Store) ClaimAuthorizationBatch(ctx context.Context, id, worker string, lease time.Duration) (ports.AuthorizationBatch, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(worker) == "" || lease <= 0 {
		return ports.AuthorizationBatch{}, ports.ErrInvalidArgument
	}
	var row authorizationBatchOutboxModel
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		candidate := tx.Where("id = ? AND completed_at IS NULL AND (locked_until IS NULL OR locked_until <= CURRENT_TIMESTAMP)", id).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).First(&row)
		if errors.Is(candidate.Error, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if candidate.Error != nil {
			return candidate.Error
		}
		blocked, err := earlierAuthorizationWork(tx, row.ResourceType, row.ResourceID, row.CreatedAt, row.ID)
		if err != nil {
			return err
		}
		if blocked {
			return ports.ErrNotFound
		}
		updated := tx.Model(&authorizationBatchOutboxModel{}).Where("id = ? AND completed_at IS NULL AND (locked_until IS NULL OR locked_until <= CURRENT_TIMESTAMP)", id).Updates(map[string]any{"locked_by": worker, "locked_until": leaseExpiry(lease.Milliseconds())})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		return tx.Where("id = ?", id).First(&row).Error
	})
	if err != nil {
		return ports.AuthorizationBatch{}, err
	}
	return authorizationBatch(row)
}

func (s *Store) AuthorizationBatchCompleted(ctx context.Context, id string) (bool, error) {
	if strings.TrimSpace(id) == "" {
		return false, ports.ErrInvalidArgument
	}
	var completedAt *time.Time
	result := s.DB.WithContext(ctx).Model(&authorizationBatchOutboxModel{}).Select("completed_at").Where("id = ?", id).Scan(&completedAt)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != 1 {
		return false, ports.ErrNotFound
	}
	return completedAt != nil, nil
}

func earlierAuthorizationWork(tx *gorm.DB, resourceType, resourceID string, createdAt time.Time, id string) (bool, error) {
	var count int64
	where := "resource_type = ? AND resource_id = ? AND completed_at IS NULL AND (created_at < ? OR (created_at = ? AND id < ?))"
	if err := tx.Table("authorization_outbox_models").Where(where, resourceType, resourceID, createdAt, createdAt, id).Count(&count).Error; err != nil {
		return false, err
	}
	if count > 0 {
		return true, nil
	}
	count = 0
	if err := tx.Table("authorization_batch_outbox_models").Where(where, resourceType, resourceID, createdAt, createdAt, id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) RenewAuthorizationBatch(ctx context.Context, id, worker string, lease time.Duration) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(worker) == "" || lease <= 0 {
		return ports.ErrInvalidArgument
	}
	result := s.DB.WithContext(ctx).Model(&authorizationBatchOutboxModel{}).Where("id = ? AND locked_by = ? AND locked_until > CURRENT_TIMESTAMP AND completed_at IS NULL", id, worker).Update("locked_until", leaseExpiry(lease.Milliseconds()))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ports.ErrNotFound
	}
	return nil
}

func (s *Store) CompleteAuthorizationBatchWithAudit(ctx context.Context, id, worker string, event audit.Event) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row authorizationBatchOutboxModel
		if err := tx.Where("id = ? AND locked_by = ? AND locked_until > CURRENT_TIMESTAMP AND completed_at IS NULL", id, worker).Clauses(clause.Locking{Strength: "UPDATE"}).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if !validMutationAudit(event, audit.AuthorizationApplied, row.ResourceType, row.ResourceID, row.OwnerUserID) || event.ActorUserID != row.ActorUserID {
			return ports.ErrInvalidArgument
		}
		updated := tx.Model(&authorizationBatchOutboxModel{}).Where("id = ? AND locked_by = ? AND locked_until > CURRENT_TIMESTAMP AND completed_at IS NULL", id, worker).Updates(map[string]any{"completed_at": gorm.Expr("CURRENT_TIMESTAMP"), "locked_by": nil, "locked_until": nil, "failure_code": ""})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		return appendAuditEvent(tx, event)
	})
}

func (s *Store) FailAuthorizationBatch(ctx context.Context, id, worker, failureCode string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(worker) == "" || failureCode != "dependency_failure" {
		return ports.ErrInvalidArgument
	}
	updated := s.DB.WithContext(ctx).Model(&authorizationBatchOutboxModel{}).Where("id = ? AND locked_by = ? AND locked_until > CURRENT_TIMESTAMP AND completed_at IS NULL", id, worker).Updates(map[string]any{"attempts": gorm.Expr("attempts + 1"), "locked_by": nil, "locked_until": nil, "failure_code": failureCode})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ports.ErrNotFound
	}
	return nil
}

func authorizationBatch(row authorizationBatchOutboxModel) (ports.AuthorizationBatch, error) {
	updates, err := decodeRelationshipUpdates(row.RelationshipUpdates)
	if err != nil {
		return ports.AuthorizationBatch{}, err
	}
	batch := ports.AuthorizationBatch{
		ID: row.ID, TransferID: row.PathOwnershipTransferID, ResourceType: row.ResourceType, ResourceID: row.ResourceID,
		OwnerUserID: row.OwnerUserID, ActorUserID: row.ActorUserID, Updates: updates, Attempts: row.Attempts,
	}
	if row.LockedBy != nil {
		batch.LockedBy = *row.LockedBy
	}
	if row.LockedUntil != nil {
		batch.LockedUntil = *row.LockedUntil
	}
	return batch, nil
}
