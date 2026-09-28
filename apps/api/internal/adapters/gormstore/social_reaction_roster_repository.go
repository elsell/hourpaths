package gormstore

import (
	"context"
	"fmt"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (repository *SocialFeedRepository) ListPracticeReactions(ctx context.Context, viewer, eventID string, reaction socialdomain.Reaction, request socialapp.ReactionRosterPageRequest) (socialapp.ReactionRosterPage, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(viewer) || !validOpaquePersistenceID(eventID) || !reaction.ValidStored() || !validCommentHeartPageRequest(socialapp.CommentHeartRosterPageRequest{AfterUserID: request.AfterUserID, AfterCreated: request.AfterCreated, Snapshot: request.Snapshot, Limit: request.Limit}) {
		return socialapp.ReactionRosterPage{}, ports.ErrInvalidArgument
	}
	db := repository.db.WithContext(ctx)
	target, err := resolvePracticeReactionTarget(db, viewer, eventID)
	if err != nil {
		return socialapp.ReactionRosterPage{}, classifySocialCommentError(err)
	}
	enabled, err := socialInteractionEnabled(db, target.OwnerUserID, "reactions_enabled")
	if err != nil {
		return socialapp.ReactionRosterPage{}, classifySocialCommentError(err)
	}
	if !enabled {
		return socialapp.ReactionRosterPage{}, ports.ErrNotFound
	}
	type row struct {
		UserID, Username, DisplayName, ProfilePictureURL, Description string
		FollowerCount, FollowingCount                                 int64
		Relationship                                                  socialdomain.RelationshipState
		CreatedAt                                                     time.Time
	}
	var rows []row
	query := db.Table("social_practice_reaction_models AS heart").Select(fmt.Sprintf(`heart.actor_user_id AS user_id, heart.updated_at AS created_at,
u.username, u.display_name, COALESCE(u.profile_picture_url, '') AS profile_picture_url, COALESCE(u.description, '') AS description,
%s, %s`, socialCountsSQL, socialRelationshipSQL), viewer, viewer, viewer).
		Joins("JOIN user_models u ON u.id = heart.actor_user_id AND u.status = 'active' AND u.username IS NOT NULL").
		Where("heart.social_feed_event_id = ? AND heart.reaction_type = ? AND heart.updated_at <= ?", eventID, reaction, request.Snapshot).
		Where(`NOT EXISTS (SELECT 1 FROM block_models heart_block
WHERE (heart_block.blocker_user_id = ? AND heart_block.blocked_user_id = heart.actor_user_id)
   OR (heart_block.blocker_user_id = heart.actor_user_id AND heart_block.blocked_user_id = ?))`, viewer, viewer)
	if request.AfterUserID != "" {
		query = query.Where("heart.updated_at > ? OR (heart.updated_at = ? AND heart.actor_user_id > ?)", request.AfterCreated, request.AfterCreated, request.AfterUserID)
	}
	if err := query.Order("heart.updated_at ASC, heart.actor_user_id ASC").Limit(request.Limit + 1).Scan(&rows).Error; err != nil {
		return socialapp.ReactionRosterPage{}, classifySocialCommentError(err)
	}
	hasMore := len(rows) > request.Limit
	if hasMore {
		rows = rows[:request.Limit]
	}
	page := socialapp.ReactionRosterPage{Items: make([]socialapp.ReactionRosterItem, 0, len(rows)), HasMore: hasMore}
	for _, row := range rows {
		page.Items = append(page.Items, socialapp.ReactionRosterItem{Profile: socialdomain.PublicProfile{ID: row.UserID, Username: row.Username, DisplayName: row.DisplayName, ProfilePictureURL: row.ProfilePictureURL, Description: row.Description, FollowerCount: row.FollowerCount, FollowingCount: row.FollowingCount, Relationship: row.Relationship}, ReactedAt: row.CreatedAt.UTC()})
	}
	return page, nil
}
