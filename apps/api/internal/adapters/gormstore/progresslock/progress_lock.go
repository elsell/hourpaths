package progresslock

import (
	"fmt"

	"gorm.io/gorm"
)

// Lock serializes every committed progress mutation with consumers that must
// decide from a stable participant-and-Path snapshot.
func Lock(tx *gorm.DB, participantID, pathID string) error {
	key := fmt.Sprintf("activity:goal-achievement:%d:%s:%d:%s", len(participantID), participantID, len(pathID), pathID)
	return LockKey(tx, key)
}

// LockKey acquires the shared transaction-scoped advisory lock for a caller's
// deterministic key.
func LockKey(tx *gorm.DB, key string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock
}
