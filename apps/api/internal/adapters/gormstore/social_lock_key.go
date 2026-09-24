package gormstore

import sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"

func socialLockKey(namespace string, parts ...string) string {
	return sociallock.Key(namespace, parts...)
}
