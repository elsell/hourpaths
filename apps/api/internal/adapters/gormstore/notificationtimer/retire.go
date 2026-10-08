package notificationtimer

import (
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

// Retire removes queued handoffs that lost their last subscription source.
// Call inside the authoritative preference or Path-lifecycle transaction.
// Already handed-off messages are left for provider receipt reconciliation.
func Retire(tx *gorm.DB, recipient, path string, at time.Time) error {
	if tx == nil || at.IsZero() || (recipient == "" && path == "") {
		return ports.ErrInvalidArgument
	}
	var notices []struct{ ID, ActorUserID, RecipientUserID, PathID string }
	query := tx.Table("notification_models notice").Select("notice.id, notice.actor_user_id, notice.recipient_user_id, notice.path_id").Where("notice.kind = 'timer_started'").
		Where("EXISTS (SELECT 1 FROM notification_push_delivery_models delivery WHERE delivery.notification_id = notice.id AND delivery.delivered_at IS NULL AND delivery.suppressed_at IS NULL AND delivery.permanently_failed_at IS NULL AND delivery.provider_ticket = '')")
	if recipient != "" {
		query = query.Where("notice.recipient_user_id = ?", recipient)
	}
	if path != "" {
		query = query.Where("notice.path_id = ?", path)
	}
	if err := query.Order("notice.recipient_user_id, notice.id").Find(&notices).Error; err != nil {
		return err
	}
	for _, notice := range notices {
		eligible, err := Eligible(tx, notice.ActorUserID, notice.PathID, notice.RecipientUserID)
		if err != nil {
			return err
		}
		if eligible {
			continue
		}
		if err := tx.Table("notification_push_delivery_models").Where("notification_id = ? AND delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL AND provider_ticket = ''", notice.ID).
			Updates(map[string]any{"suppressed_at": gorm.Expr("GREATEST(created_at, ?)", at), "failure_code": "subscription_disabled", "locked_by": nil, "locked_until": nil, "token_ciphertext": nil, "token_nonce": nil, "token_hash": nil}).Error; err != nil {
			return err
		}
		if err := refresh(tx, notice.ID); err != nil {
			return err
		}
	}
	return nil
}
func refresh(tx *gorm.DB, id string) error {
	var state struct {
		Delivered, Failed, Suppressed *time.Time
		Pending                       int64
	}
	if err := tx.Table("notification_push_delivery_models").Select("max(delivered_at) AS delivered, max(permanently_failed_at) AS failed, max(suppressed_at) AS suppressed, count(*) FILTER (WHERE delivered_at IS NULL AND permanently_failed_at IS NULL AND suppressed_at IS NULL) AS pending").Where("notification_id = ?", id).Scan(&state).Error; err != nil {
		return err
	}
	if state.Pending != 0 {
		return nil
	}
	updates := map[string]any{"locked_by": nil, "locked_until": nil}
	if state.Delivered != nil {
		updates["delivered_at"] = state.Delivered
		updates["failure_code"] = ""
	} else if state.Failed != nil {
		updates["permanently_failed_at"] = state.Failed
		updates["failure_code"] = "installation_delivery_failed"
	} else if state.Suppressed != nil {
		updates["suppressed_at"] = state.Suppressed
		updates["failure_code"] = "all_installations_suppressed"
	} else {
		return nil
	}
	return tx.Table("notification_push_outbox_models").Where("notification_id = ? AND delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL", id).Updates(updates).Error
}
