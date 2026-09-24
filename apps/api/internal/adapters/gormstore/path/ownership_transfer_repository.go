package pathstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	errInvalidPersistedOwnershipTransfer                                         = errors.New("persisted Path ownership transfer is invalid")
	_                                    application.OwnershipTransferRepository = (*OwnershipTransferRepository)(nil)
)

// OwnershipTransferRepository is separate from Repository because the
// invitation and transfer ports both define Accept with different command
// types. Both adapters participate in the same PostgreSQL transaction boundary.
type OwnershipTransferRepository struct{ DB *gorm.DB }

func NewOwnershipTransferRepository(db *gorm.DB) *OwnershipTransferRepository {
	return &OwnershipTransferRepository{DB: db}
}

type ownershipTransferModel struct {
	ID              string `gorm:"primaryKey"`
	PathID          string
	InitiatorUserID string
	RecipientUserID string
	ReviewedAt      time.Time
	CreatedAt       time.Time
	ExpiresAt       time.Time
	AcceptedAt      *time.Time
	DeclinedAt      *time.Time
	CanceledAt      *time.Time
	ExpiredAt       *time.Time
}

type ownershipTransferAuthorizationBatchModel struct {
	ID, PathOwnershipTransferID, ResourceType, ResourceID string
	OwnerUserID, ActorUserID                              string
	RelationshipUpdates                                   []byte `gorm:"type:jsonb"`
	CreatedAt                                             time.Time
}

func (ownershipTransferAuthorizationBatchModel) TableName() string {
	return "authorization_batch_outbox_models"
}

type persistedOwnershipRelationshipUpdate struct {
	Operation    ports.AuthorizationOperation `json:"operation"`
	ResourceType string                       `json:"resourceType"`
	ResourceID   string                       `json:"resourceId"`
	Relation     string                       `json:"relation"`
	SubjectType  string                       `json:"subjectType"`
	SubjectID    string                       `json:"subjectId"`
}

func (ownershipTransferModel) TableName() string { return "path_ownership_transfer_models" }

func (r *OwnershipTransferRepository) ListCandidates(ctx context.Context, query application.OwnershipTransferQuery, page application.OwnershipTransferCandidatePageRequest) (application.OwnershipTransferCandidatePage, error) {
	if !validTransferCandidatePageQuery(r, query, page) {
		return application.OwnershipTransferCandidatePage{}, ports.ErrInvalidArgument
	}
	if err := requireActiveTransferCreator(r.DB.WithContext(ctx), query); err != nil {
		return application.OwnershipTransferCandidatePage{}, err
	}
	rows, err := transferCandidateRows(r.DB.WithContext(ctx), query, page)
	if err != nil {
		return application.OwnershipTransferCandidatePage{}, err
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	items, err := ownershipTransferCandidates(rows)
	if err != nil {
		return application.OwnershipTransferCandidatePage{}, err
	}
	return application.OwnershipTransferCandidatePage{Items: items, HasMore: hasMore}, nil
}

func (r *OwnershipTransferRepository) Candidate(ctx context.Context, query application.OwnershipTransferQuery, userID string) (application.OwnershipTransferCandidate, error) {
	if !validTransferRepository(r) || strings.TrimSpace(query.ActorUserID) == "" || query.PathID == "" || query.TransferID != "" || strings.TrimSpace(userID) != userID || userID == "" || userID == query.ActorUserID {
		return application.OwnershipTransferCandidate{}, ports.ErrInvalidArgument
	}
	if err := requireActiveTransferCreator(r.DB.WithContext(ctx), query); err != nil {
		return application.OwnershipTransferCandidate{}, err
	}
	rows, err := transferCandidateRows(r.DB.WithContext(ctx).Where("membership.user_id = ?", userID), query, application.OwnershipTransferCandidatePageRequest{Limit: 1, Snapshot: time.Now().UTC()})
	if err != nil {
		return application.OwnershipTransferCandidate{}, err
	}
	if len(rows) != 1 {
		return application.OwnershipTransferCandidate{}, domain.ErrOwnershipTransferUnavailable
	}
	items, err := ownershipTransferCandidates(rows)
	if err != nil {
		return application.OwnershipTransferCandidate{}, err
	}
	return items[0], nil
}

type ownershipTransferCandidateRow struct {
	UserID, DisplayName, Role string
	Username                  *string
	CreatedAt                 time.Time
}

func validTransferCandidatePageQuery(r *OwnershipTransferRepository, query application.OwnershipTransferQuery, page application.OwnershipTransferCandidatePageRequest) bool {
	return validTransferRepository(r) && strings.TrimSpace(query.ActorUserID) != "" && query.PathID != "" && query.TransferID == "" &&
		page.Limit >= 1 && page.Limit <= 100 && !page.Snapshot.IsZero() &&
		(page.AfterUserID == "") == page.AfterCreated.IsZero() && !page.AfterCreated.After(page.Snapshot)
}

func requireActiveTransferCreator(tx *gorm.DB, query application.OwnershipTransferQuery) error {
	var owner string
	if err := tx.Model(&model{}).Select("owner_user_id").Where("id = ? AND owner_user_id = ? AND archived_at IS NULL", query.PathID, query.ActorUserID).Scan(&owner).Error; err != nil {
		return err
	}
	if owner != query.ActorUserID {
		return domain.ErrOwnershipTransferUnavailable
	}
	return nil
}

func transferCandidateRows(tx *gorm.DB, query application.OwnershipTransferQuery, page application.OwnershipTransferCandidatePageRequest) ([]ownershipTransferCandidateRow, error) {
	databaseQuery := tx.Table("path_membership_models AS membership").
		Select("membership.user_id, membership.role, candidate.username, candidate.display_name, candidate.created_at").
		Joins("JOIN user_models AS candidate ON candidate.id = membership.user_id AND candidate.status = ?", "active").
		Where("membership.path_id = ? AND membership.user_id <> ? AND membership.role IN ? AND candidate.created_at <= ?", query.PathID, query.ActorUserID, []string{"participant", "administrator"}, page.Snapshot)
	if page.AfterUserID != "" {
		databaseQuery = databaseQuery.Where("candidate.created_at > ? OR (candidate.created_at = ? AND membership.user_id > ?)", page.AfterCreated, page.AfterCreated, page.AfterUserID)
	}
	var rows []ownershipTransferCandidateRow
	err := databaseQuery.Order("candidate.created_at ASC, membership.user_id ASC").Limit(page.Limit + 1).Find(&rows).Error
	return rows, err
}

func ownershipTransferCandidates(rows []ownershipTransferCandidateRow) ([]application.OwnershipTransferCandidate, error) {
	result := make([]application.OwnershipTransferCandidate, 0, len(rows))
	for _, row := range rows {
		if row.Username == nil || strings.TrimSpace(*row.Username) == "" || strings.TrimSpace(row.DisplayName) == "" || row.CreatedAt.IsZero() {
			return nil, errInvalidPersistedOwnershipTransfer
		}
		result = append(result, application.OwnershipTransferCandidate{UserID: row.UserID, Username: *row.Username, DisplayName: row.DisplayName, Administrator: row.Role == "administrator", CreatedAt: row.CreatedAt})
	}
	return result, nil
}

func (r *OwnershipTransferRepository) GetPending(ctx context.Context, query application.OwnershipTransferQuery) (application.OwnershipTransferDecision, error) {
	if !validTransferRepository(r) || strings.TrimSpace(query.ActorUserID) == "" || (query.PathID == "") == (query.TransferID == "") || !validOptionalTransferIdempotency(query.Idempotency, query.ActorUserID) {
		return application.OwnershipTransferDecision{}, ports.ErrInvalidArgument
	}
	tx := r.DB.WithContext(ctx)
	if query.Idempotency.Operation != "" {
		replay, found, err := transferReplayForReservation(tx, query.ActorUserID, query.TransferID, query.Idempotency)
		if err != nil {
			return application.OwnershipTransferDecision{}, classifyOwnershipTransferError(err)
		}
		if found {
			counterpart, role, identityErr := ownershipTransferCounterpart(tx, replay.Transfer, query.ActorUserID)
			if identityErr != nil {
				return application.OwnershipTransferDecision{}, classifyOwnershipTransferError(identityErr)
			}
			replay.Counterpart, replay.CounterpartRole = counterpart, role
			return application.OwnershipTransferDecision{Replay: &replay, Counterpart: counterpart, CounterpartRole: role}, nil
		}
	}
	lookup := tx.Where("(initiator_user_id = ? OR recipient_user_id = ?) AND accepted_at IS NULL AND declined_at IS NULL AND canceled_at IS NULL AND expired_at IS NULL AND expires_at > CURRENT_TIMESTAMP", query.ActorUserID, query.ActorUserID)
	if query.PathID != "" {
		lookup = lookup.Where("path_id = ?", query.PathID)
	} else {
		lookup = lookup.Where("id = ?", query.TransferID)
	}
	var row ownershipTransferModel
	if err := lookup.First(&row).Error; err != nil {
		return application.OwnershipTransferDecision{}, classifyOwnershipTransferError(err)
	}
	transfer, err := ownershipTransferFromModel(row)
	if err != nil {
		return application.OwnershipTransferDecision{}, err
	}
	path, participant, err := transferDecisionState(tx, transfer)
	if err != nil {
		return application.OwnershipTransferDecision{}, classifyOwnershipTransferError(err)
	}
	counterpart, role, err := ownershipTransferCounterpart(tx, transfer, query.ActorUserID)
	if err != nil {
		return application.OwnershipTransferDecision{}, classifyOwnershipTransferError(err)
	}
	return application.OwnershipTransferDecision{Transfer: transfer, Path: path, RecipientIsParticipant: participant, Counterpart: counterpart, CounterpartRole: role}, nil
}

func ownershipTransferCounterpart(tx *gorm.DB, transfer domain.OwnershipTransfer, actor string) (application.OwnershipTransferPublicIdentity, string, error) {
	userID, role := transfer.RecipientUserID, "recipient"
	if actor == transfer.RecipientUserID {
		userID, role = transfer.InitiatorUserID, "creator"
	} else if actor != transfer.InitiatorUserID {
		return application.OwnershipTransferPublicIdentity{}, "", domain.ErrOwnershipTransferUnavailable
	}
	var row struct {
		ID, DisplayName string
		Username        *string
	}
	if err := tx.Table("user_models").Select("id, username, display_name").Where("id = ?", userID).First(&row).Error; err != nil {
		return application.OwnershipTransferPublicIdentity{}, "", err
	}
	if row.Username == nil || strings.TrimSpace(*row.Username) == "" || strings.TrimSpace(row.DisplayName) == "" {
		return application.OwnershipTransferPublicIdentity{}, "", errInvalidPersistedOwnershipTransfer
	}
	return application.OwnershipTransferPublicIdentity{UserID: row.ID, Username: *row.Username, DisplayName: row.DisplayName}, role, nil
}

func (r *OwnershipTransferRepository) Initiate(ctx context.Context, command application.InitiateOwnershipTransferCommand) (application.OwnershipTransferResult, error) {
	if !validInitiateTransferCommand(r, command) {
		return application.OwnershipTransferResult{}, ports.ErrInvalidArgument
	}
	var result application.OwnershipTransferResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		replayedResourceID, err := reserveTransferInitiation(tx, command.Idempotency, string(command.Transfer.ID), command.Transfer.CreatedAt)
		if err != nil {
			return err
		}
		if replayedResourceID != "" {
			transfer, err := loadOwnershipTransfer(tx, domain.OwnershipTransferID(replayedResourceID))
			if err != nil {
				return err
			}
			result = application.OwnershipTransferResult{Transfer: transfer, Replayed: true}
			return nil
		}
		var pathRow model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", command.Transfer.PathID).First(&pathRow).Error; err != nil {
			return err
		}
		if pathRow.OwnerUserID != command.ExpectedCreatorUserID || pathRow.ArchivedAt != nil {
			return domain.ErrOwnershipTransferUnavailable
		}
		if err := expireStaleOwnershipTransfers(tx, command.Transfer.PathID, command.Transfer.CreatedAt); err != nil {
			return err
		}
		participant, err := eligibleOwnershipRecipient(tx, command.Transfer.PathID, command.Transfer.RecipientUserID)
		if err != nil {
			return err
		}
		if !participant {
			return domain.ErrOwnershipTransferUnavailable
		}
		if err := tx.Create(ownershipTransferToModel(command.Transfer)).Error; err != nil {
			return classifyOwnershipTransferError(err)
		}
		if err := createOwnershipTransferNotification(tx, command.Notification, command.Transfer); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.OwnershipTransferResult{Transfer: command.Transfer}
		return nil
	})
	return result, classifyOwnershipTransferError(err)
}

func reserveTransferInitiation(tx *gorm.DB, idempotency ports.Idempotency, resourceID string, createdAt time.Time) (string, error) {
	reservation := idempotencyModel{PrincipalID: idempotency.PrincipalID, Operation: idempotency.Operation, Key: idempotency.Key, RequestHash: append([]byte(nil), idempotency.RequestHash...), ResourceID: resourceID, CreatedAt: createdAt}
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
	if created.Error != nil {
		return "", created.Error
	}
	if created.RowsAffected == 1 {
		return "", nil
	}
	var existing idempotencyModel
	if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&existing).Error; err != nil {
		return "", err
	}
	if !bytes.Equal(existing.RequestHash, idempotency.RequestHash) {
		return "", ports.ErrIdempotencyConflict
	}
	return existing.ResourceID, nil
}

func (r *OwnershipTransferRepository) Accept(ctx context.Context, command application.AcceptOwnershipTransferCommand) (application.OwnershipTransferResult, error) {
	if !validAcceptTransferCommand(r, command) {
		return application.OwnershipTransferResult{}, ports.ErrInvalidArgument
	}
	return r.complete(ctx, command.Transfer, command.Notification, command.Idempotency, command.Audit, "accepted_at", true)
}

func (r *OwnershipTransferRepository) Decline(ctx context.Context, command application.DeclineOwnershipTransferCommand) (application.OwnershipTransferResult, error) {
	if !validTerminalTransferCommand(r, command.Transfer, command.Notification, command.Idempotency, command.Audit, application.DeclineOwnershipTransferOperation, audit.PathOwnershipTransferDeclined, command.Transfer.RecipientUserID, command.Transfer.DeclinedAt) {
		return application.OwnershipTransferResult{}, ports.ErrInvalidArgument
	}
	return r.complete(ctx, command.Transfer, command.Notification, command.Idempotency, command.Audit, "declined_at", false)
}

func (r *OwnershipTransferRepository) Cancel(ctx context.Context, command application.CancelOwnershipTransferCommand) (application.OwnershipTransferResult, error) {
	if !validTerminalTransferCommand(r, command.Transfer, command.Notification, command.Idempotency, command.Audit, application.CancelOwnershipTransferOperation, audit.PathOwnershipTransferCanceled, command.Transfer.InitiatorUserID, command.Transfer.CanceledAt) {
		return application.OwnershipTransferResult{}, ports.ErrInvalidArgument
	}
	return r.complete(ctx, command.Transfer, command.Notification, command.Idempotency, command.Audit, "canceled_at", false)
}

func (r *OwnershipTransferRepository) complete(ctx context.Context, transfer domain.OwnershipTransfer, notification application.OwnershipTransferNotification, idempotency ports.Idempotency, event audit.Event, terminalColumn string, changeOwner bool) (application.OwnershipTransferResult, error) {
	var result application.OwnershipTransferResult
	terminalAt := transferTerminalTime(transfer, terminalColumn)
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		replayed, err := reserveTransferMutation(tx, idempotency, string(transfer.ID), terminalAt)
		if err != nil {
			return err
		}
		if replayed {
			replay, found, err := transferReplayForReservation(tx, idempotency.PrincipalID, transfer.ID, idempotency)
			if err != nil {
				return err
			}
			if !found {
				return domain.ErrOwnershipTransferUnavailable
			}
			result = replay
			return nil
		}
		var pathRow model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", transfer.PathID).First(&pathRow).Error; err != nil {
			return err
		}
		if pathRow.OwnerUserID != transfer.InitiatorUserID || (changeOwner && pathRow.ArchivedAt != nil) {
			return domain.ErrOwnershipTransferUnavailable
		}
		var row ownershipTransferModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND accepted_at IS NULL AND declined_at IS NULL AND canceled_at IS NULL AND expired_at IS NULL", transfer.ID).First(&row).Error; err != nil {
			return err
		}
		pending, err := ownershipTransferFromModel(row)
		if err != nil || !samePendingOwnershipTransfer(pending, transfer) || !terminalAt.Before(pending.ExpiresAt) {
			return domain.ErrOwnershipTransferUnavailable
		}
		var pathResult domain.Entity
		if changeOwner {
			eligible, err := eligibleOwnershipRecipient(tx, transfer.PathID, transfer.RecipientUserID)
			if err != nil {
				return err
			}
			if !eligible {
				return domain.ErrOwnershipTransferUnavailable
			}
			if err := updateOwnershipMemberships(tx, transfer); err != nil {
				return err
			}
			updated := tx.Model(&model{}).Where("id = ? AND owner_user_id = ? AND archived_at IS NULL", transfer.PathID, transfer.InitiatorUserID).Updates(map[string]any{"owner_user_id": transfer.RecipientUserID, "updated_at": terminalAt})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return domain.ErrOwnershipTransferUnavailable
			}
			pathRow.OwnerUserID, pathRow.UpdatedAt = transfer.RecipientUserID, terminalAt
			pathResult, err = toEntity(pathRow)
			if err != nil {
				return err
			}
		}
		updated := tx.Model(&ownershipTransferModel{}).Where("id = ? AND accepted_at IS NULL AND declined_at IS NULL AND canceled_at IS NULL AND expired_at IS NULL", transfer.ID).Update(terminalColumn, terminalAt)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return domain.ErrOwnershipTransferUnavailable
		}
		if err := createOwnershipTransferNotification(tx, notification, transfer); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(event)).Error; err != nil {
			return err
		}
		result = application.OwnershipTransferResult{Transfer: transfer, Path: pathResult}
		if changeOwner {
			result.RelationshipUpdates = ownershipTransferRelationshipUpdates(transfer)
			batch, err := persistOwnershipTransferAuthorizationBatch(tx, transfer, result.RelationshipUpdates)
			if err != nil {
				return err
			}
			result.AuthorizationBatch = batch
		}
		return nil
	})
	return result, classifyOwnershipTransferError(err)
}

func transferDecisionState(tx *gorm.DB, transfer domain.OwnershipTransfer) (domain.Entity, bool, error) {
	var pathRow model
	if err := tx.Where("id = ?", transfer.PathID).First(&pathRow).Error; err != nil {
		return domain.Entity{}, false, err
	}
	path, err := toEntity(pathRow)
	if err != nil {
		return domain.Entity{}, false, err
	}
	participant, err := eligibleOwnershipRecipient(tx, transfer.PathID, transfer.RecipientUserID)
	return path, participant, err
}

func eligibleOwnershipRecipient(tx *gorm.DB, pathID domain.ID, recipient string) (bool, error) {
	var count int64
	err := tx.Table("path_membership_models AS membership").Joins("JOIN user_models AS recipient ON recipient.id = membership.user_id AND recipient.status = ?", "active").Where("membership.path_id = ? AND membership.user_id = ? AND membership.role IN ?", pathID, recipient, []string{"participant", "administrator"}).Count(&count).Error
	return count == 1, err
}

func updateOwnershipMemberships(tx *gorm.DB, transfer domain.OwnershipTransfer) error {
	recipient := tx.Model(&membershipModel{}).Where("path_id = ? AND user_id = ? AND role IN ?", transfer.PathID, transfer.RecipientUserID, []string{"participant", "administrator"}).Update("role", "participant")
	if recipient.Error != nil {
		return recipient.Error
	}
	if recipient.RowsAffected != 1 {
		return domain.ErrOwnershipTransferUnavailable
	}
	former := tx.Model(&membershipModel{}).Where("path_id = ? AND user_id = ?", transfer.PathID, transfer.InitiatorUserID).Update("role", "administrator")
	if former.Error != nil {
		return former.Error
	}
	if former.RowsAffected != 1 {
		return domain.ErrOwnershipTransferUnavailable
	}
	return nil
}

func reserveTransferMutation(tx *gorm.DB, idempotency ports.Idempotency, resourceID string, createdAt time.Time) (bool, error) {
	reservation := idempotencyModel{PrincipalID: idempotency.PrincipalID, Operation: idempotency.Operation, Key: idempotency.Key, RequestHash: append([]byte(nil), idempotency.RequestHash...), ResourceID: resourceID, CreatedAt: createdAt}
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
	if created.Error != nil {
		return false, created.Error
	}
	if created.RowsAffected == 1 {
		return false, nil
	}
	var existing idempotencyModel
	if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&existing).Error; err != nil {
		return false, err
	}
	if existing.ResourceID != resourceID || !bytes.Equal(existing.RequestHash, idempotency.RequestHash) {
		return false, ports.ErrIdempotencyConflict
	}
	return true, nil
}

func transferReplayForReservation(tx *gorm.DB, actor string, transferID domain.OwnershipTransferID, idempotency ports.Idempotency) (application.OwnershipTransferResult, bool, error) {
	var reservation idempotencyModel
	err := tx.Where("principal_id = ? AND operation = ? AND key = ?", idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.OwnershipTransferResult{}, false, nil
	}
	if err != nil {
		return application.OwnershipTransferResult{}, false, err
	}
	if reservation.ResourceID != string(transferID) || !bytes.Equal(reservation.RequestHash, idempotency.RequestHash) {
		return application.OwnershipTransferResult{}, false, ports.ErrIdempotencyConflict
	}
	transfer, err := loadOwnershipTransfer(tx, transferID)
	if err != nil {
		return application.OwnershipTransferResult{}, false, err
	}
	result := application.OwnershipTransferResult{Transfer: transfer, Replayed: true}
	switch idempotency.Operation {
	case application.AcceptOwnershipTransferOperation:
		if actor != transfer.RecipientUserID || transfer.AcceptedAt.IsZero() {
			return application.OwnershipTransferResult{}, false, domain.ErrOwnershipTransferUnavailable
		}
		var row model
		if err := tx.Where("id = ? AND owner_user_id = ?", transfer.PathID, transfer.RecipientUserID).First(&row).Error; err != nil {
			return application.OwnershipTransferResult{}, false, err
		}
		result.Path, err = toEntity(row)
		result.RelationshipUpdates = ownershipTransferRelationshipUpdates(transfer)
		if err == nil {
			result.AuthorizationBatch, err = loadOwnershipTransferAuthorizationBatch(tx, transfer)
		}
		return result, err == nil, err
	case application.DeclineOwnershipTransferOperation:
		if actor != transfer.RecipientUserID || transfer.DeclinedAt.IsZero() {
			return application.OwnershipTransferResult{}, false, domain.ErrOwnershipTransferUnavailable
		}
	case application.CancelOwnershipTransferOperation:
		if actor != transfer.InitiatorUserID || transfer.CanceledAt.IsZero() {
			return application.OwnershipTransferResult{}, false, domain.ErrOwnershipTransferUnavailable
		}
	default:
		return application.OwnershipTransferResult{}, false, ports.ErrInvalidArgument
	}
	return result, true, nil
}

func ownershipTransferRelationshipUpdates(transfer domain.OwnershipTransfer) []ports.RelationshipUpdate {
	resourceID := string(transfer.PathID)
	return []ports.RelationshipUpdate{
		{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: resourceID, Relation: "creator", SubjectType: "user", SubjectID: transfer.InitiatorUserID},
		{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: resourceID, Relation: "creator", SubjectType: "user", SubjectID: transfer.RecipientUserID},
		{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: resourceID, Relation: "administrator", SubjectType: "user", SubjectID: transfer.InitiatorUserID},
		{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: resourceID, Relation: "administrator", SubjectType: "user", SubjectID: transfer.RecipientUserID},
	}
}

func persistOwnershipTransferAuthorizationBatch(tx *gorm.DB, transfer domain.OwnershipTransfer, updates []ports.RelationshipUpdate) (ports.AuthorizationBatch, error) {
	batch := ports.AuthorizationBatch{
		ID: "ownership-transfer-" + string(transfer.ID), TransferID: string(transfer.ID), ResourceType: "path", ResourceID: string(transfer.PathID),
		OwnerUserID: transfer.RecipientUserID, ActorUserID: transfer.RecipientUserID, Updates: append([]ports.RelationshipUpdate(nil), updates...),
	}
	persisted := make([]persistedOwnershipRelationshipUpdate, len(updates))
	for index, update := range updates {
		persisted[index] = persistedOwnershipRelationshipUpdate{
			Operation: update.Operation, ResourceType: update.ResourceType, ResourceID: update.ResourceID,
			Relation: update.Relation, SubjectType: update.SubjectType, SubjectID: update.SubjectID,
		}
	}
	encoded, err := json.Marshal(persisted)
	if err != nil {
		return ports.AuthorizationBatch{}, err
	}
	row := ownershipTransferAuthorizationBatchModel{
		ID: batch.ID, PathOwnershipTransferID: batch.TransferID, ResourceType: batch.ResourceType, ResourceID: batch.ResourceID,
		OwnerUserID: batch.OwnerUserID, ActorUserID: batch.ActorUserID, RelationshipUpdates: encoded, CreatedAt: transfer.AcceptedAt,
	}
	if err := tx.Create(&row).Error; err != nil {
		return ports.AuthorizationBatch{}, err
	}
	return batch, nil
}

func loadOwnershipTransferAuthorizationBatch(tx *gorm.DB, transfer domain.OwnershipTransfer) (ports.AuthorizationBatch, error) {
	var row ownershipTransferAuthorizationBatchModel
	if err := tx.Where("path_ownership_transfer_id = ?", transfer.ID).First(&row).Error; err != nil {
		return ports.AuthorizationBatch{}, err
	}
	var persisted []persistedOwnershipRelationshipUpdate
	if err := json.Unmarshal(row.RelationshipUpdates, &persisted); err != nil || len(persisted) != 4 {
		return ports.AuthorizationBatch{}, errInvalidPersistedOwnershipTransfer
	}
	updates := make([]ports.RelationshipUpdate, len(persisted))
	for index, update := range persisted {
		updates[index] = ports.RelationshipUpdate{
			Operation: update.Operation, ResourceType: update.ResourceType, ResourceID: update.ResourceID,
			Relation: update.Relation, SubjectType: update.SubjectType, SubjectID: update.SubjectID,
		}
	}
	return ports.AuthorizationBatch{
		ID: row.ID, TransferID: row.PathOwnershipTransferID, ResourceType: row.ResourceType, ResourceID: row.ResourceID,
		OwnerUserID: row.OwnerUserID, ActorUserID: row.ActorUserID, Updates: updates,
	}, nil
}

func expireStaleOwnershipTransfers(tx *gorm.DB, pathID domain.ID, at time.Time) error {
	return tx.Model(&ownershipTransferModel{}).Where("path_id = ? AND accepted_at IS NULL AND declined_at IS NULL AND canceled_at IS NULL AND expired_at IS NULL AND expires_at <= ?", pathID, at).Update("expired_at", at).Error
}

func loadOwnershipTransfer(tx *gorm.DB, id domain.OwnershipTransferID) (domain.OwnershipTransfer, error) {
	var row ownershipTransferModel
	if err := tx.Where("id = ?", id).First(&row).Error; err != nil {
		return domain.OwnershipTransfer{}, err
	}
	return ownershipTransferFromModel(row)
}

func ownershipTransferToModel(transfer domain.OwnershipTransfer) *ownershipTransferModel {
	row := &ownershipTransferModel{ID: string(transfer.ID), PathID: string(transfer.PathID), InitiatorUserID: transfer.InitiatorUserID, RecipientUserID: transfer.RecipientUserID, ReviewedAt: transfer.ReviewedAt, CreatedAt: transfer.CreatedAt, ExpiresAt: transfer.ExpiresAt}
	if !transfer.AcceptedAt.IsZero() {
		row.AcceptedAt = pointer(transfer.AcceptedAt.UTC())
	}
	if !transfer.DeclinedAt.IsZero() {
		row.DeclinedAt = pointer(transfer.DeclinedAt.UTC())
	}
	if !transfer.CanceledAt.IsZero() {
		row.CanceledAt = pointer(transfer.CanceledAt.UTC())
	}
	return row
}

func ownershipTransferFromModel(row ownershipTransferModel) (domain.OwnershipTransfer, error) {
	transfer, err := domain.NewReviewedOwnershipTransfer(domain.OwnershipTransferID(row.ID), domain.ID(row.PathID), row.InitiatorUserID, row.RecipientUserID, row.ReviewedAt.UTC(), row.CreatedAt.UTC(), row.ExpiresAt.UTC())
	if err != nil {
		return domain.OwnershipTransfer{}, errInvalidPersistedOwnershipTransfer
	}
	terminalCount := 0
	for _, terminal := range []*time.Time{row.AcceptedAt, row.DeclinedAt, row.CanceledAt, row.ExpiredAt} {
		if terminal != nil {
			terminalCount++
		}
	}
	if terminalCount > 1 {
		return domain.OwnershipTransfer{}, errInvalidPersistedOwnershipTransfer
	}
	if row.AcceptedAt != nil {
		transfer.AcceptedAt = row.AcceptedAt.UTC()
	}
	if row.DeclinedAt != nil {
		transfer.DeclinedAt = row.DeclinedAt.UTC()
	}
	if row.CanceledAt != nil {
		transfer.CanceledAt = row.CanceledAt.UTC()
	}
	return transfer, nil
}

func validInitiateTransferCommand(r *OwnershipTransferRepository, command application.InitiateOwnershipTransferCommand) bool {
	transfer := command.Transfer
	pending, err := domain.NewReviewedOwnershipTransfer(transfer.ID, transfer.PathID, transfer.InitiatorUserID, transfer.RecipientUserID, transfer.ReviewedAt, transfer.CreatedAt, transfer.ExpiresAt)
	return validTransferRepository(r) && err == nil && pending == transfer && command.ExpectedCreatorUserID == transfer.InitiatorUserID && command.RequireActivePath && command.RequireRecipientParticipant && command.RequireNoPendingForPath && validPersistedOwnershipTransferNotification(command.Notification, transfer) && validTransferIdempotency(command.Idempotency, transfer.InitiatorUserID, application.InitiateOwnershipTransferOperation) && validOwnershipTransferAudit(command.Audit, audit.PathOwnershipTransferCreated, transfer, transfer.InitiatorUserID, transfer.CreatedAt)
}

func validAcceptTransferCommand(r *OwnershipTransferRepository, command application.AcceptOwnershipTransferCommand) bool {
	transfer := command.Transfer
	pending, err := domain.NewReviewedOwnershipTransfer(transfer.ID, transfer.PathID, transfer.InitiatorUserID, transfer.RecipientUserID, transfer.ReviewedAt, transfer.CreatedAt, transfer.ExpiresAt)
	accepted, transitionErr := pending.Accept(transfer.RecipientUserID, transfer.AcceptedAt)
	return validTransferRepository(r) && err == nil && transitionErr == nil && accepted == transfer && command.ExpectedCreatorUserID == transfer.InitiatorUserID && command.RequireActivePath && command.RequireRecipientParticipant && command.PromoteRecipientToCreator && command.RetainFormerCreatorAsAdministrator && command.PreserveMembershipAndActivity && validPersistedOwnershipTransferNotification(command.Notification, transfer) && validTransferIdempotency(command.Idempotency, transfer.RecipientUserID, application.AcceptOwnershipTransferOperation) && validOwnershipTransferAudit(command.Audit, audit.PathOwnershipTransferAccepted, transfer, transfer.RecipientUserID, transfer.AcceptedAt)
}

func validTerminalTransferCommand(r *OwnershipTransferRepository, transfer domain.OwnershipTransfer, notification application.OwnershipTransferNotification, idempotency ports.Idempotency, event audit.Event, operation string, action audit.Action, actor string, at time.Time) bool {
	pending, err := domain.NewReviewedOwnershipTransfer(transfer.ID, transfer.PathID, transfer.InitiatorUserID, transfer.RecipientUserID, transfer.ReviewedAt, transfer.CreatedAt, transfer.ExpiresAt)
	if err != nil {
		return false
	}
	var terminal domain.OwnershipTransfer
	if operation == application.DeclineOwnershipTransferOperation {
		terminal, err = pending.Decline(actor, at)
	} else {
		terminal, err = pending.Cancel(actor, at)
	}
	return validTransferRepository(r) && err == nil && terminal == transfer && validPersistedOwnershipTransferNotification(notification, transfer) && validTransferIdempotency(idempotency, actor, operation) && validOwnershipTransferAudit(event, action, transfer, actor, at)
}

func validOwnershipTransferAudit(event audit.Event, action audit.Action, transfer domain.OwnershipTransfer, actor string, at time.Time) bool {
	return event.Valid() && event.Action == action && event.Outcome == audit.Succeeded && event.TargetType == "path_ownership_transfer" && event.TargetID == string(transfer.ID) && event.OwnerUserID == transfer.InitiatorUserID && event.ActorUserID == actor && event.OccurredAt.Equal(at)
}

func validTransferRepository(r *OwnershipTransferRepository) bool { return r != nil && r.DB != nil }

func validTransferIdempotency(idempotency ports.Idempotency, principal, operation string) bool {
	return idempotency.PrincipalID == principal && idempotency.Operation == operation && strings.TrimSpace(idempotency.Key) != "" && len(idempotency.RequestHash) == 32
}

func validOptionalTransferIdempotency(idempotency ports.Idempotency, actor string) bool {
	if idempotency.PrincipalID == "" && idempotency.Operation == "" && idempotency.Key == "" && len(idempotency.RequestHash) == 0 {
		return true
	}
	return validTransferIdempotency(idempotency, actor, idempotency.Operation) && (idempotency.Operation == application.AcceptOwnershipTransferOperation || idempotency.Operation == application.DeclineOwnershipTransferOperation || idempotency.Operation == application.CancelOwnershipTransferOperation)
}

func samePendingOwnershipTransfer(pending, terminal domain.OwnershipTransfer) bool {
	return pending.ID == terminal.ID && pending.PathID == terminal.PathID && pending.InitiatorUserID == terminal.InitiatorUserID && pending.RecipientUserID == terminal.RecipientUserID && pending.CreatedAt.Equal(terminal.CreatedAt) && pending.ExpiresAt.Equal(terminal.ExpiresAt)
}

func transferTerminalTime(transfer domain.OwnershipTransfer, column string) time.Time {
	switch column {
	case "accepted_at":
		return transfer.AcceptedAt
	case "declined_at":
		return transfer.DeclinedAt
	default:
		return transfer.CanceledAt
	}
}

type sqlStateError interface{ SQLState() string }

func classifyOwnershipTransferError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrOwnershipTransferUnavailable
	}
	var state sqlStateError
	if errors.As(err, &state) && state.SQLState() == "23505" {
		return domain.ErrOwnershipTransferUnavailable
	}
	return err
}
