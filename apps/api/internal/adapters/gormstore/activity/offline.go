package activitystore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	timerstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationtimer"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type offlineTimerState struct {
	ParticipantID    string `gorm:"primaryKey"`
	ClientTimerID    string `gorm:"primaryKey"`
	PathID           string
	IdentityHash     []byte
	CanonicalTimerID string
	Outcome          string
	ActivityID       *string
	TerminalAt       *time.Time
	SavedSeconds     int64
}

func (offlineTimerState) TableName() string { return "offline_timer_state_models" }

type offlineTimerReplay struct {
	ParticipantID    string `gorm:"primaryKey"`
	Key              string `gorm:"primaryKey"`
	PathID           string
	ClientTimerID    string
	RequestHash      []byte
	Outcome          string
	SavedSeconds     int64
	DiscardedSeconds int64
}

func (offlineTimerReplay) TableName() string { return "offline_timer_replay_models" }

func (r *Repository) SynchronizeTimer(ctx context.Context, command application.OfflineTimerCommand) (application.OfflineTimerResult, error) {
	action := audit.ActivityTimerStarted
	if command.Kind != "start" {
		action = audit.ActivityTimerStopped
	}
	occurrence := command.Timer
	if command.CorrectedStartedAt != nil {
		occurrence.StartedAt = *command.CorrectedStartedAt
	}
	if r == nil || r.DB == nil || !validTimer(command.Timer) || len(command.IdentityHash) != 32 || !validIdempotency(command.Idempotency, command.Timer.ParticipantID, application.OfflineTimerOperation) || !validTimerAudit(command.Audit, action, command.Timer) || command.RecordedAt.IsZero() || !validTimer(occurrence) || occurrence.StartedAt.After(command.RecordedAt) || (command.Kind != "correct" && command.Timer.StartedAt.After(command.RecordedAt)) || (command.Kind != "start" && command.Kind != "stop" && command.Kind != "correct") || (command.Kind == "correct") != (command.CorrectedStartedAt != nil) || (command.Kind == "start" && command.EndedAt != nil) || (command.Kind != "start" && (command.EndedAt == nil || !command.EndedAt.After(occurrence.StartedAt) || command.EndedAt.After(command.RecordedAt))) {
		return application.OfflineTimerResult{}, ports.ErrInvalidArgument
	}
	var result application.OfflineTimerResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := timerstore.Lock(tx, command.Timer.PathID); err != nil {
			return err
		}

		// Serialize this account's offline identities across Paths before the normal
		// progress/Path locks. Reusing a client identity on another Path cannot race.
		if err := progresslock.LockKey(tx, "offline-timers:"+command.Timer.ParticipantID); err != nil {
			return err
		}
		joined, archived, err := lockPathMembership(tx, command.Timer.PathID, command.Timer.ParticipantID)
		if err != nil {
			return err
		}
		if occurrence.StartedAt.Before(joined) {
			return ports.ErrNotFound
		}
		var replay offlineTimerReplay
		found := tx.Where("participant_id = ? AND key = ?", command.Timer.ParticipantID, command.Idempotency.Key).Take(&replay).Error
		if found != nil && !errors.Is(found, gorm.ErrRecordNotFound) {
			return found
		}
		if found == nil && (replay.PathID != command.Timer.PathID || replay.ClientTimerID != command.Timer.ID || !sameHash(replay.RequestHash, command.Idempotency.RequestHash)) {
			return ports.ErrIdempotencyConflict
		}
		var adopted timerModel
		adoption := tx.Where("participant_id = ? AND path_id = ? AND id = ?", command.Timer.ParticipantID, command.Timer.PathID, command.Timer.ID).Take(&adopted).Error
		if adoption != nil && !errors.Is(adoption, gorm.ErrRecordNotFound) {
			return adoption
		}
		if adoption == nil {
			if err := registerOnlineTimer(tx, adopted); err != nil {
				return err
			}
		}
		state, exists, err := loadOfflineState(tx, command)
		if err != nil {
			return err
		}
		if !exists {
			state, exists, err = adoptLegacyTerminalTimer(tx, command)
			if err != nil {
				return err
			}
		}
		if found == nil {
			result, err = offlineCurrentResult(tx, state, replay.Outcome, replay.DiscardedSeconds)
			result.Replayed = true
			return err
		}
		if !exists {
			digest := sha256.Sum256([]byte(command.Timer.ParticipantID + "\x00" + command.Timer.PathID + "\x00" + command.Timer.ID))
			state = offlineTimerState{ParticipantID: command.Timer.ParticipantID, ClientTimerID: command.Timer.ID, PathID: command.Timer.PathID, IdentityHash: command.IdentityHash, CanonicalTimerID: "offline:" + hex.EncodeToString(digest[:]), Outcome: "active"}
			if err := tx.Create(&state).Error; err != nil {
				return err
			}
		}
		outcome, discarded, err := applyOfflineTimer(tx, &state, command, archived)
		if err != nil {
			return err
		}
		// Only a newly admitted, still-running occurrence creates a start notice.
		// Previously synchronized identities never notify again on replay/stop.
		if command.Kind == "start" && command.NotifyStart && !exists && state.Outcome == "active" {
			var active timerModel
			if err := tx.Where("participant_id = ? AND path_id = ? AND id = ?", state.ParticipantID, state.PathID, state.CanonicalTimerID).Take(&active).Error; err != nil {
				return err
			}
			if err := timerstore.Create(tx, toTimer(active), command.NotificationRecipients, command.RecordedAt); err != nil {
				return err
			}
		}
		if err := tx.Model(&offlineTimerState{}).Where("participant_id = ? AND client_timer_id = ?", state.ParticipantID, state.ClientTimerID).Updates(map[string]any{"outcome": state.Outcome, "activity_id": state.ActivityID, "terminal_at": state.TerminalAt, "saved_seconds": state.SavedSeconds}).Error; err != nil {
			return err
		}
		replay = offlineTimerReplay{ParticipantID: state.ParticipantID, Key: command.Idempotency.Key, PathID: state.PathID, ClientTimerID: command.Timer.ID, RequestHash: command.Idempotency.RequestHash, Outcome: outcome, SavedSeconds: state.SavedSeconds, DiscardedSeconds: discarded}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result, err = offlineCurrentResult(tx, state, outcome, discarded)
		return err
	})
	return result, err
}

func loadOfflineState(tx *gorm.DB, command application.OfflineTimerCommand) (offlineTimerState, bool, error) {
	var state offlineTimerState
	err := tx.Where("participant_id = ? AND (client_timer_id = ? OR canonical_timer_id = ?)", command.Timer.ParticipantID, command.Timer.ID, command.Timer.ID).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return state, false, nil
	}
	if err != nil {
		return state, false, err
	}
	if state.PathID != command.Timer.PathID || !sameHash(state.IdentityHash, command.IdentityHash) {
		return state, false, ports.ErrIdempotencyConflict
	}
	return state, true, nil
}

func applyOfflineTimer(tx *gorm.DB, state *offlineTimerState, command application.OfflineTimerCommand, archived *time.Time) (string, int64, error) {
	if state.Outcome == "archived" {
		var discarded int64
		if command.EndedAt != nil {
			discarded = int64(command.EndedAt.Sub(offlineOccurrence(command).StartedAt) / time.Second)
		}
		return "archived", discarded, nil
	}
	// The first authoritative archival decision belongs to this session. A later
	// Path restoration permits new sessions, not time beyond the old cutoff.
	if state.Outcome == "archive_waiting" {
		if state.TerminalAt == nil {
			return "", 0, ports.ErrConflict
		}
		archived = state.TerminalAt
	}
	if state.Outcome == "conflict" {
		return "conflict", 0, nil
	}
	if state.Outcome == "stopped" {
		return "accepted", 0, nil
	}
	if state.Outcome == "archive_pending" {
		if command.Kind == "start" {
			return "accepted", 0, nil
		}
		return reconcileArchivedOfflineTimer(tx, state, command)
	}
	timer := offlineOccurrence(command)
	timer.ID = state.CanonicalTimerID
	timer.StartedAt = postgresInstant(timer.StartedAt)
	if archived != nil && !timer.StartedAt.Before(*archived) {
		state.Outcome = "archived"
		var discarded int64
		if command.EndedAt != nil {
			discarded = int64(command.EndedAt.Sub(timer.StartedAt) / time.Second)
		}
		return "archived", discarded, nil
	}
	if command.Kind == "start" {
		if archived != nil {
			state.Outcome = "archive_waiting"
			state.TerminalAt = archived
			return "accepted", 0, nil
		} // The client ends a still-running timer, retaining an already queued stop.
		var current timerModel
		err := tx.Where("participant_id = ? AND path_id = ?", timer.ParticipantID, timer.PathID).Take(&current).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", 0, err
		}
		if err == nil {
			if current.ID == timer.ID {
				return "accepted", 0, nil
			}
			winner, err := domain.OfflineTimerWinner(timer, toTimer(current))
			if err != nil {
				return "", 0, err
			}
			if winner.ID != timer.ID {
				state.Outcome = "conflict"
				return "conflict", 0, nil
			}
			// Preserve the losing identity even when it was started by an API
			// version predating the offline registry.
			if err := registerOnlineTimer(tx, current); err != nil {
				return "", 0, err
			}
			if err := tx.Model(&offlineTimerState{}).Where("participant_id = ? AND path_id = ? AND canonical_timer_id = ?", timer.ParticipantID, timer.PathID, current.ID).Update("outcome", "conflict").Error; err != nil {
				return "", 0, err
			}
			if err := tx.Where("participant_id = ? AND path_id = ? AND id = ?", timer.ParticipantID, timer.PathID, current.ID).Delete(&timerModel{}).Error; err != nil {
				return "", 0, err
			}
		}
		return "accepted", 0, tx.Create(fromTimer(timer)).Error
	}
	// A stop may arrive after a lost start acknowledgement. The durable identity
	// determines whether it was discarded; it never stops another device's timer.
	resolved, err := domain.ResolveOfflineStop(timer, command.ActivityID, postgresInstant(*command.EndedAt), postgresInstant(command.RecordedAt), archived)
	if err != nil {
		return "", 0, ports.ErrInvalidArgument
	}
	if resolved.Saved {
		if err := persistCompletedActivity(tx, resolved.Activity); err != nil {
			return "", 0, err
		}
		// The offline stop predates any edit authored before reconnect. Receipt
		// time remains audit metadata, never the timer entry's causal baseline.
		order, err := domain.NewActivityEditOrder(postgresInstant(*command.EndedAt), 0, command.Idempotency.Key)
		if err != nil {
			return "", 0, err
		}
		if err := saveEditOrder(tx, resolved.Activity.ID, order); err != nil {
			return "", 0, err
		}
		state.ActivityID = &resolved.Activity.ID
		state.SavedSeconds = resolved.Activity.DurationSeconds()
	}
	if err := tx.Where("participant_id = ? AND path_id = ? AND id = ?", timer.ParticipantID, timer.PathID, timer.ID).Delete(&timerModel{}).Error; err != nil {
		return "", 0, err
	}
	state.Outcome = "stopped"
	state.TerminalAt = command.EndedAt
	outcome := "accepted"
	if resolved.Archived {
		outcome = "archived"
	}
	return outcome, resolved.DiscardedSeconds, nil
}

func offlineCurrentResult(tx *gorm.DB, state offlineTimerState, outcome string, discarded int64) (application.OfflineTimerResult, error) {
	result := application.OfflineTimerResult{Outcome: outcome, DiscardedSeconds: discarded,
		Terminal: state.Outcome == "stopped" || state.Outcome == "conflict" || state.Outcome == "archived",
		MustStop: state.Outcome == "archive_pending" || state.Outcome == "archive_waiting"}
	if state.Outcome == "conflict" {
		result.Outcome = "conflict"
		return result, nil
	}
	if state.Outcome == "archived" {
		result.Outcome = "archived"
	}
	if state.ActivityID != nil {
		var row activityModel
		err := tx.Where("participant_id = ? AND path_id = ? AND id = ?", state.ParticipantID, state.PathID, *state.ActivityID).Take(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return result, err
		}
		if err == nil {
			entry := toActivity(row)
			result.Activity = &entry
			result.SavedSeconds = entry.DurationSeconds()
		}
	}
	if state.Outcome == "active" {
		var row timerModel
		err := tx.Where("participant_id = ? AND path_id = ? AND id = ?", state.ParticipantID, state.PathID, state.CanonicalTimerID).Take(&row).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return result, err
		}
		if err == nil {
			timer := toTimer(row)
			result.Timer = &timer
		}
	}
	return result, nil
}

// settleOfflineTimer joins ordinary online/lifecycle stops to the same durable
// terminal identity. A delayed offline stop must return this entry, not add one.
func settleOfflineTimer(tx *gorm.DB, timer timerModel, entry domain.RecordedActivity, saved bool, endedAt time.Time, outcome string) error {
	if err := registerOnlineTimer(tx, timer); err != nil {
		return err
	}
	var activityID *string
	var seconds int64
	if saved {
		activityID = &entry.ID
		seconds = entry.DurationSeconds()
	}
	return tx.Model(&offlineTimerState{}).Where("participant_id = ? AND path_id = ? AND canonical_timer_id = ? AND outcome = ?", timer.ParticipantID, timer.PathID, timer.ID, "active").Updates(map[string]any{"outcome": outcome, "activity_id": activityID, "terminal_at": postgresInstant(endedAt), "saved_seconds": seconds}).Error
}

func archiveOfflineTimer(tx *gorm.DB, timer timerModel, entry domain.RecordedActivity, saved bool, endedAt time.Time) (bool, error) {
	if err := registerOnlineTimer(tx, timer); err != nil {
		return false, err
	}
	var count int64
	if err := tx.Model(&offlineTimerState{}).Where("participant_id = ? AND path_id = ? AND canonical_timer_id = ? AND outcome = ?", timer.ParticipantID, timer.PathID, timer.ID, "active").Count(&count).Error; err != nil {
		return false, err
	}
	if count == 0 {
		return false, nil
	}
	if saved {
		if err := persistCompletedActivity(tx, entry); err != nil {
			return false, err
		}
	}
	return true, settleOfflineTimer(tx, timer, entry, saved, endedAt, "archive_pending")
}

// Archival may have stopped a server-known timer while its device already had
// an earlier offline stop queued. Correct the same entry, retaining its history
// and removing achievements no longer supported by the corrected interval.
func reconcileArchivedOfflineTimer(tx *gorm.DB, state *offlineTimerState, command application.OfflineTimerCommand) (string, int64, error) {
	timer := offlineOccurrence(command)
	timer.ID = state.CanonicalTimerID
	resolved, err := domain.ResolveOfflineStop(timer, command.ActivityID, postgresInstant(*command.EndedAt), postgresInstant(command.RecordedAt), state.TerminalAt)
	if err != nil {
		return "", 0, ports.ErrInvalidArgument
	}
	outcome := "accepted"
	if resolved.Archived {
		outcome = "archived"
	}
	state.Outcome = "stopped"
	if state.ActivityID == nil {
		// Null after a previously saved entry means deletion. Never recreate it.
		state.SavedSeconds = 0
		return outcome, resolved.DiscardedSeconds, nil
	}
	var row activityModel
	if err := tx.Where("participant_id = ? AND path_id = ? AND id = ?", state.ParticipantID, state.PathID, *state.ActivityID).Take(&row).Error; err != nil {
		return "", 0, err
	}
	if !resolved.Saved {
		if err := tx.Where("participant_id = ? AND path_id = ? AND id = ?", state.ParticipantID, state.PathID, row.ID).Delete(&activityModel{}).Error; err != nil {
			return "", 0, err
		}
		state.ActivityID = nil
		state.SavedSeconds = 0
	} else {
		state.SavedSeconds = resolved.Activity.DurationSeconds()
		if !row.EndedAt.Equal(resolved.Activity.EndedAt) || !row.StartedAt.Equal(resolved.Activity.StartedAt) {
			var revisions int64
			if err := tx.Model(&activityRevisionModel{}).Where("activity_id = ?", row.ID).Count(&revisions).Error; err != nil {
				return "", 0, err
			}
			prior := domain.ActivityRevision{Activity: toActivity(row), ReplacedAt: postgresInstant(command.RecordedAt)}
			revision := fromRevision(prior, revisions+1, true)
			if err := tx.Create(&revision).Error; err != nil {
				return "", 0, err
			}
			if err := tx.Model(&activityModel{}).Where("participant_id = ? AND path_id = ? AND id = ?", state.ParticipantID, state.PathID, row.ID).Updates(map[string]any{"started_at": resolved.Activity.StartedAt, "ended_at": resolved.Activity.EndedAt, "updated_at": postgresInstant(command.RecordedAt)}).Error; err != nil {
				return "", 0, err
			}
		}
	}
	if _, err := removeUnsupportedGoalAchievements(tx, state.ParticipantID, state.PathID); err != nil {
		return "", 0, err
	}
	return outcome, resolved.DiscardedSeconds, nil
}

// Existing rows are kept unchanged: a canonical alias must never reset a
// conflict, completed timer, or archive correction into an active timer.
func registerOnlineTimer(tx *gorm.DB, timer timerModel) error {
	var existing offlineTimerState
	err := tx.Where("participant_id = ? AND canonical_timer_id = ?", timer.ParticipantID, timer.ID).Take(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	state := offlineTimerState{ParticipantID: timer.ParticipantID, ClientTimerID: timer.ID, PathID: timer.PathID, IdentityHash: application.OfflineTimerIdentityHash(toTimer(timer)), CanonicalTimerID: timer.ID, Outcome: "active"}
	created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state)
	if created.Error != nil {
		return created.Error
	}
	if created.RowsAffected != 1 {
		return ports.ErrConflict
	}
	return nil
}

// An old API may have completed this timer before the offline registry existed.
// Its persisted start binds the original occurrence even when activity deletion
// has scrubbed the stop result. Missing active state is terminal, never a new
// offline start. All reads remain scoped to this participant and Path.
func adoptLegacyTerminalTimer(tx *gorm.DB, command application.OfflineTimerCommand) (offlineTimerState, bool, error) {
	var start mutationModel
	lookup := tx.Where("participant_id = ? AND path_id = ? AND timer_id = ? AND operation = ?", command.Timer.ParticipantID, command.Timer.PathID, command.Timer.ID, application.StartTimerOperation)
	err := lookup.Take(&start).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return offlineTimerState{}, false, nil
	}
	if err != nil {
		return offlineTimerState{}, false, err
	}
	if start.ResultStartedAt == nil || start.ResultTimeZone == "" {
		return offlineTimerState{}, false, ports.ErrConflict
	}
	original := domain.RunningTimer{ID: command.Timer.ID, ParticipantID: command.Timer.ParticipantID, PathID: command.Timer.PathID, StartedAt: start.ResultStartedAt.UTC(), OccurrenceTimeZone: start.ResultTimeZone}
	identity := application.OfflineTimerIdentityHash(original)
	if !sameHash(identity, command.IdentityHash) {
		return offlineTimerState{}, false, ports.ErrIdempotencyConflict
	}
	state := offlineTimerState{ParticipantID: original.ParticipantID, ClientTimerID: original.ID, PathID: original.PathID, IdentityHash: identity, CanonicalTimerID: original.ID, Outcome: "stopped"}
	var stop mutationModel
	err = tx.Where("participant_id = ? AND path_id = ? AND timer_id = ? AND operation = ?", original.ParticipantID, original.PathID, original.ID, application.StopTimerOperation).Take(&stop).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return state, false, err
	}
	if err == nil {
		state.TerminalAt = stop.ResultEndedAt
		if stop.ResultActivitySaved && !stop.ResultActivityDeleted && stop.ResultActivityID != nil {
			var entry activityModel
			err = tx.Where("participant_id = ? AND path_id = ? AND id = ?", original.ParticipantID, original.PathID, *stop.ResultActivityID).Take(&entry).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return state, false, err
			}
			if err == nil {
				state.ActivityID = &entry.ID
				state.SavedSeconds = toActivity(entry).DurationSeconds()
			}
		}
	}
	if err := tx.Create(&state).Error; err != nil {
		return state, false, err
	}
	return state, true, nil
}

func offlineOccurrence(command application.OfflineTimerCommand) domain.RunningTimer {
	timer := command.Timer
	if command.CorrectedStartedAt != nil {
		timer.StartedAt = *command.CorrectedStartedAt
	}
	return timer
}
