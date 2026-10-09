package gormstore

import (
	"context"
	"fmt"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

func (r *SocialRelationshipRepository) ListConnections(ctx context.Context, viewer, owner string, direction socialapp.ConnectionDirection, page ports.PageRequest) (socialapp.ConnectionPage, error) {
	if !r.ready() || viewer == "" || owner == "" || !direction.Valid() || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() || page.Snapshot.Location() != time.UTC || (!page.AfterCreated.IsZero() && (page.AfterID == "" || page.AfterCreated.After(page.Snapshot))) {
		return socialapp.ConnectionPage{}, ports.ErrInvalidArgument
	}
	result := socialapp.ConnectionPage{Profiles: []domain.PublicProfile{}}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize with removal/block/privacy operations so page visibility cannot
		// outlive a concurrent relationship change between admission and projection.
		if viewer != owner {
			if err := lockSocialPair(tx, viewer, owner); err != nil {
				return err
			}
			blocked, err := socialPairBlocked(tx, viewer, owner)
			if err != nil {
				return err
			}
			if blocked {
				return ports.ErrNotFound
			}
			var follows int64
			if err = tx.Model(&socialFollowModel{}).Where("follower_user_id = ? AND following_user_id = ?", viewer, owner).Count(&follows).Error; err != nil {
				return err
			}
			if follows != 1 {
				return ports.ErrNotFound
			}
		} else {
			if err := lockSocialInteractionOwners(tx, []string{owner}); err != nil {
				return err
			}
			var active int64
			if err := tx.Table("user_models").Where("id = ? AND status = ?", owner, identity.StatusActive).Count(&active).Error; err != nil {
				return err
			}
			if active != 1 {
				return ports.ErrNotFound
			}
		}
		joined, owned := "f.follower_user_id", "f.following_user_id"
		if direction == socialapp.Following {
			joined, owned = "f.following_user_id", "f.follower_user_id"
		}
		query := tx.Table("user_models AS u").Joins("JOIN follow_models AS f ON "+joined+" = u.id").Select(fmt.Sprintf(`u.id,u.username,u.display_name,COALESCE(u.profile_picture_url,'') AS profile_picture_url,COALESCE(u.description,'') AS description,f.created_at AS connected_at,%s,%s`, socialCountsSQL, socialRelationshipSQL), viewer, viewer, viewer).
			Where(owned+" = ? AND u.status = ? AND u.username IS NOT NULL AND f.created_at <= ?", owner, identity.StatusActive, page.Snapshot).
			Where(`NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id IN ? AND b.blocked_user_id = u.id) OR (b.blocker_user_id = u.id AND b.blocked_user_id IN ?))`, []string{owner, viewer}, []string{owner, viewer})
		if !page.AfterCreated.IsZero() {
			query = query.Where("f.created_at < ? OR (f.created_at = ? AND u.id < ?)", page.AfterCreated, page.AfterCreated, page.AfterID)
		}
		var rows []struct {
			Profile     socialProfileRow `gorm:"embedded"`
			ConnectedAt time.Time
		}
		if err := query.Order("f.created_at DESC,u.id DESC").Limit(page.Limit + 1).Find(&rows).Error; err != nil {
			return err
		}
		result.HasMore = len(rows) > page.Limit
		if result.HasMore {
			rows = rows[:page.Limit]
		}
		for _, row := range rows {
			result.Profiles = append(result.Profiles, socialProfile(row.Profile))
			result.LastID = row.Profile.ID
			result.LastCreated = row.ConnectedAt.UTC()
		}
		return nil
	})
	return result, classifySocialRelationshipError(err)
}

// A removal cannot overtake an earlier grant, and a new follow cannot overtake
// its pending removal. Replays are admitted before this fence so they can finish
// their original authorization change without repeating the database mutation.
func waitForFollowerRemoval(tx *gorm.DB, owner, follower string, onlyRemoval bool) error {
	q := tx.Table("authorization_outbox_models AS a").Where("a.resource_type = 'user' AND a.resource_id = ? AND a.relation = 'follower' AND a.subject_type = 'user' AND a.subject_id = ? AND a.completed_at IS NULL", owner, follower)
	if onlyRemoval {
		q = q.Where("EXISTS (SELECT 1 FROM social_relationship_replay_models r WHERE r.authorization_change_id = a.id AND r.operation = 'social.follower.remove')")
	}
	var pending []struct{ DeadLetteredAt *time.Time }
	if err := q.Select("a.dead_lettered_at").Find(&pending).Error; err != nil {
		return err
	}
	for _, item := range pending {
		if item.DeadLetteredAt != nil {
			return ports.ErrAuthorizationDeadLettered
		}
	}
	if len(pending) > 0 {
		return ports.ErrAuthorizationPending
	}
	return nil
}

func lockVisibleSocialTargetID(tx *gorm.DB, actor, id string) (socialRelationshipUser, error) {
	if actor == id {
		return socialRelationshipUser{}, ports.ErrNotFound
	}
	if err := lockSocialPair(tx, actor, id); err != nil {
		return socialRelationshipUser{}, err
	}
	blocked, err := socialPairBlocked(tx, actor, id)
	if err != nil {
		return socialRelationshipUser{}, err
	}
	if blocked {
		return socialRelationshipUser{}, ports.ErrNotFound
	}
	var target socialRelationshipUser
	if err := tx.Table("user_models").Where("id = ? AND status = ?", id, identity.StatusActive).Take(&target).Error; err != nil {
		return socialRelationshipUser{}, err
	}
	if target.Username == nil || target.ProfileVisibility == nil {
		return socialRelationshipUser{}, ports.ErrNotFound
	}
	return target, nil
}
