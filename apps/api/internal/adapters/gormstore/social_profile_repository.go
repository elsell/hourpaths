package gormstore

import (
	"context"
	"fmt"
	"strings"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

type SocialProfileRepository struct{ db *gorm.DB }

func NewSocialProfileRepository(db *gorm.DB) *SocialProfileRepository {
	return &SocialProfileRepository{db: db}
}

type socialProfileRow struct {
	ID, Username, DisplayName, ProfilePictureURL, Description string
	FollowerCount, FollowingCount                             int64
	Relevance                                                 int
	Relationship                                              domain.RelationshipState
}

const socialCountsSQL = `
  (SELECT count(*) FROM follow_models f
    JOIN user_models follower ON follower.id = f.follower_user_id AND follower.status = 'active'
    WHERE f.following_user_id = u.id
      AND NOT EXISTS (SELECT 1 FROM block_models relationship_block
        WHERE (relationship_block.blocker_user_id = f.follower_user_id AND relationship_block.blocked_user_id = u.id)
           OR (relationship_block.blocker_user_id = u.id AND relationship_block.blocked_user_id = f.follower_user_id))) AS follower_count,
  (SELECT count(*) FROM follow_models f
    JOIN user_models following ON following.id = f.following_user_id AND following.status = 'active'
    WHERE f.follower_user_id = u.id
      AND NOT EXISTS (SELECT 1 FROM block_models relationship_block
        WHERE (relationship_block.blocker_user_id = u.id AND relationship_block.blocked_user_id = f.following_user_id)
           OR (relationship_block.blocker_user_id = f.following_user_id AND relationship_block.blocked_user_id = u.id))) AS following_count`

const socialRelationshipSQL = `CASE
  WHEN u.id = ? THEN 'self'
  WHEN EXISTS (SELECT 1 FROM follow_models viewer_follow
    WHERE viewer_follow.follower_user_id = ? AND viewer_follow.following_user_id = u.id) THEN 'following'
  WHEN EXISTS (SELECT 1 FROM follow_request_models viewer_request
    WHERE viewer_request.requester_user_id = ? AND viewer_request.target_user_id = u.id
      AND viewer_request.accepted_at IS NULL AND viewer_request.rejected_at IS NULL AND viewer_request.canceled_at IS NULL) THEN 'requested'
  ELSE 'none'
END AS relationship`

func (repository *SocialProfileRepository) Search(ctx context.Context, viewer, query string, afterRank int, afterUsername string, limit int) (socialapp.ProfilePage, error) {
	if repository == nil || repository.db == nil || viewer == "" || query == "" || afterRank < -1 || afterRank > 3 || limit < 1 || limit > 100 {
		return socialapp.ProfilePage{}, ports.ErrInvalidArgument
	}
	pattern := escapeLike(query)
	base := repository.db.WithContext(ctx).Table("user_models AS u").Select(fmt.Sprintf(`
  u.id, u.username, lower(u.username) AS normalized_username, u.display_name,
    COALESCE(u.profile_picture_url, '') AS profile_picture_url,
    COALESCE(u.description, '') AS description,
    CASE
      WHEN lower(u.username) = ? THEN 0
      WHEN lower(u.username) LIKE ? ESCAPE '\' THEN 1
      WHEN lower(normalize(u.display_name, NFC)) LIKE ? ESCAPE '\' THEN 2
      ELSE 3
    END AS relevance,
	%s,
	%s`, socialCountsSQL, socialRelationshipSQL), query, pattern+"%", pattern+"%", viewer, viewer, viewer).
		Where("u.status = ? AND u.username IS NOT NULL", "active").
		Where("(lower(u.username) LIKE ? ESCAPE '\\' OR lower(normalize(u.display_name, NFC)) LIKE ? ESCAPE '\\')", "%"+pattern+"%", "%"+pattern+"%").
		Where(`NOT EXISTS (SELECT 1 FROM block_models viewer_block
      WHERE (viewer_block.blocker_user_id = ? AND viewer_block.blocked_user_id = u.id)
         OR (viewer_block.blocker_user_id = u.id AND viewer_block.blocked_user_id = ?))`, viewer, viewer)
	var rows []socialProfileRow
	result := repository.db.WithContext(ctx).Table("(?) AS visible_profiles", base).
		Where("? < 0 OR relevance > ? OR (relevance = ? AND normalized_username > ?)", afterRank, afterRank, afterRank, afterUsername).
		Order("relevance ASC, normalized_username ASC").Limit(limit + 1).Find(&rows)
	if result.Error != nil {
		return socialapp.ProfilePage{}, fmt.Errorf("search profiles: %w: %v", ports.ErrUnavailable, result.Error)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	page := socialapp.ProfilePage{Profiles: make([]domain.PublicProfile, 0, len(rows)), HasMore: hasMore, LastRank: -1}
	for _, row := range rows {
		page.Profiles = append(page.Profiles, socialProfile(row))
	}
	if len(rows) > 0 {
		page.LastRank = rows[len(rows)-1].Relevance
	}
	return page, nil
}

func (repository *SocialProfileRepository) GetByUsername(ctx context.Context, viewer, username string) (domain.PublicProfile, error) {
	if repository == nil || repository.db == nil || viewer == "" || username == "" {
		return domain.PublicProfile{}, ports.ErrInvalidArgument
	}
	var row socialProfileRow
	result := repository.db.WithContext(ctx).Table("user_models AS u").Select(fmt.Sprintf(`u.id, u.username, u.display_name,
  COALESCE(u.profile_picture_url, '') AS profile_picture_url,
  COALESCE(u.description, '') AS description,
  %s,
  %s`, socialCountsSQL, socialRelationshipSQL), viewer, viewer, viewer).
		Where("u.status = ? AND lower(u.username) = ?", "active", username).
		Where(`NOT EXISTS (SELECT 1 FROM block_models viewer_block
    WHERE (viewer_block.blocker_user_id = ? AND viewer_block.blocked_user_id = u.id)
       OR (viewer_block.blocker_user_id = u.id AND viewer_block.blocked_user_id = ?))`, viewer, viewer).Limit(1).Find(&row)
	if result.Error != nil {
		return domain.PublicProfile{}, fmt.Errorf("get profile: %w: %v", ports.ErrUnavailable, result.Error)
	}
	if result.RowsAffected != 1 || row.ID == "" {
		return domain.PublicProfile{}, ports.ErrNotFound
	}
	return socialProfile(row), nil
}

func socialProfile(row socialProfileRow) domain.PublicProfile {
	return domain.PublicProfile{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName, ProfilePictureURL: row.ProfilePictureURL, Description: row.Description, FollowerCount: row.FollowerCount, FollowingCount: row.FollowingCount, Relationship: row.Relationship}
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
