package notificationtimer

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"time"

	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Create joins the timer's existing transaction. Only
// application-authorized candidates can enter; subscription changes and blocks
// are rechecked after locking each receiving account.
func Create(tx *gorm.DB, timer domain.RunningTimer, recipients []string, at time.Time) error {
	if tx == nil || at.IsZero() || at.Location() != time.UTC {
		return ports.ErrInvalidArgument
	}
	if _, err := domain.StartTimer(timer.ID, timer.PathID, timer.ParticipantID, timer.StartedAt, timer.OccurrenceTimeZone, at); err != nil {
		return ports.ErrInvalidArgument
	}
	ordered := append([]string(nil), recipients...)
	slices.Sort(ordered)
	for index, recipient := range ordered {
		if !validOpaquePersistenceID(recipient) || recipient == timer.ParticipantID || index > 0 && ordered[index-1] == recipient {
			return ports.ErrInvalidArgument
		}
		enabled, err := channelstore.Allowed(tx, recipient, "tracking_activity")
		if err != nil {
			return err
		}
		if !enabled {
			continue
		}
		var count int64
		if err := timerNotificationCandidates(tx, timer.ParticipantID, timer.PathID).Where("recipient.id = ?", recipient).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			continue
		}
		digest := sha256.Sum256([]byte(timer.ID + "\x00" + recipient))
		id := "timer:" + hex.EncodeToString(digest[:])
		row := map[string]any{"id": id, "recipient_user_id": recipient, "actor_user_id": timer.ParticipantID, "path_id": timer.PathID, "timer_id": timer.ID,
			"kind": "timer_started", "presentation_class": "informational", "channel": "tracking_activity", "created_at": at}
		created := tx.Table("notification_models").Clauses(clause.OnConflict{DoNothing: true}).Create(row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			continue
		}
		if err := channelstore.QueuePush(tx, id, recipient, at); err != nil {
			return err
		}
	}
	return nil
}
