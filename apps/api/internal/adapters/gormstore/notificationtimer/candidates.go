package notificationtimer

import (
	"context"
	"fmt"
	"strings"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

// TimerNotificationCandidates returns coarse subscription candidates. The
// application must check each candidate's current path view permission before
// admitting a notification, and persistence rechecks subscriptions at commit.
func Candidates(ctx context.Context, db *gorm.DB, actor, path string) ([]string, error) {
	if db == nil || !validOpaquePersistenceID(actor) || !validOpaquePersistenceID(path) {
		return nil, ports.ErrInvalidArgument
	}
	var recipients []string
	err := timerNotificationCandidates(db.WithContext(ctx), actor, path).Order("recipient.id").Pluck("recipient.id", &recipients).Error
	if err != nil {
		return nil, fmt.Errorf("%w: timer notification candidates: %v", ports.ErrUnavailable, err)
	}
	return recipients, nil
}

func timerNotificationCandidates(db *gorm.DB, actor, path string) *gorm.DB {
	return db.Table("user_models recipient").
		Joins("JOIN path_models path ON path.id = ? AND path.archived_at IS NULL", path).
		Joins("JOIN user_models actor ON actor.id = ? AND actor.status = 'active'", actor).
		Where("recipient.status = 'active' AND recipient.id <> actor.id").
		Where("(path.owner_user_id = actor.id OR EXISTS (SELECT 1 FROM path_membership_models starter WHERE starter.path_id = path.id AND starter.user_id = actor.id AND starter.role IN ('administrator', 'participant')))").
		Where("NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id = recipient.id AND b.blocked_user_id IN (actor.id, path.owner_user_id)) OR (b.blocked_user_id = recipient.id AND b.blocker_user_id IN (actor.id, path.owner_user_id)))").
		Where("NOT EXISTS (SELECT 1 FROM notification_channel_preference_models channel WHERE channel.user_id = recipient.id AND channel.channel = 'tracking_activity' AND channel.enabled = false)").
		Where(`(
          EXISTS (SELECT 1 FROM follow_models subscribed WHERE subscribed.follower_user_id = recipient.id AND subscribed.following_user_id = actor.id AND subscribed.activity_notifications_enabled)
          OR ((path.owner_user_id = recipient.id OR EXISTS (SELECT 1 FROM path_membership_models member WHERE member.path_id = path.id AND member.user_id = recipient.id AND member.role IN ('administrator', 'participant')))
            AND NOT EXISTS (SELECT 1 FROM path_timer_subscription_models preference WHERE preference.user_id = recipient.id AND preference.path_id = path.id AND preference.enabled = false))
        )`).
		Where(`(path.visibility = 'public' OR path.owner_user_id = recipient.id
          OR EXISTS (SELECT 1 FROM path_membership_models access WHERE access.path_id = path.id AND access.user_id = recipient.id)
          OR (path.visibility = 'followers' AND EXISTS (SELECT 1 FROM follow_models owner_follow WHERE owner_follow.follower_user_id = recipient.id AND owner_follow.following_user_id = path.owner_user_id)))`)
}

func Eligible(db *gorm.DB, actor, path, recipient string) (bool, error) {
	if db == nil || !validOpaquePersistenceID(actor) || !validOpaquePersistenceID(path) || !validOpaquePersistenceID(recipient) {
		return false, ports.ErrInvalidArgument
	}
	var count int64
	err := timerNotificationCandidates(db, actor, path).Where("recipient.id = ?", recipient).Count(&count).Error
	return count == 1, err
}

func validOpaquePersistenceID(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value && !strings.ContainsRune(value, '\x00')
}
