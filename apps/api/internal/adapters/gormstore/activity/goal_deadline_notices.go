package activitystore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	channelstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationchannel"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) ListGoalDeadlineCandidates(ctx context.Context, at time.Time, after application.GoalDeadlineCandidate, limit int) (application.GoalDeadlinePage, error) {
	if r == nil || r.DB == nil || at.IsZero() || limit < 1 || limit > 100 || (after.ParticipantID == "") != (after.PathID == "") {
		return application.GoalDeadlinePage{}, ports.ErrInvalidArgument
	}
	var rows []application.GoalDeadlineCandidate
	query := r.DB.WithContext(ctx).Table("path_membership_models member").Select("member.user_id AS participant_id, member.path_id").
		Joins("JOIN user_models participant ON participant.id = member.user_id AND participant.status = 'active'").
		Joins("JOIN path_models path ON path.id = member.path_id AND path.archived_at IS NULL AND path.interval_goal_target_seconds IS NOT NULL").
		Where("member.role IN ('participant','administrator')").
		Where("NOT EXISTS (SELECT 1 FROM running_timer_models timer WHERE timer.path_id = member.path_id AND timer.participant_id = member.user_id)").
		Where("NOT EXISTS (SELECT 1 FROM goal_reminder_preference_models preference WHERE preference.path_id = member.path_id AND preference.participant_id = member.user_id AND NOT preference.enabled)").
		Where("NOT EXISTS (SELECT 1 FROM notification_channel_preference_models preference WHERE preference.user_id = member.user_id AND preference.channel = 'goal_reminders' AND NOT preference.enabled)").
		Where("NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id = member.user_id AND b.blocked_user_id = path.owner_user_id) OR (b.blocked_user_id = member.user_id AND b.blocker_user_id = path.owner_user_id))")
	if after.ParticipantID != "" {
		query = query.Where("member.user_id > ? OR (member.user_id = ? AND member.path_id > ?)", after.ParticipantID, after.ParticipantID, after.PathID)
	}
	if err := query.Order("member.user_id, member.path_id").Limit(limit).Find(&rows).Error; err != nil {
		return application.GoalDeadlinePage{}, err
	}
	page := application.GoalDeadlinePage{Candidates: make([]application.GoalDeadlineCandidate, 0, len(rows))}
	for _, c := range rows {
		_, due, err := goalDeadlineDue(r.DB.WithContext(ctx), c, at)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return application.GoalDeadlinePage{}, err
		}
		if due {
			page.Candidates = append(page.Candidates, c)
		}
	}
	if len(rows) == limit {
		page.Next = rows[len(rows)-1]
	}
	return page, nil
}

func goalDeadlineDue(tx *gorm.DB, c application.GoalDeadlineCandidate, at time.Time) (application.IntervalWindow, bool, error) {
	goals, err := currentAchievementGoals(tx, c.PathID)
	if err != nil {
		return application.IntervalWindow{}, false, err
	}
	goal, err := intervalGoalFromRow(goals.IntervalGoal)
	if err != nil || !goal.Present {
		return application.IntervalWindow{}, false, err
	}
	zone, err := currentParticipantTimeZone(tx, c.ParticipantID)
	if err != nil {
		return application.IntervalWindow{}, false, err
	}
	window, err := application.CurrentIntervalWindow(goal, zone, at)
	if err != nil {
		return window, false, err
	}
	completed, err := intervalSeconds(tx, c.ParticipantID, c.PathID, window)
	if err != nil {
		return window, false, err
	}
	var running int64
	if err := tx.Model(&timerModel{}).Where("participant_id = ? AND path_id = ?", c.ParticipantID, c.PathID).Count(&running).Error; err != nil {
		return window, false, err
	}
	due, err := (domain.GoalDeadlineState{IntervalStartedAt: window.StartedAt, IntervalEndedAt: window.EndedAt, TargetSeconds: goal.TargetSeconds, RecordedSeconds: completed, Running: running > 0}).NoLongerAchievable(at)
	if err != nil || !due {
		return window, false, err
	}
	var delivered int64
	err = tx.Table("goal_deadline_notice_receipt_models").Where("participant_id = ? AND path_id = ? AND interval_started_at = ? AND interval_ended_at = ?", c.ParticipantID, c.PathID, window.StartedAt, window.EndedAt).Count(&delivered).Error
	return window, delivered == 0, err
}

func (r *Repository) PublishGoalDeadlineNotice(ctx context.Context, c application.GoalDeadlineNoticeCommand) (bool, error) {
	candidate := c.Candidate
	if r == nil || r.DB == nil || !validReminderID(candidate.ParticipantID) || !validReminderID(candidate.PathID) || c.At.IsZero() || !c.Audit.Valid() || c.Audit.ActorUserID != candidate.ParticipantID || c.Audit.OwnerUserID != candidate.ParticipantID || c.Audit.TargetID != candidate.PathID || c.Audit.TargetType != "goal_deadline_notice" || c.Audit.Action != audit.ResourceCreated || c.Audit.Outcome != audit.Succeeded || !c.Audit.OccurredAt.Equal(c.At) {
		return false, ports.ErrInvalidArgument
	}
	published := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActivePath(tx, candidate.PathID, candidate.ParticipantID); err != nil {
			if errors.Is(err, ports.ErrNotFound) || errors.Is(err, ports.ErrConflict) {
				return nil
			}
			return err
		}
		preference, err := New(tx).GetGoalReminderPreference(ctx, candidate.ParticipantID, candidate.PathID)
		if errors.Is(err, ports.ErrNotFound) {
			return nil
		}
		if err != nil || !preference.Enabled {
			return err
		}
		allowed, err := channelstore.Allowed(tx, candidate.ParticipantID, "goal_reminders")
		if err != nil || !allowed {
			return err
		}
		window, due, err := goalDeadlineDue(tx, candidate, c.At)
		if err != nil || !due {
			return err
		}
		created := tx.Table("goal_deadline_notice_receipt_models").Clauses(clause.OnConflict{DoNothing: true}).Create(map[string]any{"participant_id": candidate.ParticipantID, "path_id": candidate.PathID, "interval_started_at": window.StartedAt, "interval_ended_at": window.EndedAt, "created_at": c.At})
		if created.Error != nil || created.RowsAffected == 0 {
			return created.Error
		}
		id := goalDeadlineNoticeID(candidate, window)
		if err := tx.Table("notification_models").Create(map[string]any{"id": id, "recipient_user_id": candidate.ParticipantID, "actor_user_id": candidate.ParticipantID, "path_id": candidate.PathID, "goal_interval_started_at": window.StartedAt, "goal_interval_ended_at": window.EndedAt, "kind": "goal_no_longer_achievable", "presentation_class": "informational", "channel": "goal_reminders", "created_at": c.At}).Error; err != nil {
			return err
		}
		if err := channelstore.QueuePush(tx, id, candidate.ParticipantID, c.At); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(c.Audit)).Error; err != nil {
			return err
		}
		published = true
		return nil
	})
	return published && err == nil, err
}
func goalDeadlineNoticeID(c application.GoalDeadlineCandidate, w application.IntervalWindow) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{c.ParticipantID, c.PathID, w.StartedAt.UTC().Format(time.RFC3339Nano), w.EndedAt.UTC().Format(time.RFC3339Nano)}, "\x00")))
	return "goal-deadline:" + hex.EncodeToString(digest[:])
}
