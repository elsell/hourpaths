package pathstore

import (
	"context"
	"sort"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) MarkNotificationRead(
	ctx context.Context,
	command application.NotificationMutationCommand,
) (application.NotificationMutationResult, error) {
	if !validNotificationMutation(r, command, audit.ResourceUpdated, false) {
		return application.NotificationMutationResult{}, ports.ErrInvalidArgument
	}
	return r.mutateOneNotification(ctx, command, func(tx *gorm.DB, row notificationMutationRow) error {
		if row.ReadAt != nil {
			return nil
		}
		return tx.Model(&notificationModel{}).
			Where("id = ? AND recipient_user_id = ? AND deleted_at IS NULL AND read_at IS NULL",
				command.NotificationID, command.RecipientUserID).
			Update("read_at", command.ChangedAt).Error
	})
}

func (r *Repository) DeleteNotification(
	ctx context.Context,
	command application.NotificationMutationCommand,
) (application.NotificationMutationResult, error) {
	if !validNotificationMutation(r, command, audit.ResourceDeleted, false) {
		return application.NotificationMutationResult{}, ports.ErrInvalidArgument
	}
	return r.mutateOneNotification(ctx, command, func(tx *gorm.DB, _ notificationMutationRow) error {
		updated := tx.Model(&notificationModel{}).
			Where("id = ? AND recipient_user_id = ? AND deleted_at IS NULL",
				command.NotificationID, command.RecipientUserID).
			Update("deleted_at", command.ChangedAt)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func (r *Repository) MarkAllNotificationsRead(
	ctx context.Context,
	command application.NotificationMutationCommand,
) (application.NotificationMutationResult, error) {
	if !validNotificationMutation(r, command, audit.ResourceUpdated, true) {
		return application.NotificationMutationResult{}, ports.ErrInvalidArgument
	}
	var result application.NotificationMutationResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked []struct{ ID string }
		if err := tx.Table("user_models").Select("id").Where("id = ?", command.RecipientUserID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Find(&locked).Error; err != nil {
			return err
		}
		if len(locked) != 1 {
			return gorm.ErrRecordNotFound
		}
		if err := tx.Model(&notificationModel{}).
			Where("recipient_user_id = ? AND created_at <= ? AND deleted_at IS NULL AND read_at IS NULL",
				command.RecipientUserID, command.ChangedAt).
			Where(notificationPairVisiblePredicate).
			Update("read_at", command.ChangedAt).Error; err != nil {
			return err
		}
		count, err := visibleUnreadNotificationCount(tx, command.RecipientUserID, command.ChangedAt)
		if err != nil {
			return err
		}
		result.UnreadCount = count
		return tx.Create(fromAudit(command.Audit)).Error
	})
	return result, classifyInvitationPersistenceError(err)
}

func (r *Repository) mutateOneNotification(
	ctx context.Context,
	command application.NotificationMutationCommand,
	mutate func(*gorm.DB, notificationMutationRow) error,
) (application.NotificationMutationResult, error) {
	var result application.NotificationMutationResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var subject struct{ ActorUserID string }
		if err := tx.Table("notification_models").Select("actor_user_id").Where("id = ? AND recipient_user_id = ?", command.NotificationID, command.RecipientUserID).Take(&subject).Error; err != nil {
			return err
		}
		ids := []string{command.RecipientUserID, subject.ActorUserID}
		sort.Strings(ids)
		var locked []struct{ ID string }
		if err := tx.Table("user_models").Select("id").Where("id IN ?", ids).Order("id ASC").Clauses(clause.Locking{Strength: "UPDATE"}).Find(&locked).Error; err != nil {
			return err
		}
		if len(locked) != 2 {
			return gorm.ErrRecordNotFound
		}
		var row notificationMutationRow
		if err := tx.Table("notification_models").Select("id, recipient_user_id, created_at, read_at, deleted_at").Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND recipient_user_id = ? AND deleted_at IS NULL",
				command.NotificationID, command.RecipientUserID).
			Where(notificationPairVisiblePredicate).
			First(&row).Error; err != nil {
			return err
		}
		if row.ID != command.NotificationID || row.RecipientUserID != command.RecipientUserID ||
			row.CreatedAt.IsZero() || row.CreatedAt.After(command.ChangedAt) ||
			(row.ReadAt != nil && row.ReadAt.Before(row.CreatedAt)) {
			return errInvalidPersistedInvitation
		}
		if err := mutate(tx, row); err != nil {
			return err
		}
		count, err := visibleUnreadNotificationCount(tx, command.RecipientUserID, command.ChangedAt)
		if err != nil {
			return err
		}
		result.UnreadCount = count
		return tx.Create(fromAudit(command.Audit)).Error
	})
	return result, classifyInvitationPersistenceError(err)
}

type notificationMutationRow struct {
	ID, RecipientUserID string
	CreatedAt           time.Time
	ReadAt, DeletedAt   *time.Time
}

func validNotificationMutation(
	r *Repository,
	command application.NotificationMutationCommand,
	action audit.Action,
	all bool,
) bool {
	targetID := command.NotificationID
	if all {
		targetID = "history"
	}
	return r != nil && r.DB != nil &&
		strings.TrimSpace(command.RecipientUserID) != "" &&
		strings.TrimSpace(command.NotificationID) != "" &&
		command.NotificationID == strings.TrimSpace(command.NotificationID) &&
		!command.ChangedAt.IsZero() &&
		command.Audit.Valid() &&
		command.Audit.OwnerUserID == command.RecipientUserID &&
		command.Audit.ActorUserID == command.RecipientUserID &&
		command.Audit.Action == action &&
		command.Audit.TargetType == "notification" &&
		command.Audit.TargetID == targetID &&
		command.Audit.Outcome == audit.Succeeded &&
		command.Audit.OccurredAt.Equal(command.ChangedAt)
}
