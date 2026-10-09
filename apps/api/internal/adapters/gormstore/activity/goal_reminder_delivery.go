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
)

type reminderProjection struct {
	Plan   domain.GoalReminderPlan
	Window application.IntervalWindow
}

func reminderPlan(tx *gorm.DB, owner, path string, at time.Time) (reminderProjection, bool, error) {
	preference, err := New(tx).GetGoalReminderPreference(tx.Statement.Context, owner, path)
	if errors.Is(err, ports.ErrNotFound) {
		return reminderProjection{}, false, nil
	}
	if err != nil || !preference.Enabled {
		return reminderProjection{}, false, err
	}
	allowed, err := channelstore.Allowed(tx, owner, "goal_reminders")
	if err != nil || !allowed {
		return reminderProjection{}, false, err
	}
	goals, err := currentAchievementGoals(tx, path)
	if err != nil {
		return reminderProjection{}, false, err
	}
	goal, err := intervalGoalFromRow(goals.IntervalGoal)
	if err != nil || !goal.Present {
		return reminderProjection{}, false, err
	}
	zone, err := currentParticipantTimeZone(tx, owner)
	if err != nil {
		return reminderProjection{}, false, err
	}
	window, err := application.CurrentIntervalWindow(goal, zone, at)
	if err != nil {
		return reminderProjection{}, false, err
	}
	recorded, err := intervalSeconds(tx, owner, path, window)
	if err != nil {
		return reminderProjection{}, false, err
	}
	var running int64
	if err := tx.Model(&timerModel{}).Where("participant_id = ? AND path_id = ?", owner, path).Count(&running).Error; err != nil {
		return reminderProjection{}, false, err
	}
	state := domain.GoalReminderState{IntervalStartedAt: window.StartedAt, IntervalEndedAt: window.EndedAt, TargetSeconds: goal.TargetSeconds, RecordedSeconds: recorded, Running: running > 0}
	scheduled, eligible, err := state.Schedule()
	if err != nil || !eligible {
		return reminderProjection{}, false, err
	}
	// A delayed scan must never suggest a goal that can no longer be completed.
	if at.After(scheduled.Add(30 * time.Minute)) {
		return reminderProjection{}, false, nil
	}
	var delivered int64
	if err := tx.Table("goal_reminder_receipt_models").Where("participant_id = ? AND path_id = ? AND interval_started_at = ? AND interval_ended_at = ?", owner, path, window.StartedAt, window.EndedAt).Count(&delivered).Error; err != nil {
		return reminderProjection{}, false, err
	}
	return reminderProjection{Plan: domain.GoalReminderPlan{PathID: path, ScheduledAt: scheduled}, Window: window}, delivered == 0, nil
}

func (r *Repository) ListGoalReminderCandidates(ctx context.Context, at time.Time, after string, limit int) (application.GoalReminderCandidatePage, error) {
	if r == nil || r.DB == nil || at.IsZero() || limit < 1 || limit > 100 || (after != "" && !validReminderID(after)) {
		return application.GoalReminderCandidatePage{}, ports.ErrInvalidArgument
	}
	var owners []string
	err := r.DB.WithContext(ctx).Table("path_membership_models member").Distinct("member.user_id").
		Joins("JOIN user_models participant ON participant.id = member.user_id AND participant.status = 'active'").
		Joins("JOIN path_models path ON path.id = member.path_id AND path.archived_at IS NULL AND path.interval_goal_target_seconds IS NOT NULL").
		Where("member.role IN ('participant','administrator') AND member.user_id > ?", after).Order("member.user_id").Limit(limit).Pluck("member.user_id", &owners).Error
	if err != nil {
		return application.GoalReminderCandidatePage{}, err
	}
	page := application.GoalReminderCandidatePage{}
	for _, owner := range owners {
		candidate, err := r.GoalReminderCandidateForParticipant(ctx, at, owner)
		if err != nil {
			return page, err
		}

		if len(candidate.PathIDs) > 0 {
			page.Candidates = append(page.Candidates, candidate)
		}
	}
	if len(owners) == limit {
		page.NextParticipantID = owners[len(owners)-1]
	}
	return page, nil
}

func (r *Repository) PublishGoalReminders(ctx context.Context, c application.GoalReminderDeliveryCommand) (int, error) {
	if r == nil || r.DB == nil || !validReminderID(c.ParticipantID) || len(c.AuthorizedPathIDs) == 0 || c.At.IsZero() || !c.Audit.Valid() || c.Audit.OwnerUserID != c.ParticipantID || c.Audit.ActorUserID != c.ParticipantID || c.Audit.TargetID != c.ParticipantID || c.Audit.TargetType != "goal_reminder_delivery" || c.Audit.Action != audit.ResourceCreated || c.Audit.Outcome != audit.Succeeded || !c.Audit.OccurredAt.Equal(c.At) {
		return 0, ports.ErrInvalidArgument
	}
	previous := ""
	for _, id := range c.AuthorizedPathIDs {
		if !validReminderID(id) || id <= previous {
			return 0, ports.ErrInvalidArgument
		}
		previous = id
	}
	published := 0
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		projections := make(map[string]reminderProjection, len(c.AuthorizedPathIDs))
		plans := make([]domain.GoalReminderPlan, 0, len(c.AuthorizedPathIDs))
		for _, path := range c.AuthorizedPathIDs {
			if err := lockActivePath(tx, path, c.ParticipantID); err != nil {
				if errors.Is(err, ports.ErrNotFound) || errors.Is(err, ports.ErrConflict) {
					continue
				}
				return err
			}
			p, eligible, err := reminderPlan(tx, c.ParticipantID, path, c.At)
			if err != nil {
				return err
			}
			if eligible {
				projections[path] = p
				plans = append(plans, p.Plan)
			}
		}
		bundles, err := domain.BundleGoalReminders(plans)
		if err != nil {
			return err
		}
		for _, bundle := range bundles {
			if bundle.ScheduledAt.After(c.At) {
				continue
			}
			parts := []string{c.ParticipantID}
			for _, p := range bundle.Paths {
				w := projections[p.PathID].Window
				parts = append(parts, p.PathID, w.StartedAt.UTC().Format(time.RFC3339Nano), w.EndedAt.UTC().Format(time.RFC3339Nano))
			}
			digest := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
			id := "goal-reminder:" + hex.EncodeToString(digest[:])
			if err := tx.Table("notification_models").Create(map[string]any{"id": id, "recipient_user_id": c.ParticipantID, "actor_user_id": c.ParticipantID, "kind": "goal_practice_reminder", "presentation_class": "informational", "channel": "goal_reminders", "created_at": c.At}).Error; err != nil {
				return err
			}
			for _, p := range bundle.Paths {
				w := projections[p.PathID].Window
				if err := tx.Table("goal_reminder_receipt_models").Create(map[string]any{"participant_id": c.ParticipantID, "path_id": p.PathID, "interval_started_at": w.StartedAt, "interval_ended_at": w.EndedAt, "notification_id": id, "scheduled_at": p.ScheduledAt, "created_at": c.At}).Error; err != nil {
					return err
				}
			}
			if err := channelstore.QueuePush(tx, id, c.ParticipantID, c.At); err != nil {
				return err
			}
			published++
		}
		if published > 0 {
			return tx.Create(fromAudit(c.Audit)).Error
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return published, nil
}

var _ application.GoalReminderRepository = (*Repository)(nil)

func (r *Repository) GoalReminderCandidateForParticipant(ctx context.Context, at time.Time, owner string) (application.GoalReminderCandidate, error) {
	if r == nil || r.DB == nil || at.IsZero() || !validReminderID(owner) {
		return application.GoalReminderCandidate{}, ports.ErrInvalidArgument
	}
	var paths []string
	if err := r.DB.WithContext(ctx).Table("path_membership_models").Where("user_id = ? AND role IN ('participant','administrator')", owner).Order("path_id").Pluck("path_id", &paths).Error; err != nil {
		return application.GoalReminderCandidate{}, err
	}
	plans := make([]domain.GoalReminderPlan, 0, len(paths))
	for _, path := range paths {
		p, eligible, err := reminderPlan(r.DB.WithContext(ctx), owner, path, at)
		if err != nil {
			return application.GoalReminderCandidate{}, err
		}
		if eligible {
			plans = append(plans, p.Plan)
		}
	}
	bundles, err := domain.BundleGoalReminders(plans)
	if err != nil {
		return application.GoalReminderCandidate{}, err
	}
	due := make(map[string]bool)
	for _, bundle := range bundles {
		if !bundle.ScheduledAt.After(at) {
			for _, p := range bundle.Paths {
				due[p.PathID] = true
			}
		}
	}
	candidate := application.GoalReminderCandidate{ParticipantID: owner}
	for _, path := range paths {
		if due[path] {
			candidate.PathIDs = append(candidate.PathIDs, path)
		}
	}
	return candidate, nil
}
