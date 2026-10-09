// Package pathaccess shares actor-scoped Path visibility between persistence adapters.
package pathaccess

import "gorm.io/gorm"

// Query restricts reads to owned/member Paths or visible, unblocked public/followed Paths.
// Callers must additionally enforce their application-level SpiceDB permissions.
func Query(db *gorm.DB, viewer, id string) *gorm.DB {
	if viewer == "" || id == "" {
		return db.Table("path_models").Where("1 = 0")
	}
	return db.
		Table("path_models").
		Select("path_models.*").
		Joins("LEFT JOIN path_membership_models ON path_membership_models.path_id = path_models.id AND path_membership_models.user_id = ?", viewer).
		Where(`path_models.id = ? AND (
path_models.owner_user_id = ? OR path_membership_models.user_id = ? OR (
  NOT EXISTS (SELECT 1 FROM block_models path_block
    WHERE (path_block.blocker_user_id = ? AND path_block.blocked_user_id = path_models.owner_user_id)
       OR (path_block.blocker_user_id = path_models.owner_user_id AND path_block.blocked_user_id = ?))
  AND (path_models.visibility = 'public' OR (
    path_models.visibility = 'followers' AND EXISTS (
      SELECT 1 FROM follow_models path_follow
      WHERE path_follow.follower_user_id = ? AND path_follow.following_user_id = path_models.owner_user_id
    )
  ))
))`, id, viewer, viewer, viewer, viewer, viewer)
}
