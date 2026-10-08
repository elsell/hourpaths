package gormstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TimerSubscriptionRepository struct{ db *gorm.DB }

func NewTimerSubscriptionRepository(db *gorm.DB) *TimerSubscriptionRepository {
	return &TimerSubscriptionRepository{db: db}
}

type pathTimerSubscriptionModel struct {
	UserID, PathID       string
	Enabled              bool
	Revision             int64
	CreatedAt, UpdatedAt time.Time
}

func (pathTimerSubscriptionModel) TableName() string { return "path_timer_subscription_models" }

type timerSubscriptionReplayModel struct {
	ActorUserID, IdempotencyKey string
	RequestHash                 []byte
	FollowingUserID, PathID     *string
	ResultEnabled               bool
	ResultRevision              int64
	CreatedAt                   time.Time
}

func (timerSubscriptionReplayModel) TableName() string { return "timer_subscription_replay_models" }

func (r *TimerSubscriptionRepository) GetTimerSubscription(ctx context.Context, actor string, scope socialapp.TimerSubscriptionScope, subject string) (socialapp.TimerSubscription, error) {
	if r == nil || r.db == nil || !validOpaquePersistenceID(actor) || !validOpaquePersistenceID(subject) || !scope.Valid() {
		return socialapp.TimerSubscription{}, ports.ErrInvalidArgument
	}
	var result socialapp.TimerSubscription
	var query *gorm.DB
	if scope == socialapp.TimerSubscriptionPerson {
		query = r.db.WithContext(ctx).Table("follow_models AS subscription").
			Select("subscription.activity_notifications_enabled AS enabled, subscription.activity_notifications_revision AS revision").
			Joins("JOIN user_models actor ON actor.id = subscription.follower_user_id AND actor.status = 'active'").
			Joins("JOIN user_models subject ON subject.id = subscription.following_user_id AND subject.status = 'active'").
			Where("subscription.follower_user_id = ? AND subscription.following_user_id = ?", actor, subject).
			Where("NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id = ? AND b.blocked_user_id = ?) OR (b.blocker_user_id = ? AND b.blocked_user_id = ?))", actor, subject, subject, actor)
	} else {
		query = r.db.WithContext(ctx).Table("path_models p").
			Select("COALESCE(subscription.enabled, true) AS enabled, COALESCE(subscription.revision, 0) AS revision").
			Joins("JOIN user_models actor ON actor.id = ? AND actor.status = 'active'", actor).
			Joins("LEFT JOIN path_timer_subscription_models subscription ON subscription.path_id = p.id AND subscription.user_id = ?", actor).
			Where("p.id = ? AND (p.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models m WHERE m.path_id = p.id AND m.user_id = ? AND m.role IN ('administrator', 'participant')))", subject, actor, actor)
	}
	err := query.Take(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, ports.ErrNotFound
	}
	return result, classifyNudgeNotificationChannelError(err)
}

func (r *TimerSubscriptionRepository) UpdateTimerSubscription(ctx context.Context, command socialapp.TimerSubscriptionCommand) (socialapp.TimerSubscriptionResult, error) {
	if !validTimerSubscriptionCommand(r, command) {
		return socialapp.TimerSubscriptionResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.TimerSubscriptionResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if command.Scope == socialapp.TimerSubscriptionPath {
			if err := progresslock.Lock(tx, command.ActorUserID, command.SubjectID); err != nil {
				return err
			}
		}
		if command.Scope == socialapp.TimerSubscriptionPerson {
			if err := lockSocialPair(tx, command.ActorUserID, command.SubjectID); err != nil {
				return err
			}
		} else {
			if err := lockSocialInteractionOwner(tx, command.ActorUserID); err != nil {
				return err
			}
			var actor struct{ ID string }
			if err := tx.Table("user_models").Select("id").Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = 'active'", command.ActorUserID).Take(&actor).Error; err != nil {
				return err
			}
			var path struct{ OwnerUserID string }
			if err := tx.Table("path_models").Select("owner_user_id").Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", command.SubjectID).Take(&path).Error; err != nil {
				return err
			}
			if path.OwnerUserID != command.ActorUserID {
				var membership struct{ UserID string }
				if err := tx.Table("path_membership_models").Select("user_id").Clauses(clause.Locking{Strength: "SHARE"}).Where("path_id = ? AND user_id = ? AND role IN ('administrator', 'participant')", command.SubjectID, command.ActorUserID).Take(&membership).Error; err != nil {
					return err
				}
			}
		}
		current, err := NewTimerSubscriptionRepository(tx).GetTimerSubscription(ctx, command.ActorUserID, command.Scope, command.SubjectID)
		if err != nil {
			return err
		}
		var replay timerSubscriptionReplayModel
		err = tx.Where("actor_user_id = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) || !replayMatchesSubscription(replay, command) {
				return ports.ErrIdempotencyConflict
			}
			result = socialapp.TimerSubscriptionResult{Preference: socialapp.TimerSubscription{Enabled: replay.ResultEnabled, Revision: replay.ResultRevision}, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if current.Revision != command.ExpectedRevision {
			return ports.ErrConflict
		}
		next := socialapp.TimerSubscription{Enabled: command.Enabled, Revision: current.Revision + 1}
		if command.Scope == socialapp.TimerSubscriptionPerson {
			update := tx.Table("follow_models").Where("follower_user_id = ? AND following_user_id = ? AND activity_notifications_revision = ?", command.ActorUserID, command.SubjectID, current.Revision).
				Updates(map[string]any{"activity_notifications_enabled": next.Enabled, "activity_notifications_revision": next.Revision})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return ports.ErrConflict
			}
		} else if current.Revision == 0 {
			if err := tx.Create(&pathTimerSubscriptionModel{UserID: command.ActorUserID, PathID: command.SubjectID, Enabled: next.Enabled, Revision: next.Revision, CreatedAt: command.OccurredAt, UpdatedAt: command.OccurredAt}).Error; err != nil {
				return err
			}
		} else {
			update := tx.Model(&pathTimerSubscriptionModel{}).Where("user_id = ? AND path_id = ? AND revision = ?", command.ActorUserID, command.SubjectID, current.Revision).
				Updates(map[string]any{"enabled": next.Enabled, "revision": next.Revision, "updated_at": command.OccurredAt})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return ports.ErrConflict
			}
		}
		if !next.Enabled {
			if err := suppressUnsubscribedTimerDeliveries(tx, command.ActorUserID, command.OccurredAt); err != nil {
				return err
			}
		}
		replay = timerSubscriptionReplayModel{ActorUserID: command.ActorUserID, IdempotencyKey: command.Idempotency.Key, RequestHash: command.Idempotency.RequestHash, ResultEnabled: next.Enabled, ResultRevision: next.Revision, CreatedAt: command.OccurredAt}
		if command.Scope == socialapp.TimerSubscriptionPerson {
			replay.FollowingUserID = &command.SubjectID
		} else {
			replay.PathID = &command.SubjectID
		}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return fmt.Errorf("%w: timer subscription audit: %v", ports.ErrUnavailable, err)
		}
		result.Preference = next
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, classifyNudgeNotificationChannelError(err)
}

func replayMatchesSubscription(replay timerSubscriptionReplayModel, command socialapp.TimerSubscriptionCommand) bool {
	if replay.ResultEnabled != command.Enabled || replay.ResultRevision != command.ExpectedRevision+1 {
		return false
	}
	if command.Scope == socialapp.TimerSubscriptionPerson {
		return replay.FollowingUserID != nil && *replay.FollowingUserID == command.SubjectID && replay.PathID == nil
	}
	return replay.PathID != nil && *replay.PathID == command.SubjectID && replay.FollowingUserID == nil
}

func validTimerSubscriptionCommand(r *TimerSubscriptionRepository, command socialapp.TimerSubscriptionCommand) bool {
	return r != nil && r.db != nil && command.Scope.Valid() && validOpaquePersistenceID(command.ActorUserID) && validOpaquePersistenceID(command.SubjectID) &&
		command.ExpectedRevision >= 0 && command.ExpectedRevision < 9223372036854775807 && !command.OccurredAt.IsZero() && command.OccurredAt.Location() == time.UTC &&
		command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == socialapp.UpdateTimerSubscriptionOperation &&
		len(command.Idempotency.Key) >= 16 && len(command.Idempotency.Key) <= 128 && len(command.Idempotency.RequestHash) == sha256.Size &&
		validMutationAudit(command.Audit, audit.ResourceUpdated, "timer_subscription", command.SubjectID, command.ActorUserID) && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.OccurredAt)
}

var _ socialapp.TimerSubscriptionRepository = (*TimerSubscriptionRepository)(nil)
