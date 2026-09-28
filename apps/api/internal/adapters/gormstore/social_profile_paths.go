package gormstore

import (
	"context"
	"database/sql"
	"fmt"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

func (repository *SocialFeedRepository) ListProfilePathCandidates(ctx context.Context, viewer, participant string, at time.Time) ([]socialapp.ProfilePathCandidate, error) {
	if repository == nil || repository.db == nil || viewer == "" || participant == "" || at.IsZero() {
		return nil, ports.ErrInvalidArgument
	}
	items := []socialapp.ProfilePathCandidate{}
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			ID, Name  string
			TimerID   *string
			StartedAt *time.Time
		}
		err := tx.Table("path_models AS path").Select("path.id, path.name, timer.id AS timer_id, timer.started_at").
			Joins("LEFT JOIN running_timer_models timer ON timer.path_id = path.id AND timer.participant_id = ?", participant).
			Where("path.archived_at IS NULL").
			Where(`path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models member WHERE member.path_id = path.id AND member.user_id = ? AND member.role IN ('administrator','participant'))`, participant, participant).
			Where(`EXISTS (SELECT 1 FROM user_models u WHERE u.id = ? AND u.status = 'active') AND EXISTS (SELECT 1 FROM user_models u WHERE u.id = ? AND u.status = 'active')`, viewer, participant).
			Where(`NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id = ? AND b.blocked_user_id = ?) OR (b.blocker_user_id = ? AND b.blocked_user_id = ?))`, viewer, participant, participant, viewer).
			Where(`path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models m WHERE m.path_id = path.id AND m.user_id = ?) OR path.visibility = 'public' OR (path.visibility = 'followers' AND EXISTS (SELECT 1 FROM follow_models f WHERE f.follower_user_id = ? AND f.following_user_id = path.owner_user_id))`, viewer, viewer, viewer).
			Order("timer.started_at ASC NULLS LAST, timer.id ASC, path.id ASC").Scan(&rows).Error
		if err != nil {
			return fmt.Errorf("profile paths: %w", ports.ErrUnavailable)
		}
		for _, row := range rows {
			item := socialapp.ProfilePathCandidate{ID: row.ID}
			if row.TimerID != nil && row.StartedAt != nil {
				progress, err := (&SocialFeedRepository{db: tx}).activePathProgress(ctx, participant, row.ID, at)
				if err != nil {
					return err
				}
				item.Timer = &socialapp.ActiveFollowingTimer{ID: *row.TimerID, PathID: row.ID, PathName: row.Name, StartedAt: row.StartedAt.UTC(), Progress: progress}
			}
			items = append(items, item)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return items, err
}
