package gormstore

import (
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	"gorm.io/gorm"
)

func lockSocialPathAudience(tx *gorm.DB, pathID string) error {
	return progresslock.LockKey(tx, sociallock.PathAudienceKey(pathID))
}
