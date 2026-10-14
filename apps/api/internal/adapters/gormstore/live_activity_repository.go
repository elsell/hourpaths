package gormstore

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type liveActivityModel struct {
	ID                                           string `gorm:"primaryKey"`
	OwnerUserID, AccountLease                    string
	TokenCiphertext, TokenNonce, TokenHash       []byte
	BoundariesJSON                               string `gorm:"column:boundaries_json;type:jsonb"`
	Version                                      int64
	Failures                                     int
	ExpiresAt, AvailableAt, CreatedAt, UpdatedAt time.Time
	RevokedAt, LockedUntil                       *time.Time
	LockedBy                                     *string
}

func (liveActivityModel) TableName() string { return "live_activity_registration_models" }

var _ ports.LiveActivityRegistrationRepository = (*PushRepository)(nil)
var _ ports.LiveActivityRefreshRepository = (*PushRepository)(nil)

func lockLiveActivityOwner(tx *gorm.DB, owner string) (string, error) {
	var row struct{ Status string }
	err := tx.Table("user_models").Select("status").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", owner).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ports.ErrNotFound
	}
	return row.Status, err
}

func liveActivityOwnerEligible(ctx context.Context, tx *gorm.DB, owner string, now time.Time) (bool, error) {
	status, err := lockLiveActivityOwner(tx, owner)
	if err != nil || status != "active" {
		return false, err
	}
	restriction, err := (&Store{DB: tx}).ActiveAccountRestriction(ctx, owner, now)
	return restriction == nil && err == nil, err
}

func (r *PushRepository) UpsertLiveActivity(ctx context.Context, registration liveactivity.Registration, event audit.Event) error {
	if r == nil || r.db == nil || !registration.Valid(event.OccurredAt) || !validMutationAudit(event, audit.ResourceUpdated, "live_activity_registration", registration.ID, registration.OwnerUserID) || event.ActorUserID != registration.OwnerUserID {
		return ports.ErrInvalidArgument
	}
	ciphertext, nonce, hash, err := r.protectToken(registration.OwnerUserID, "live-activity:"+registration.ID, registration.Token)
	if err != nil {
		return err
	}
	boundaries, err := json.Marshal(registration.Boundaries)
	if err != nil {
		return ports.ErrInvalidArgument
	}
	if registration.Boundaries == nil {
		boundaries = []byte("[]")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		eligible, err := liveActivityOwnerEligible(ctx, tx, registration.OwnerUserID, event.OccurredAt)
		if err != nil {
			return err
		}
		if !eligible {
			return ports.ErrNotFound
		}
		var existing liveActivityModel
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", registration.ID).Take(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		fresh := errors.Is(err, gorm.ErrRecordNotFound)
		if !fresh && (existing.OwnerUserID != registration.OwnerUserID || !existing.ExpiresAt.After(event.OccurredAt) || existing.RevokedAt != nil && existing.AccountLease == registration.AccountLease) {
			return ports.ErrNotFound
		}
		if fresh || existing.RevokedAt != nil {
			var count int64
			if err := tx.Model(&liveActivityModel{}).Where("owner_user_id=? AND revoked_at IS NULL AND expires_at>?", registration.OwnerUserID, event.OccurredAt).Count(&count).Error; err != nil {
				return err
			}
			if count >= 8 {
				return ports.ErrConflict
			}
		}
		expiry := registration.ExpiresAt
		if !fresh && expiry.After(existing.ExpiresAt) {
			expiry = existing.ExpiresAt
		}
		// Discard later boundaries when an update attempted to extend the original
		// native lifetime. The initial registration remains the expiry authority.
		clipped := []time.Time{}
		for _, boundary := range registration.Boundaries {
			if !boundary.After(expiry) {
				clipped = append(clipped, boundary)
			}
		}
		boundaries, _ = json.Marshal(clipped)
		if fresh {
			row := liveActivityModel{ID: registration.ID, OwnerUserID: registration.OwnerUserID, AccountLease: registration.AccountLease,
				TokenCiphertext: ciphertext, TokenNonce: nonce, TokenHash: hash, BoundariesJSON: string(boundaries), Version: 1,
				ExpiresAt: expiry, AvailableAt: event.OccurredAt, CreatedAt: event.OccurredAt, UpdatedAt: event.OccurredAt}
			if err := tx.Create(&row).Error; err != nil {
				return translatePushWriteError(err)
			}
		} else {
			if err := tx.Model(&existing).Updates(map[string]any{"account_lease": registration.AccountLease, "token_ciphertext": ciphertext, "token_nonce": nonce, "token_hash": hash,
				"boundaries_json": string(boundaries), "version": gorm.Expr("version + 1"), "failures": 0, "expires_at": expiry, "available_at": event.OccurredAt, "updated_at": event.OccurredAt, "revoked_at": nil, "locked_by": nil, "locked_until": nil}).Error; err != nil {
				return translatePushWriteError(err)
			}
		}
		return appendAuditEvent(tx, event)
	})
}

func (r *PushRepository) RevokeLiveActivity(ctx context.Context, owner, id, lease string, event audit.Event) error {
	if r == nil || r.db == nil || !liveactivity.ValidID(owner) || !liveactivity.ValidID(id) || !liveactivity.ValidID(lease) || !validMutationAudit(event, audit.ResourceDeleted, "live_activity_registration", id, owner) || event.ActorUserID != owner {
		return ports.ErrInvalidArgument
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockLiveActivityOwner(tx, owner); err != nil {
			return err
		}
		var row liveActivityModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_user_id=? AND account_lease=?", id, owner, lease).Take(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if err := tx.Model(&row).Updates(map[string]any{"revoked_at": event.OccurredAt, "token_ciphertext": nil, "token_nonce": nil, "token_hash": nil, "boundaries_json": "[]",
			"version": gorm.Expr("version + 1"), "updated_at": event.OccurredAt, "available_at": row.ExpiresAt, "locked_by": nil, "locked_until": nil}).Error; err != nil {
			return err
		}
		return appendAuditEvent(tx, event)
	})
}

func (r *PushRepository) ClaimLiveActivities(ctx context.Context, worker string, now time.Time, lease time.Duration, limit int) ([]ports.LiveActivityClaim, error) {
	if r == nil || r.db == nil || !liveactivity.ValidID(worker) || now.IsZero() || lease <= 0 || lease > time.Minute || limit < 1 || limit > 100 {
		return nil, ports.ErrInvalidArgument
	}
	claims := []ports.LiveActivityClaim{}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []liveActivityModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("available_at<=? AND (locked_until IS NULL OR locked_until<=?)", now, now).Order("available_at,id").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			registration := liveactivity.Registration{ID: row.ID, OwnerUserID: row.OwnerUserID, AccountLease: row.AccountLease, ExpiresAt: row.ExpiresAt}
			if err := json.Unmarshal([]byte(row.BoundariesJSON), &registration.Boundaries); err != nil {
				return ports.ErrInvalidArgument
			}
			if row.RevokedAt == nil {
				token, err := r.revealToken(row.OwnerUserID, "live-activity:"+row.ID, row.TokenCiphertext, row.TokenNonce)
				if err != nil {
					return err
				}
				registration.Token = token
			}
			if err := tx.Model(&row).Updates(map[string]any{"locked_by": worker, "locked_until": now.Add(lease)}).Error; err != nil {
				return err
			}
			claims = append(claims, ports.LiveActivityClaim{Registration: registration, Version: row.Version, Failures: row.Failures})
		}
		return nil
	})
	return claims, err
}

func (r *PushRepository) HandoffLiveActivity(ctx context.Context, worker string, claim ports.LiveActivityClaim, now time.Time, callback func(bool) ports.LiveActivityTransition) error {
	if r == nil || r.db == nil || callback == nil || !liveactivity.ValidID(worker) || !liveactivity.ValidID(claim.Registration.OwnerUserID) || !liveactivity.ValidID(claim.Registration.ID) || claim.Version < 1 || now.IsZero() {
		return ports.ErrInvalidArgument
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		eligible, err := liveActivityOwnerEligible(ctx, tx, claim.Registration.OwnerUserID, now)
		if errors.Is(err, ports.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		var row liveActivityModel
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND owner_user_id=? AND version=? AND locked_by=? AND locked_until>?", claim.Registration.ID, claim.Registration.OwnerUserID, claim.Version, worker, now).Take(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		transition := callback(eligible && row.RevokedAt == nil && row.ExpiresAt.After(now))
		event := transition.Audit
		action := audit.ResourceUpdated
		if transition.Retired {
			action = audit.ResourceDeleted
		}
		if !event.Valid() || event.Action != action || event.TargetType != "live_activity_registration" || event.TargetID != row.ID || event.OwnerUserID != row.OwnerUserID || event.ActorUserID != row.OwnerUserID || !event.OccurredAt.Equal(now) || event.Outcome == audit.Denied {
			return ports.ErrInvalidArgument
		}
		if transition.Retired && !row.ExpiresAt.After(now) {
			if err := tx.Delete(&row).Error; err != nil {
				return err
			}
		} else {
			available := transition.AvailableAt
			if available.Before(now) || available.After(row.ExpiresAt) {
				return ports.ErrInvalidArgument
			}
			updates := map[string]any{"available_at": available, "updated_at": now, "locked_by": nil, "locked_until": nil}
			if transition.Retired {
				updates["revoked_at"] = now
				updates["available_at"] = row.ExpiresAt
				updates["token_ciphertext"] = nil
				updates["token_nonce"] = nil
				updates["token_hash"] = nil
				updates["boundaries_json"] = "[]"
			} else if transition.Outcome == ports.LiveActivityAccepted {
				updates["failures"] = 0
			} else {
				updates["failures"] = gorm.Expr("failures + 1")
			}
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return err
			}
		}
		return appendAuditEvent(tx, event)
	})
}
