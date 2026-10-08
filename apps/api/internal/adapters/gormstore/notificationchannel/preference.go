package notificationchannel

import (
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/notification"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Allowed checks the receiving account inside the producer's existing transaction.
// A disabled category suppresses presentation, never the underlying product write.
func Allowed(tx *gorm.DB, recipient string, channel notification.Channel) (bool, error) {
	if tx == nil || recipient == "" || !channel.Valid() {
		return false, ports.ErrInvalidArgument
	}
	// The settings writer locks this same row before changing a channel and
	// retiring its queue. Read the preference only after acquiring this lock.
	var owner struct{ ID string }
	if err := tx.Table("user_models").Select("id").Clauses(clause.Locking{Strength: "KEY SHARE"}).Where("id = ?", recipient).Take(&owner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	var enabled bool
	err := tx.Table("user_models AS recipient").Select("COALESCE(preference.enabled, true)").
		Joins("LEFT JOIN notification_channel_preference_models preference ON preference.user_id = recipient.id AND preference.channel = ?", string(channel)).
		Where("recipient.id = ?", recipient).Take(&enabled).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return enabled, err
}
