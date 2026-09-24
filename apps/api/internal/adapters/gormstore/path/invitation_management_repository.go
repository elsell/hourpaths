package pathstore

import (
	"bytes"
	"context"
	"errors"
	"strings"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type managedInvitationRow struct {
	InvitationPersistence `gorm:"embedded"`
	InviterID             string
	InviterUsername       *string
	InviterName           string
	RecipientID           string
	RecipientUsername     *string
	RecipientName         string
}

func (r *Repository) ListManagedPending(ctx context.Context, actorUserID string, pathID domain.ID, page application.InvitationPageRequest) (application.ManagedInvitationPage, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(actorUserID) == "" || pathID == "" || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() || (page.AfterID == "") != page.AfterCreated.IsZero() || (!page.AfterCreated.IsZero() && page.AfterCreated.After(page.Snapshot)) {
		return application.ManagedInvitationPage{}, ports.ErrInvalidArgument
	}
	query := r.DB.WithContext(ctx).Table("path_invitation_models").Select(`path_invitation_models.*,
      invitation_inviter.id AS inviter_id, invitation_inviter.username AS inviter_username, invitation_inviter.display_name AS inviter_name,
      invitation_recipient.id AS recipient_id, invitation_recipient.username AS recipient_username, invitation_recipient.display_name AS recipient_name`).
		Joins("JOIN path_models ON path_models.id = path_invitation_models.path_id").
		Joins("JOIN user_models AS invitation_inviter ON invitation_inviter.id = path_invitation_models.inviter_user_id AND invitation_inviter.status = ?", identity.StatusActive).
		Joins("JOIN user_models AS invitation_recipient ON invitation_recipient.id = path_invitation_models.recipient_user_id AND invitation_recipient.status = ?", identity.StatusActive).
		Where(`path_invitation_models.path_id = ? AND path_invitation_models.created_at <= ? AND path_invitation_models.accepted_at IS NULL AND path_invitation_models.rejected_at IS NULL AND path_invitation_models.canceled_at IS NULL AND (path_models.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models manager WHERE manager.path_id = path_models.id AND manager.user_id = ? AND manager.role = ?))`, pathID, page.Snapshot, actorUserID, actorUserID, domain.RoleAdministrator)
	if page.AfterID != "" {
		query = query.Where("path_invitation_models.created_at < ? OR (path_invitation_models.created_at = ? AND path_invitation_models.id < ?)", page.AfterCreated, page.AfterCreated, page.AfterID)
	}
	var rows []managedInvitationRow
	if err := query.Order("path_invitation_models.created_at DESC, path_invitation_models.id DESC").Limit(page.Limit + 1).Find(&rows).Error; err != nil {
		return application.ManagedInvitationPage{}, err
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	items := make([]application.ManagedInvitation, len(rows))
	for index, row := range rows {
		item, err := managedInvitationFromRow(row, pathID)
		if err != nil {
			return application.ManagedInvitationPage{}, err
		}
		items[index] = item
	}
	return application.ManagedInvitationPage{Items: items, HasMore: hasMore}, nil
}

func managedInvitationFromRow(row managedInvitationRow, pathID domain.ID) (application.ManagedInvitation, error) {
	invitation, err := invitationFromModel(row.InvitationPersistence)
	if err != nil || !invitation.Pending() || invitation.PathID != pathID || row.InviterID != invitation.InviterUserID || row.RecipientID != invitation.RecipientUserID || row.InviterUsername == nil || row.RecipientUsername == nil {
		return application.ManagedInvitation{}, errInvalidPersistedInvitation
	}
	item := application.ManagedInvitation{Invitation: invitation, Inviter: application.InvitationPublicIdentity{UserID: row.InviterID, Username: *row.InviterUsername, DisplayName: row.InviterName}, Recipient: application.InvitationPublicIdentity{UserID: row.RecipientID, Username: *row.RecipientUsername, DisplayName: row.RecipientName}}
	if !validManagedIdentity(item.Inviter) || !validManagedIdentity(item.Recipient) {
		return application.ManagedInvitation{}, errInvalidPersistedInvitation
	}
	return item, nil
}

func validManagedIdentity(value application.InvitationPublicIdentity) bool {
	return strings.TrimSpace(value.UserID) != "" && value.UserID == strings.TrimSpace(value.UserID) && strings.TrimSpace(value.Username) != "" && value.Username == strings.TrimSpace(value.Username) && strings.TrimSpace(value.DisplayName) != "" && value.DisplayName == strings.TrimSpace(value.DisplayName)
}

func (r *Repository) CancellationDecision(ctx context.Context, actorUserID string, pathID domain.ID, invitationID domain.InvitationID, idempotency ports.Idempotency) (application.CancellationDecision, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(actorUserID) == "" || pathID == "" || invitationID == "" || !validInvitationIdempotency(idempotency, actorUserID, application.CancelInvitationOperation) {
		return application.CancellationDecision{}, ports.ErrInvalidArgument
	}
	tx := r.DB.WithContext(ctx)
	var reservation idempotencyModel
	err := tx.Where("principal_id = ? AND operation = ? AND key = ?", idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&reservation).Error
	if err == nil {
		if reservation.ResourceID != string(invitationID) || !bytes.Equal(reservation.RequestHash, idempotency.RequestHash) {
			return application.CancellationDecision{}, ports.ErrIdempotencyConflict
		}
		replay, replayErr := canceledInvitationReplay(tx, pathID, invitationID)
		if replayErr != nil {
			return application.CancellationDecision{}, classifyUnavailableInvitation(replayErr)
		}
		return application.CancellationDecision{Replay: &replay}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return application.CancellationDecision{}, err
	}
	var row invitationModel
	if err := tx.Where("id = ? AND path_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL", invitationID, pathID).First(&row).Error; err != nil {
		return application.CancellationDecision{}, classifyUnavailableInvitation(err)
	}
	invitation, err := invitationFromModel(row)
	if err != nil {
		return application.CancellationDecision{}, err
	}
	var pathRow model
	if err := tx.Where("id = ?", pathID).First(&pathRow).Error; err != nil {
		return application.CancellationDecision{}, classifyUnavailableInvitation(err)
	}
	path, err := toEntity(pathRow)
	if err != nil {
		return application.CancellationDecision{}, err
	}
	return application.CancellationDecision{Invitation: invitation, Path: path}, nil
}

func (r *Repository) Cancel(ctx context.Context, command application.CancelInvitationCommand) (application.CancelInvitationResult, error) {
	if !validCancelInvitationCommand(r, command) {
		return application.CancelInvitationResult{}, ports.ErrInvalidArgument
	}
	var result application.CancelInvitationResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{PrincipalID: command.Idempotency.PrincipalID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ResourceID: string(command.Invitation.ID), CreatedAt: command.Invitation.CanceledAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, err := invitationReservation(tx, command.Idempotency)
			if err != nil {
				return err
			}
			if existing.ResourceID != string(command.Invitation.ID) || !bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			replay, err := canceledInvitationReplay(tx, command.Invitation.PathID, command.Invitation.ID)
			if err != nil {
				return err
			}
			result = replay
			return nil
		}
		var row invitationModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND path_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL", command.Invitation.ID, command.Invitation.PathID).First(&row).Error; err != nil {
			return classifyUnavailableInvitation(err)
		}
		pending, err := invitationFromModel(row)
		if err != nil || !samePendingCancellation(pending, command.Invitation) {
			return domain.ErrInvitationUnavailable
		}
		var pathRow model
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND archived_at IS NULL", command.Invitation.PathID).First(&pathRow).Error; err != nil {
			return classifyUnavailableInvitation(err)
		}
		if pathRow.OwnerUserID != command.ActorUserID {
			var count int64
			if err := tx.Model(&membershipModel{}).Where("path_id = ? AND user_id = ? AND role = ?", command.Invitation.PathID, command.ActorUserID, domain.RoleAdministrator).Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return domain.ErrInvitationUnavailable
			}
		}
		if pathRow.OwnerUserID != command.Audit.OwnerUserID {
			return domain.ErrInvitationUnavailable
		}
		if err := tx.Model(&notificationModel{}).Where("path_invitation_id = ? AND recipient_user_id = ? AND kind = ? AND deleted_at IS NULL", command.Invitation.ID, command.Invitation.RecipientUserID, "path_invitation_received").Update("deleted_at", command.Invitation.CanceledAt).Error; err != nil {
			return err
		}
		if err := suppressRejectedInvitationPush(tx, command.Invitation.ID, command.Invitation.CanceledAt); err != nil {
			return err
		}
		updated := tx.Model(&invitationModel{}).Where("id = ? AND path_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL", command.Invitation.ID, command.Invitation.PathID).Update("canceled_at", command.Invitation.CanceledAt)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return domain.ErrInvitationUnavailable
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.CancelInvitationResult{Invitation: command.Invitation}
		return nil
	})
	return result, classifyInvitationPersistenceError(err)
}

func validCancelInvitationCommand(r *Repository, command application.CancelInvitationCommand) bool {
	invitation := command.Invitation
	pending, err := domain.NewInvitation(invitation.ID, invitation.PathID, invitation.InviterUserID, invitation.RecipientUserID, invitation.OfferedRole, invitation.CreatedAt)
	if err != nil {
		return false
	}
	canceled, err := pending.Cancel(invitation.CanceledAt)
	return r != nil && r.DB != nil && err == nil && canceled == invitation && strings.TrimSpace(command.ActorUserID) != "" && validInvitationIdempotency(command.Idempotency, command.ActorUserID, application.CancelInvitationOperation) && validInvitationAudit(command.Audit, audit.PathInvitationCanceled, invitation, command.ActorUserID, invitation.CanceledAt)
}
func samePendingCancellation(pending, canceled domain.Invitation) bool {
	return pending.ID == canceled.ID && pending.PathID == canceled.PathID && pending.InviterUserID == canceled.InviterUserID && pending.RecipientUserID == canceled.RecipientUserID && pending.OfferedRole == canceled.OfferedRole && pending.CreatedAt.Equal(canceled.CreatedAt) && !canceled.CanceledAt.IsZero() && canceled.AcceptedAt.IsZero() && canceled.RejectedAt.IsZero()
}
func canceledInvitationReplay(tx *gorm.DB, pathID domain.ID, invitationID domain.InvitationID) (application.CancelInvitationResult, error) {
	var row invitationModel
	if err := tx.Where("id = ? AND path_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NOT NULL", invitationID, pathID).First(&row).Error; err != nil {
		return application.CancelInvitationResult{}, err
	}
	invitation, err := invitationFromModel(row)
	if err != nil {
		return application.CancelInvitationResult{}, errInvalidPersistedInvitation
	}
	return application.CancelInvitationResult{Invitation: invitation, Replayed: true}, nil
}
