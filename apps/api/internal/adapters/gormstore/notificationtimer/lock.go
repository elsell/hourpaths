package notificationtimer

import (
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

// Lock orders timer admission, Path lifecycle, and final provider handoff before
// participant progress and receiving-account locks.
func Lock(tx *gorm.DB, path string) error {
	if tx == nil || !validOpaquePersistenceID(path) {
		return ports.ErrInvalidArgument
	}
	return progresslock.LockKey(tx, sociallock.PathAudienceKey(path))
}
