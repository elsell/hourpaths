package gormstore

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PushRepository struct {
	db        *gorm.DB
	aead      cipher.AEAD
	lookupKey [32]byte
}

type pushInstallationModel struct {
	ID, OwnerUserID, Provider, Platform, Locale string
	TokenCiphertext, TokenNonce, TokenHash      []byte
	CreatedAt, UpdatedAt                        time.Time
	DeletedAt                                   *time.Time
}

func (pushInstallationModel) TableName() string { return "push_installation_models" }

type pushDeliveryClaimRow struct {
	NotificationID, InstallationID, RecipientUserID string
	Provider, Platform, Locale                      string
	TokenCiphertext, TokenNonce                     []byte
	Attempts                                        int
	CreatedAt, LockedUntil                          time.Time
	LockedBy                                        string
	ProviderTicket                                  string
}

// NewPushRepository creates the isolated push persistence adapter. key must be
// application secret material and is deliberately not read from database config.
func NewPushRepository(db *gorm.DB, key []byte) (*PushRepository, error) {
	if db == nil || len(key) < 32 {
		return nil, ports.ErrInvalidArgument
	}
	encryptionKey := derivePushKey(key, "hourpaths/push-token/encryption/v1")
	lookupKey := derivePushKey(key, "hourpaths/push-token/lookup/v1")
	block, err := aes.NewCipher(encryptionKey[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &PushRepository{db: db, aead: aead, lookupKey: lookupKey}, nil
}

func derivePushKey(key []byte, purpose string) [32]byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(purpose))
	var result [32]byte
	copy(result[:], mac.Sum(nil))
	return result
}

func (r *PushRepository) protectToken(owner, installationID, token string) ([]byte, []byte, []byte, error) {
	nonce := make([]byte, r.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, nil, err
	}
	ciphertext := r.aead.Seal(nil, nonce, []byte(token), pushTokenAAD(owner, installationID))
	mac := hmac.New(sha256.New, r.lookupKey[:])
	_, _ = mac.Write([]byte(token))
	return ciphertext, nonce, mac.Sum(nil), nil
}

func (r *PushRepository) revealToken(owner, installationID string, ciphertext, nonce []byte) (string, error) {
	plaintext, err := r.aead.Open(nil, nonce, ciphertext, pushTokenAAD(owner, installationID))
	if err != nil {
		return "", errors.New("decrypt push credential")
	}
	return string(plaintext), nil
}

func pushTokenAAD(owner, installationID string) []byte {
	return []byte(owner + "\x00" + installationID)
}

func (r *PushRepository) UpsertPushInstallation(ctx context.Context, installation ports.PushInstallation, event audit.Event) error {
	if r == nil || r.db == nil || strings.TrimSpace(installation.ID) == "" ||
		strings.TrimSpace(installation.OwnerUserID) == "" || strings.TrimSpace(installation.Token) == "" ||
		!validPushProvider(installation.Provider) || !validPushPlatform(installation.Platform) ||
		!validPushLocale(installation.Locale) || installation.CreatedAt.IsZero() ||
		installation.UpdatedAt.Before(installation.CreatedAt) ||
		!validMutationAudit(event, audit.ResourceUpdated, "push_installation", installation.ID, installation.OwnerUserID) ||
		event.ActorUserID != installation.OwnerUserID || !event.OccurredAt.Equal(installation.UpdatedAt) {
		return ports.ErrInvalidArgument
	}
	ciphertext, nonce, tokenHash, err := r.protectToken(installation.OwnerUserID, installation.ID, installation.Token)
	if err != nil {
		return err
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing pushInstallationModel
		read := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", installation.ID).First(&existing)
		switch {
		case read.Error == nil && existing.OwnerUserID != installation.OwnerUserID:
			if existing.DeletedAt != nil || !hmac.Equal(existing.TokenHash, tokenHash) {
				return ports.ErrNotFound
			}
			ids, err := pendingPushNotificationIDs(tx, installation.ID)
			if err != nil {
				return err
			}
			if err := suppressInstallationDeliveries(tx, installation.ID, installation.UpdatedAt, "installation_reassigned"); err != nil {
				return err
			}
			for _, id := range ids {
				if err := refreshNotificationPushOutbox(tx, id); err != nil {
					return err
				}
			}
			if err := tx.Model(&existing).Updates(map[string]any{
				"owner_user_id": installation.OwnerUserID, "provider": installation.Provider,
				"platform": installation.Platform, "locale": installation.Locale,
				"token_ciphertext": ciphertext, "token_nonce": nonce, "token_hash": tokenHash,
				"updated_at": installation.UpdatedAt, "deleted_at": nil,
			}).Error; err != nil {
				return translatePushWriteError(err)
			}
		case read.Error != nil && !errors.Is(read.Error, gorm.ErrRecordNotFound):
			return read.Error
		case read.Error == nil:
			updates := map[string]any{
				"provider": installation.Provider, "platform": installation.Platform, "locale": installation.Locale,
				"token_ciphertext": ciphertext, "token_nonce": nonce, "token_hash": tokenHash,
				"updated_at": installation.UpdatedAt, "deleted_at": nil,
			}
			if err := tx.Model(&existing).Updates(updates).Error; err != nil {
				return translatePushWriteError(err)
			}
		default:
			row := pushInstallationModel{
				ID: installation.ID, OwnerUserID: installation.OwnerUserID, Provider: installation.Provider,
				Platform: installation.Platform, Locale: installation.Locale, TokenCiphertext: ciphertext,
				TokenNonce: nonce, TokenHash: tokenHash, CreatedAt: installation.CreatedAt, UpdatedAt: installation.UpdatedAt,
			}
			if err := tx.Create(&row).Error; err != nil {
				return translatePushWriteError(err)
			}
		}
		return appendAuditEvent(tx, event)
	})
	return err
}

func (r *PushRepository) DeletePushInstallation(ctx context.Context, ownerUserID, installationID string, changedAt time.Time, event audit.Event) error {
	if r == nil || r.db == nil || ownerUserID == "" || installationID == "" || changedAt.IsZero() ||
		!validMutationAudit(event, audit.ResourceDeleted, "push_installation", installationID, ownerUserID) ||
		event.ActorUserID != ownerUserID || !event.OccurredAt.Equal(changedAt) {
		return ports.ErrInvalidArgument
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ids, err := pendingPushNotificationIDs(tx, installationID)
		if err != nil {
			return err
		}
		result := tx.Model(&pushInstallationModel{}).
			Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", installationID, ownerUserID).
			Updates(map[string]any{"deleted_at": changedAt, "updated_at": changedAt, "token_ciphertext": nil, "token_nonce": nil, "token_hash": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		if err := suppressInstallationDeliveries(tx, installationID, changedAt, "installation_deleted"); err != nil {
			return err
		}
		for _, id := range ids {
			if err := refreshNotificationPushOutbox(tx, id); err != nil {
				return err
			}
		}
		return appendAuditEvent(tx, event)
	})
}

func (r *PushRepository) ClaimPushDeliveries(ctx context.Context, worker string, lease time.Duration, limit int) ([]ports.PushDelivery, error) {
	if r == nil || r.db == nil || strings.TrimSpace(worker) == "" || lease <= 0 || limit < 1 || limit > 100 {
		return nil, ports.ErrInvalidArgument
	}
	var deliveries []ports.PushDelivery
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := suppressBlockedPushDeliveries(tx); err != nil {
			return err
		}
		var rows []pushDeliveryClaimRow
		if err := tx.Raw(`
WITH candidates AS (
  SELECT d.notification_id, d.installation_id
  FROM notification_push_delivery_models d
	JOIN notification_models notification
	  ON notification.id = d.notification_id
  JOIN push_installation_models i
    ON i.id = d.installation_id
   AND i.owner_user_id = d.recipient_user_id
   AND i.deleted_at IS NULL
  WHERE d.delivered_at IS NULL AND d.suppressed_at IS NULL AND d.permanently_failed_at IS NULL
    AND d.available_at <= CURRENT_TIMESTAMP
    AND (d.locked_until IS NULL OR d.locked_until <= CURRENT_TIMESTAMP)
	AND NOT EXISTS (SELECT 1 FROM block_models delivery_block
	  WHERE (delivery_block.blocker_user_id = notification.recipient_user_id AND
	         delivery_block.blocked_user_id = notification.actor_user_id)
	     OR (delivery_block.blocker_user_id = notification.actor_user_id AND
	         delivery_block.blocked_user_id = notification.recipient_user_id))
  ORDER BY d.available_at, d.created_at, d.notification_id, d.installation_id
  FOR UPDATE OF d SKIP LOCKED
  LIMIT ?
)
UPDATE notification_push_delivery_models d
SET locked_by = ?, locked_until = CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond'),
    attempts = d.attempts + 1
FROM candidates c
WHERE d.notification_id = c.notification_id AND d.installation_id = c.installation_id
RETURNING d.notification_id, d.installation_id, d.recipient_user_id,
          d.provider, d.platform, d.locale, d.token_ciphertext, d.token_nonce, d.provider_ticket,
          d.attempts, d.created_at, d.locked_by, d.locked_until`,
			limit, worker, lease.Milliseconds()).Scan(&rows).Error; err != nil {
			return err
		}
		deliveries = make([]ports.PushDelivery, 0, len(rows))
		for _, row := range rows {
			token, err := r.revealToken(row.RecipientUserID, row.InstallationID, row.TokenCiphertext, row.TokenNonce)
			if err != nil {
				return err
			}
			deliveries = append(deliveries, ports.PushDelivery{
				NotificationID: row.NotificationID, InstallationID: row.InstallationID,
				RecipientUserID: row.RecipientUserID, Provider: row.Provider, Platform: row.Platform,
				Locale: row.Locale, Token: token, Attempts: row.Attempts, CreatedAt: row.CreatedAt,
				LockedBy: row.LockedBy, LockedUntil: row.LockedUntil, ProviderTicket: row.ProviderTicket,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return deliveries, nil
}

func suppressBlockedPushDeliveries(tx *gorm.DB) error {
	blocked := tx.Table("notification_push_delivery_models AS delivery").
		Joins("JOIN notification_models notification ON notification.id = delivery.notification_id").
		Where("delivery.delivered_at IS NULL AND delivery.suppressed_at IS NULL AND delivery.permanently_failed_at IS NULL").
		Where(`EXISTS (SELECT 1 FROM block_models delivery_block
WHERE (delivery_block.blocker_user_id = notification.recipient_user_id AND
       delivery_block.blocked_user_id = notification.actor_user_id)
   OR (delivery_block.blocker_user_id = notification.actor_user_id AND
       delivery_block.blocked_user_id = notification.recipient_user_id))`)
	type blockedNotification struct{ NotificationID string }
	var rows []blockedNotification
	if err := blocked.Select("delivery.notification_id").
		Group("delivery.notification_id").
		Order("MIN(delivery.available_at), delivery.notification_id").
		Limit(100).
		Scan(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	notificationIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		notificationIDs = append(notificationIDs, row.NotificationID)
	}
	if err := tx.Table("notification_push_delivery_models").
		Where("notification_id IN ?", notificationIDs).
		Where("delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL").
		Updates(map[string]any{
			"suppressed_at": gorm.Expr("CURRENT_TIMESTAMP"), "failure_code": "blocked_relationship",
			"locked_by": nil, "locked_until": nil,
			"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil,
		}).Error; err != nil {
		return err
	}
	for _, notificationID := range notificationIDs {
		if err := refreshNotificationPushOutbox(tx, notificationID); err != nil {
			return err
		}
	}
	return nil
}

func (r *PushRepository) PushDeliveryEligible(ctx context.Context, worker, notificationID, installationID string) (bool, error) {
	if r == nil || r.db == nil || strings.TrimSpace(worker) == "" || strings.TrimSpace(notificationID) == "" || strings.TrimSpace(installationID) == "" {
		return false, ports.ErrInvalidArgument
	}
	return pushDeliveryEligible(r.db.WithContext(ctx), worker, notificationID, installationID)
}

func (r *PushRepository) HandoffPushDelivery(ctx context.Context, worker, notificationID, installationID string, send func() ports.PushTicket) (ports.PushTicket, bool, error) {
	if r == nil || r.db == nil || strings.TrimSpace(worker) == "" || strings.TrimSpace(notificationID) == "" || strings.TrimSpace(installationID) == "" || send == nil {
		return ports.PushTicket{}, false, ports.ErrInvalidArgument
	}
	var ticket ports.PushTicket
	handedOff := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var notice struct{ RecipientUserID, ActorUserID string }
		err := tx.Table("notification_push_delivery_models AS delivery").
			Select("notice.recipient_user_id, notice.actor_user_id").
			Joins("JOIN notification_models notice ON notice.id = delivery.notification_id").
			Where("delivery.notification_id = ? AND delivery.installation_id = ? AND delivery.locked_by = ? AND delivery.locked_until > CURRENT_TIMESTAMP", notificationID, installationID, worker).
			Where("delivery.delivered_at IS NULL AND delivery.suppressed_at IS NULL AND delivery.permanently_failed_at IS NULL").
			Take(&notice).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if notice.ActorUserID != notice.RecipientUserID {
			if err := lockSocialPair(tx, notice.ActorUserID, notice.RecipientUserID); err != nil {
				if errors.Is(err, ports.ErrNotFound) {
					return nil
				}
				return err
			}
		}
		eligible, err := pushDeliveryEligible(tx, worker, notificationID, installationID)
		if err != nil || !eligible {
			return err
		}
		ticket = send()
		handedOff = true
		return nil
	})
	if err != nil {
		return ports.PushTicket{}, false, err
	}
	return ticket, handedOff, nil
}

func pushDeliveryEligible(tx *gorm.DB, worker, notificationID, installationID string) (bool, error) {
	var count int64
	err := tx.Table("notification_push_delivery_models AS delivery").
		Joins("JOIN notification_models notice ON notice.id = delivery.notification_id").
		Where("delivery.notification_id = ? AND delivery.installation_id = ? AND delivery.locked_by = ? AND delivery.locked_until > CURRENT_TIMESTAMP", notificationID, installationID, worker).
		Where("delivery.delivered_at IS NULL AND delivery.suppressed_at IS NULL AND delivery.permanently_failed_at IS NULL").
		Where(`NOT EXISTS (SELECT 1 FROM block_models delivery_block
WHERE (delivery_block.blocker_user_id = notice.recipient_user_id AND delivery_block.blocked_user_id = notice.actor_user_id)
   OR (delivery_block.blocker_user_id = notice.actor_user_id AND delivery_block.blocked_user_id = notice.recipient_user_id))`).
		Count(&count).Error
	return count == 1, err
}

func (r *PushRepository) TransitionPushDelivery(ctx context.Context, worker string, transition ports.PushDeliveryTransition, event audit.Event) error {
	if r == nil || r.db == nil || strings.TrimSpace(worker) == "" ||
		transition.NotificationID == "" || transition.InstallationID == "" || transition.OccurredAt.IsZero() {
		return ports.ErrInvalidArgument
	}
	updates := map[string]any{"locked_by": nil, "locked_until": nil}
	switch transition.Outcome {
	case ports.PushDeliveryDelivered:
		if transition.FailureCode != "" {
			return ports.ErrInvalidArgument
		}
		updates["delivered_at"], updates["failure_code"] = transition.OccurredAt, ""
		erasePushCredential(updates)
	case ports.PushDeliveryAwaitingReceipt:
		if transition.ProviderTicket == "" || transition.AvailableAt.IsZero() {
			return ports.ErrInvalidArgument
		}
		updates["provider_ticket"], updates["available_at"] = transition.ProviderTicket, transition.AvailableAt
	case ports.PushDeliveryRetry:
		if transition.FailureCode == "" || transition.AvailableAt.IsZero() {
			return ports.ErrInvalidArgument
		}
		updates["failure_code"], updates["available_at"] = transition.FailureCode, transition.AvailableAt
	case ports.PushDeliverySuppressed:
		if transition.FailureCode == "" {
			return ports.ErrInvalidArgument
		}
		updates["suppressed_at"], updates["failure_code"] = transition.OccurredAt, transition.FailureCode
		erasePushCredential(updates)
	case ports.PushDeliveryPermanentlyFailed:
		if transition.FailureCode == "" {
			return ports.ErrInvalidArgument
		}
		updates["permanently_failed_at"], updates["failure_code"] = transition.OccurredAt, transition.FailureCode
		erasePushCredential(updates)
	default:
		return ports.ErrInvalidArgument
	}
	terminal := transition.Outcome == ports.PushDeliveryDelivered ||
		transition.Outcome == ports.PushDeliverySuppressed ||
		transition.Outcome == ports.PushDeliveryPermanentlyFailed
	if !terminal && event != (audit.Event{}) {
		return ports.ErrInvalidArgument
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state struct{ RecipientUserID string }
		if err := tx.Table("notification_push_delivery_models").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("recipient_user_id").
			Where(`notification_id = ? AND installation_id = ? AND locked_by = ?
				AND locked_until > CURRENT_TIMESTAMP AND delivered_at IS NULL
				AND suppressed_at IS NULL AND permanently_failed_at IS NULL`,
				transition.NotificationID, transition.InstallationID, worker).
			Take(&state).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if terminal && (!validMutationAudit(event, audit.ResourceUpdated, "notification_push_delivery",
			transition.NotificationID+"/"+transition.InstallationID, state.RecipientUserID) ||
			!event.OccurredAt.Equal(transition.OccurredAt)) {
			return ports.ErrInvalidArgument
		}
		result := tx.Table("notification_push_delivery_models").
			Where(`notification_id = ? AND installation_id = ? AND locked_by = ?
				AND locked_until > CURRENT_TIMESTAMP AND delivered_at IS NULL
				AND suppressed_at IS NULL AND permanently_failed_at IS NULL`,
				transition.NotificationID, transition.InstallationID, worker).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		if err := refreshNotificationPushOutbox(tx, transition.NotificationID); err != nil {
			return err
		}
		if terminal {
			return appendAuditEvent(tx, event)
		}
		return nil
	})
}

func (r *PushRepository) DisablePushInstallation(
	ctx context.Context,
	worker, notificationID, installationID string,
	occurredAt time.Time,
	failureCode string,
	event audit.Event,
) error {
	if r == nil || r.db == nil || worker == "" || notificationID == "" || installationID == "" ||
		occurredAt.IsZero() || failureCode == "" {
		return ports.ErrInvalidArgument
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var state struct{ RecipientUserID string }
		if err := tx.Table("notification_push_delivery_models").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Select("recipient_user_id").
			Where(`notification_id = ? AND installation_id = ? AND locked_by = ?
				AND locked_until > CURRENT_TIMESTAMP AND delivered_at IS NULL
				AND suppressed_at IS NULL AND permanently_failed_at IS NULL`,
				notificationID, installationID, worker).
			Take(&state).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if !validMutationAudit(event, audit.ResourceDeleted, "push_installation", installationID, state.RecipientUserID) ||
			!event.OccurredAt.Equal(occurredAt) {
			return ports.ErrInvalidArgument
		}
		ids, err := pendingPushNotificationIDs(tx, installationID)
		if err != nil {
			return err
		}
		result := tx.Model(&pushInstallationModel{}).
			Where("id = ? AND owner_user_id = ? AND deleted_at IS NULL", installationID, state.RecipientUserID).
			Updates(map[string]any{
				"deleted_at": occurredAt, "updated_at": occurredAt,
				"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		if err := tx.Exec(`
UPDATE notification_push_delivery_models
SET permanently_failed_at = ?, failure_code = ?, locked_by = NULL, locked_until = NULL,
    token_ciphertext = NULL, token_nonce = NULL, token_hash = NULL
WHERE installation_id = ?
  AND delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL`,
			occurredAt, failureCode, installationID).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if err := refreshNotificationPushOutbox(tx, id); err != nil {
				return err
			}
		}
		return appendAuditEvent(tx, event)
	})
}

func erasePushCredential(updates map[string]any) {
	updates["token_ciphertext"], updates["token_nonce"], updates["token_hash"] = nil, nil, nil
}

func pendingPushNotificationIDs(tx *gorm.DB, installationID string) ([]string, error) {
	var ids []string
	err := tx.Table("notification_push_delivery_models").Distinct("notification_id").
		Where(`installation_id = ? AND delivered_at IS NULL
			AND suppressed_at IS NULL AND permanently_failed_at IS NULL`, installationID).
		Pluck("notification_id", &ids).Error
	return ids, err
}

func suppressInstallationDeliveries(tx *gorm.DB, installationID string, at time.Time, code string) error {
	return tx.Exec(`
UPDATE notification_push_delivery_models
SET suppressed_at = ?, failure_code = ?, locked_by = NULL, locked_until = NULL,
    token_ciphertext = NULL, token_nonce = NULL, token_hash = NULL
WHERE installation_id = ?
  AND delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL`,
		at, code, installationID).Error
}

func refreshNotificationPushOutbox(tx *gorm.DB, notificationID string) error {
	return tx.Exec(`
UPDATE notification_push_outbox_models o
SET delivered_at = CASE WHEN EXISTS (
      SELECT 1 FROM notification_push_delivery_models d
      WHERE d.notification_id = o.notification_id AND d.delivered_at IS NOT NULL
    ) THEN (SELECT max(d.delivered_at) FROM notification_push_delivery_models d
            WHERE d.notification_id = o.notification_id) END,
    permanently_failed_at = CASE WHEN NOT EXISTS (
      SELECT 1 FROM notification_push_delivery_models d
      WHERE d.notification_id = o.notification_id AND d.delivered_at IS NOT NULL
    ) AND EXISTS (
      SELECT 1 FROM notification_push_delivery_models d
      WHERE d.notification_id = o.notification_id AND d.permanently_failed_at IS NOT NULL
    ) THEN (SELECT max(d.permanently_failed_at) FROM notification_push_delivery_models d
            WHERE d.notification_id = o.notification_id) END,
    suppressed_at = CASE WHEN NOT EXISTS (
      SELECT 1 FROM notification_push_delivery_models d
      WHERE d.notification_id = o.notification_id
        AND (d.delivered_at IS NOT NULL OR d.permanently_failed_at IS NOT NULL)
    ) THEN (SELECT max(d.suppressed_at) FROM notification_push_delivery_models d
            WHERE d.notification_id = o.notification_id) END,
    failure_code = CASE
      WHEN EXISTS (SELECT 1 FROM notification_push_delivery_models d
                   WHERE d.notification_id = o.notification_id AND d.delivered_at IS NOT NULL) THEN ''
      WHEN EXISTS (SELECT 1 FROM notification_push_delivery_models d
                   WHERE d.notification_id = o.notification_id AND d.permanently_failed_at IS NOT NULL)
        THEN 'installation_delivery_failed'
      ELSE 'all_installations_suppressed' END,
    locked_by = NULL, locked_until = NULL
WHERE o.notification_id = ?
  AND o.delivered_at IS NULL AND o.suppressed_at IS NULL AND o.permanently_failed_at IS NULL
  AND EXISTS (SELECT 1 FROM notification_push_delivery_models d WHERE d.notification_id = o.notification_id)
  AND NOT EXISTS (
    SELECT 1 FROM notification_push_delivery_models d
    WHERE d.notification_id = o.notification_id
      AND d.delivered_at IS NULL AND d.suppressed_at IS NULL AND d.permanently_failed_at IS NULL
  )`, notificationID).Error
}

func validPushProvider(value string) bool {
	return value == "expo" || value == "apns" || value == "fcm"
}
func validPushPlatform(value string) bool { return value == "ios" || value == "android" }
func validPushLocale(value string) bool   { return value == "en" || value == "es" }

func translatePushWriteError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ports.ErrConflict
	}
	return err
}
