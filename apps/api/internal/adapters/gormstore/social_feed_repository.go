package gormstore

import (
	"context"
	"fmt"
	"strings"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

// SocialFeedRepository returns current coarse-eligible candidates. The
// application layer remains responsible for the authoritative SpiceDB check on
// every candidate before exposure.
type SocialFeedRepository struct{ db *gorm.DB }

func NewSocialFeedRepository(db *gorm.DB) *SocialFeedRepository {
	return &SocialFeedRepository{db: db}
}

func (repository *SocialFeedRepository) ListPracticeCandidates(ctx context.Context, viewer string, page socialapp.FeedPageRequest) (socialapp.FeedCandidatePage, error) {
	return repository.listPracticeCandidates(ctx, viewer, page, "")
}

func (repository *SocialFeedRepository) GetPracticeCandidate(ctx context.Context, viewer, eventID string, snapshot time.Time) (socialapp.PracticeFeedItem, error) {
	if !validOpaquePersistenceID(eventID) {
		return socialapp.PracticeFeedItem{}, ports.ErrInvalidArgument
	}
	page, err := repository.listPracticeCandidates(ctx, viewer, socialapp.FeedPageRequest{Snapshot: snapshot, Limit: 1}, eventID)
	if err != nil {
		return socialapp.PracticeFeedItem{}, err
	}
	if len(page.Items) != 1 {
		return socialapp.PracticeFeedItem{}, ports.ErrNotFound
	}
	return page.Items[0], nil
}

func (repository *SocialFeedRepository) listPracticeCandidates(ctx context.Context, viewer string, page socialapp.FeedPageRequest, eventID string) (socialapp.FeedCandidatePage, error) {
	if repository == nil || repository.db == nil || strings.TrimSpace(viewer) == "" || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() || page.Snapshot.Location() != time.UTC ||
		(page.AfterID == "") != page.AfterPublished.IsZero() || (!page.AfterPublished.IsZero() && (page.AfterPublished.Location() != time.UTC || page.AfterPublished.After(page.Snapshot))) {
		return socialapp.FeedCandidatePage{}, ports.ErrInvalidArgument
	}

	type feedCandidateRow struct {
		ID, EventType, ParticipantID, Username, DisplayName, ProfilePictureURL string
		PathID, PathName, ActivityID                                           string
		AchievementKind                                                        string
		AchievementTargetSeconds                                               *int64
		IntervalStartedAt, IntervalEndedAt                                     *time.Time
		PublishedAt                                                            time.Time
		DurationSeconds                                                        int64
		Edited                                                                 bool
		HeartCount, ApplauseCount, FireCount, StrongCount, CelebrateCount      int64
		ViewerReaction                                                         string
		CommentsEnabled, ReactionsEnabled                                      bool
	}
	var rows []feedCandidateRow
	query := repository.db.WithContext(ctx).Table("social_feed_event_models AS event").
		Select(`event.id, event.published_at,
  CASE WHEN event.source_activity_id IS NOT NULL THEN 'practice_session' ELSE 'goal_achievement' END AS event_type,
  event.participant_user_id AS participant_id,
  participant.username, participant.display_name,
  COALESCE(participant.profile_picture_url, '') AS profile_picture_url,
  event.path_id, path.name AS path_name, COALESCE(activity.id, '') AS activity_id,
  COALESCE(FLOOR(EXTRACT(EPOCH FROM (activity.ended_at - activity.started_at))), 0)::bigint AS duration_seconds,
  CASE WHEN activity.id IS NULL THEN false ELSE EXISTS (
    SELECT 1 FROM recorded_activity_revision_models public_edit
    WHERE public_edit.activity_id = activity.id AND public_edit.public_changed
  ) END AS edited,
  COALESCE(achievement.kind, '') AS achievement_kind,
  achievement.target_seconds AS achievement_target_seconds,
  achievement.interval_started_at, achievement.interval_ended_at,
	COALESCE(interaction_settings.comments_enabled, true) AS comments_enabled,
	COALESCE(interaction_settings.reactions_enabled, true) AS reactions_enabled,
  COUNT(reaction.actor_user_id) FILTER (WHERE reaction.reaction_type = 'heart') AS heart_count,
  COUNT(reaction.actor_user_id) FILTER (WHERE reaction.reaction_type = 'applause') AS applause_count,
  COUNT(reaction.actor_user_id) FILTER (WHERE reaction.reaction_type = 'fire') AS fire_count,
  COUNT(reaction.actor_user_id) FILTER (WHERE reaction.reaction_type = 'strong') AS strong_count,
  COUNT(reaction.actor_user_id) FILTER (WHERE reaction.reaction_type = 'celebrate') AS celebrate_count,
  COALESCE(MAX(reaction.reaction_type) FILTER (WHERE reaction.actor_user_id = ?), '') AS viewer_reaction`, viewer).
		Joins("LEFT JOIN recorded_activity_models activity ON activity.id = event.source_activity_id AND activity.participant_id = event.participant_user_id AND activity.path_id = event.path_id").
		Joins("LEFT JOIN social_goal_achievement_models achievement ON achievement.id = event.achievement_id AND achievement.participant_user_id = event.participant_user_id AND achievement.path_id = event.path_id").
		Joins("JOIN user_models participant ON participant.id = event.participant_user_id AND participant.status = 'active' AND participant.username IS NOT NULL").
		Joins("JOIN path_models path ON path.id = event.path_id").
		Joins("LEFT JOIN social_interaction_setting_models interaction_settings ON interaction_settings.user_id = event.participant_user_id").
		Joins(`LEFT JOIN social_practice_reaction_models reaction
  ON reaction.social_feed_event_id = event.id
 AND COALESCE(interaction_settings.reactions_enabled, true)
 AND NOT EXISTS (SELECT 1 FROM block_models reaction_block
   WHERE (reaction_block.blocker_user_id = ? AND reaction_block.blocked_user_id = reaction.actor_user_id)
      OR (reaction_block.blocker_user_id = reaction.actor_user_id AND reaction_block.blocked_user_id = ?))`, viewer, viewer).
		Where("event.published_at <= ?", page.Snapshot).
		Where("(event.source_activity_id IS NOT NULL AND activity.id IS NOT NULL) OR (event.achievement_id IS NOT NULL AND achievement.id IS NOT NULL)").
		Where(`path.owner_user_id = event.participant_user_id OR EXISTS (
  SELECT 1 FROM path_membership_models current_source_membership
  WHERE current_source_membership.path_id = path.id
    AND current_source_membership.user_id = event.participant_user_id
    AND current_source_membership.role IN ('administrator', 'participant')
)`).
		Where("EXISTS (SELECT 1 FROM user_models feed_viewer WHERE feed_viewer.id = ? AND feed_viewer.status = 'active')", viewer).
		Where(`NOT EXISTS (SELECT 1 FROM block_models feed_block
  WHERE (feed_block.blocker_user_id = ? AND feed_block.blocked_user_id = event.participant_user_id)
     OR (feed_block.blocker_user_id = event.participant_user_id AND feed_block.blocked_user_id = ?))`, viewer, viewer).
		Where(`path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models visibility_member
  WHERE visibility_member.path_id = path.id AND visibility_member.user_id = ?) OR path.visibility = 'public' OR (
  path.visibility = 'followers' AND EXISTS (SELECT 1 FROM follow_models visibility_follow
    WHERE visibility_follow.follower_user_id = ? AND visibility_follow.following_user_id = path.owner_user_id)
)`, viewer, viewer, viewer).
		Where(`(
  EXISTS (SELECT 1 FROM follow_models feed_follow
    WHERE feed_follow.follower_user_id = ? AND feed_follow.following_user_id = event.participant_user_id)
  OR (
    (path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models viewer_membership
      WHERE viewer_membership.path_id = path.id AND viewer_membership.user_id = ?
        AND viewer_membership.role IN ('administrator', 'participant')))
    AND
    (path.owner_user_id = event.participant_user_id OR EXISTS (SELECT 1 FROM path_membership_models source_membership
      WHERE source_membership.path_id = path.id AND source_membership.user_id = event.participant_user_id
        AND source_membership.role IN ('administrator', 'participant')))
  )
)`, viewer, viewer, viewer)
	if eventID != "" {
		query = query.Where("event.id = ?", eventID)
	} else {
		query = query.Where("event.participant_user_id <> ?", viewer)
	}
	if eventID == "" && page.AfterID != "" {
		query = query.Where("event.published_at < ? OR (event.published_at = ? AND event.id < ?)", page.AfterPublished, page.AfterPublished, page.AfterID)
	}
	query = query.Group("event.id, event.published_at, event.source_activity_id, event.achievement_id, event.participant_user_id, event.path_id, participant.username, participant.display_name, participant.profile_picture_url, path.name, activity.id, activity.started_at, activity.ended_at, achievement.kind, achievement.target_seconds, achievement.interval_started_at, achievement.interval_ended_at, interaction_settings.comments_enabled, interaction_settings.reactions_enabled")
	result := query.Order("event.published_at DESC, event.id DESC").Limit(page.Limit + 1).Scan(&rows)
	if result.Error != nil {
		return socialapp.FeedCandidatePage{}, fmt.Errorf("list practice feed candidates: %w: %v", ports.ErrUnavailable, result.Error)
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	items := make([]socialapp.PracticeFeedItem, 0, len(rows))
	for _, row := range rows {
		item := socialapp.PracticeFeedItem{
			ID: row.ID, PublishedAt: row.PublishedAt.UTC(), ParticipantID: row.ParticipantID,
			Username: row.Username, DisplayName: row.DisplayName, ProfilePictureURL: row.ProfilePictureURL,
			PathID: row.PathID, PathName: row.PathName, ActivityID: row.ActivityID,
			DurationSeconds: row.DurationSeconds, Edited: row.Edited,
			CommentsEnabled: row.CommentsEnabled, ReactionsEnabled: row.ReactionsEnabled,
			Reactions: socialdomain.ReactionSummary{Counts: socialdomain.ReactionCounts{
				Heart: row.HeartCount, Applause: row.ApplauseCount, Fire: row.FireCount,
				Strong: row.StrongCount, Celebrate: row.CelebrateCount,
			}, ViewerReaction: socialdomain.Reaction(row.ViewerReaction)},
		}
		switch socialapp.FeedEventType(row.EventType) {
		case socialapp.FeedEventPracticeSession:
			item.Type = socialapp.FeedEventPracticeSession
		case socialapp.FeedEventGoalAchievement:
			if row.AchievementTargetSeconds == nil {
				return socialapp.FeedCandidatePage{}, fmt.Errorf("list practice feed candidates: %w: incomplete achievement projection", ports.ErrUnavailable)
			}
			item.Type = socialapp.FeedEventGoalAchievement
			item.Achievement = &socialapp.GoalAchievement{
				Kind: socialapp.AchievementKind(row.AchievementKind), TargetSeconds: *row.AchievementTargetSeconds,
			}
			if row.IntervalStartedAt != nil {
				item.Achievement.IntervalStartedAt = row.IntervalStartedAt.UTC()
			}
			if row.IntervalEndedAt != nil {
				item.Achievement.IntervalEndedAt = row.IntervalEndedAt.UTC()
			}
		default:
			return socialapp.FeedCandidatePage{}, fmt.Errorf("list practice feed candidates: %w: invalid event type", ports.ErrUnavailable)
		}
		items = append(items, item)
	}
	return socialapp.FeedCandidatePage{Items: items, HasMore: hasMore}, nil
}

func (repository *SocialFeedRepository) ListActiveTimerCandidates(ctx context.Context, viewer string, page socialapp.ActiveFollowingPageRequest) (socialapp.ActiveFollowingCandidatePage, error) {
	if repository == nil || repository.db == nil || strings.TrimSpace(viewer) == "" || page.Limit < 1 || page.Limit > 100 {
		return socialapp.ActiveFollowingCandidatePage{}, ports.ErrInvalidArgument
	}
	type activeTimerRow struct {
		ParticipantID, Username, DisplayName, ProfilePictureURL string
		TimerID, PathID, PathName                               string
		StartedAt                                               time.Time
	}
	var rows []activeTimerRow
	result := repository.db.WithContext(ctx).Raw(`
WITH selected_participants AS (
  SELECT followed.id
  FROM follow_models active_follow
  JOIN user_models followed
    ON followed.id = active_follow.following_user_id
   AND followed.status = 'active'
   AND followed.username IS NOT NULL
  JOIN running_timer_models candidate_timer
    ON candidate_timer.participant_id = followed.id
  JOIN path_models candidate_path
    ON candidate_path.id = candidate_timer.path_id
   AND candidate_path.archived_at IS NULL
  WHERE active_follow.follower_user_id = ?
    AND followed.id > ?
    AND EXISTS (
      SELECT 1 FROM user_models active_viewer
      WHERE active_viewer.id = ? AND active_viewer.status = 'active'
    )
    AND NOT EXISTS (
      SELECT 1 FROM block_models active_block
      WHERE (active_block.blocker_user_id = ? AND active_block.blocked_user_id = followed.id)
         OR (active_block.blocker_user_id = followed.id AND active_block.blocked_user_id = ?)
    )
    AND (candidate_path.owner_user_id = ? OR EXISTS (
      SELECT 1 FROM path_membership_models visibility_member
      WHERE visibility_member.path_id = candidate_path.id AND visibility_member.user_id = ?
    ) OR candidate_path.visibility = 'public' OR (candidate_path.visibility = 'followers' AND EXISTS (
      SELECT 1 FROM follow_models visibility_follow
      WHERE visibility_follow.follower_user_id = ? AND visibility_follow.following_user_id = candidate_path.owner_user_id
    )))
  GROUP BY followed.id
  ORDER BY followed.id ASC
  LIMIT ?
)
SELECT participant.id AS participant_id,
       participant.username,
       participant.display_name,
       COALESCE(participant.profile_picture_url, '') AS profile_picture_url,
       timer.id AS timer_id,
       timer.path_id,
       path.name AS path_name,
       timer.started_at
FROM selected_participants selected
JOIN user_models participant ON participant.id = selected.id
JOIN running_timer_models timer ON timer.participant_id = participant.id
JOIN path_models path ON path.id = timer.path_id AND path.archived_at IS NULL
WHERE path.owner_user_id = ? OR EXISTS (
  SELECT 1 FROM path_membership_models visibility_member
  WHERE visibility_member.path_id = path.id AND visibility_member.user_id = ?
) OR path.visibility = 'public' OR (path.visibility = 'followers' AND EXISTS (
  SELECT 1 FROM follow_models visibility_follow
  WHERE visibility_follow.follower_user_id = ? AND visibility_follow.following_user_id = path.owner_user_id
))
ORDER BY participant.id ASC, timer.started_at ASC, timer.id ASC
`, viewer, page.AfterParticipantID, viewer, viewer, viewer, viewer, viewer, viewer, page.Limit+1, viewer, viewer, viewer).Scan(&rows)
	if result.Error != nil {
		return socialapp.ActiveFollowingCandidatePage{}, fmt.Errorf("list active following candidates: %w: %v", ports.ErrUnavailable, result.Error)
	}

	items := make([]socialapp.ActiveFollowingCandidate, 0, page.Limit)
	participantIndex := make(map[string]int, page.Limit)
	hasMore := false
	for _, row := range rows {
		index, exists := participantIndex[row.ParticipantID]
		if !exists {
			if len(items) == page.Limit {
				hasMore = true
				continue
			}
			index = len(items)
			participantIndex[row.ParticipantID] = index
			items = append(items, socialapp.ActiveFollowingCandidate{
				ParticipantID: row.ParticipantID, Username: row.Username, DisplayName: row.DisplayName,
				ProfilePictureURL: row.ProfilePictureURL, Timers: make([]socialapp.ActiveFollowingTimer, 0, 1),
			})
		}
		if index >= len(items) {
			continue
		}
		items[index].Timers = append(items[index].Timers, socialapp.ActiveFollowingTimer{
			ID: row.TimerID, PathID: row.PathID, PathName: row.PathName, StartedAt: row.StartedAt.UTC(),
		})
	}
	return socialapp.ActiveFollowingCandidatePage{Items: items, HasMore: hasMore}, nil
}
