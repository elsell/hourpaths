package gormstore

import (
	timerstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationtimer"
	"gorm.io/gorm"
	"time"
)

func suppressUnsubscribedTimerDeliveries(tx *gorm.DB, recipient string, at time.Time) error {
	return timerstore.Retire(tx, recipient, "", at)
}
