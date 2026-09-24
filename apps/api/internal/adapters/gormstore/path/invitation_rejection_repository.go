package pathstore

import (
	"bytes"
	"context"
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

type invitationPushDeliveryModel struct {
	NotificationID      string `gorm:"primaryKey"`
	InstallationID      string `gorm:"primaryKey"`
	TokenCiphertext     []byte
	TokenNonce          []byte
	TokenHash           []byte
	LockedBy            *string
	LockedUntil         *time.Time
	DeliveredAt         *time.Time
	SuppressedAt        *time.Time
	PermanentlyFailedAt *time.Time
	FailureCode         string
}

func (invitationPushDeliveryModel) TableName() string { return "notification_push_delivery_models" }

func (r *Repository) RejectionDecision(
	ctx context.Context,
	recipientUserID string,
	invitationID domain.InvitationID,
	idempotency ports.Idempotency,
) (application.RejectionDecision, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(recipientUserID) == "" || invitationID == "" ||
		!validInvitationIdempotency(idempotency, recipientUserID, application.RejectInvitationOperation) {
		return application.RejectionDecision{}, ports.ErrInvalidArgument
	}
	tx := r.DB.WithContext(ctx)
	var reservation idempotencyModel
	err := tx.Where("principal_id = ? AND operation = ? AND key = ?",
		idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&reservation).Error
	switch {
	case err == nil:
		if reservation.ResourceID != string(invitationID) || !bytes.Equal(reservation.RequestHash, idempotency.RequestHash) {
			return application.RejectionDecision{}, ports.ErrIdempotencyConflict
		}
		replay, err := rejectedInvitationReplay(tx, recipientUserID, invitationID)
		if err != nil {
			return application.RejectionDecision{}, classifyUnavailableInvitation(err)
		}
		return application.RejectionDecision{Replay: &replay}, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return application.RejectionDecision{}, err
	}

	var row invitationModel
	if err := tx.Where(
		"id = ? AND recipient_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
		invitationID, recipientUserID,
	).First(&row).Error; err != nil {
		return application.RejectionDecision{}, classifyUnavailableInvitation(err)
	}
	invitation, err := invitationFromModel(row)
	if err != nil {
		return application.RejectionDecision{}, err
	}
	var pathRow model
	if err := tx.Select("id", "owner_user_id").Where("id = ?", invitation.PathID).First(&pathRow).Error; err != nil {
		return application.RejectionDecision{}, classifyUnavailableInvitation(err)
	}
	if pathRow.ID != string(invitation.PathID) || strings.TrimSpace(pathRow.OwnerUserID) == "" {
		return application.RejectionDecision{}, errInvalidPersistedInvitation
	}
	return application.RejectionDecision{Invitation: invitation, OwnerUserID: pathRow.OwnerUserID}, nil
}

func (r *Repository) Reject(ctx context.Context, command application.RejectInvitationCommand) (application.RejectInvitationResult, error) {
	if !validRejectInvitationCommand(r, command) {
		return application.RejectInvitationResult{}, ports.ErrInvalidArgument
	}
	var result application.RejectInvitationResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{
			PrincipalID: command.Idempotency.PrincipalID, Operation: command.Idempotency.Operation,
			Key: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...),
			ResourceID: string(command.Invitation.ID), CreatedAt: command.Invitation.RejectedAt,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, err := invitationReservation(tx, command.Idempotency)
			if err != nil {
				return err
			}
			if existing.ResourceID != string(command.Invitation.ID) ||
				!bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			replay, err := rejectedInvitationReplay(tx, command.Invitation.RecipientUserID, command.Invitation.ID)
			if err != nil {
				return err
			}
			result = replay
			return nil
		}

		var row invitationModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
			"id = ? AND recipient_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
			command.Invitation.ID, command.Invitation.RecipientUserID,
		).First(&row).Error; err != nil {
			return classifyUnavailableInvitation(err)
		}
		pending, err := invitationFromModel(row)
		if err != nil || !samePendingRejection(pending, command.Invitation) {
			return domain.ErrInvitationUnavailable
		}
		var pathRow model
		if err := tx.Clauses(clause.Locking{Strength: "KEY SHARE"}).
			Select("id", "owner_user_id").Where("id = ?", command.Invitation.PathID).First(&pathRow).Error; err != nil {
			return classifyUnavailableInvitation(err)
		}
		if pathRow.OwnerUserID != command.Audit.OwnerUserID {
			return domain.ErrInvitationUnavailable
		}
		if err := tx.Model(&notificationModel{}).Where(
			"path_invitation_id = ? AND recipient_user_id = ? AND kind = ? AND deleted_at IS NULL",
			command.Invitation.ID, command.Invitation.RecipientUserID, "path_invitation_received",
		).Update("deleted_at", command.Invitation.RejectedAt).Error; err != nil {
			return err
		}
		if err := suppressRejectedInvitationPush(tx, command.Invitation.ID, command.Invitation.RejectedAt); err != nil {
			return err
		}
		count, err := visibleUnreadNotificationCount(
			tx, command.Invitation.RecipientUserID, command.Invitation.RejectedAt,
		)
		if err != nil {
			return err
		}
		updated := tx.Model(&invitationModel{}).Where(
			"id = ? AND recipient_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
			command.Invitation.ID, command.Invitation.RecipientUserID,
		).Updates(map[string]any{
			"rejected_at": command.Invitation.RejectedAt, "rejection_unread_count": count,
		})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return domain.ErrInvitationUnavailable
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.RejectInvitationResult{Invitation: command.Invitation, UnreadCount: count}
		return nil
	})
	return result, classifyInvitationPersistenceError(err)
}

func suppressRejectedInvitationPush(tx *gorm.DB, invitationID domain.InvitationID, rejectedAt time.Time) error {
	var notificationIDs []string
	if err := tx.Model(&notificationModel{}).
		Where("path_invitation_id = ? AND kind = ?", invitationID, "path_invitation_received").
		Pluck("id", &notificationIDs).Error; err != nil || len(notificationIDs) == 0 {
		return err
	}
	terminal := "delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL"
	if err := tx.Model(&invitationPushDeliveryModel{}).
		Where("notification_id IN ? AND "+terminal, notificationIDs).
		Updates(map[string]any{
			"suppressed_at": rejectedAt, "failure_code": "invitation_resolved",
			"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil,
			"locked_by": nil, "locked_until": nil,
		}).Error; err != nil {
		return err
	}
	return tx.Model(&notificationPushOutboxModel{}).
		Where("notification_id IN ? AND "+terminal, notificationIDs).
		Updates(map[string]any{
			"suppressed_at": rejectedAt, "failure_code": "invitation_resolved",
			"locked_by": nil, "locked_until": nil,
		}).Error
}

func validRejectInvitationCommand(r *Repository, command application.RejectInvitationCommand) bool {
	invitation := command.Invitation
	pending, err := domain.NewInvitation(
		invitation.ID, invitation.PathID, invitation.InviterUserID,
		invitation.RecipientUserID, invitation.OfferedRole, invitation.CreatedAt,
	)
	if err != nil {
		return false
	}
	rejected, err := pending.Reject(invitation.RecipientUserID, invitation.RejectedAt)
	return r != nil && r.DB != nil && err == nil && rejected == invitation &&
		validInvitationIdempotency(command.Idempotency, invitation.RecipientUserID, application.RejectInvitationOperation) &&
		validInvitationAudit(command.Audit, audit.PathInvitationRejected, invitation, invitation.RecipientUserID, invitation.RejectedAt)
}

func samePendingRejection(pending, rejected domain.Invitation) bool {
	return pending.ID == rejected.ID && pending.PathID == rejected.PathID &&
		pending.InviterUserID == rejected.InviterUserID &&
		pending.RecipientUserID == rejected.RecipientUserID &&
		pending.OfferedRole == rejected.OfferedRole &&
		pending.CreatedAt.Equal(rejected.CreatedAt) && rejected.AcceptedAt.IsZero() &&
		!rejected.RejectedAt.IsZero()
}

func rejectedInvitationReplay(tx *gorm.DB, recipientUserID string, invitationID domain.InvitationID) (application.RejectInvitationResult, error) {
	var row invitationModel
	if err := tx.Where(
		"id = ? AND recipient_user_id = ? AND accepted_at IS NULL AND rejected_at IS NOT NULL AND canceled_at IS NULL",
		invitationID, recipientUserID,
	).First(&row).Error; err != nil {
		return application.RejectInvitationResult{}, err
	}
	invitation, err := invitationFromModel(row)
	if err != nil || row.RejectionUnreadCount == nil || *row.RejectionUnreadCount < 0 {
		return application.RejectInvitationResult{}, errInvalidPersistedInvitation
	}
	return application.RejectInvitationResult{
		Invitation: invitation, UnreadCount: *row.RejectionUnreadCount, Replayed: true,
	}, nil
}
