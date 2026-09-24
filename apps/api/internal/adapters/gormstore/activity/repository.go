package activitystore

import (
	"context"
	"errors"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct{ DB *gorm.DB }

type timerModel struct {
	ID                 string `gorm:"primaryKey"`
	PathID             string
	ParticipantID      string
	StartedAt          time.Time
	OccurrenceTimeZone string
}

func (timerModel) TableName() string { return "running_timer_models" }

type activityModel struct {
	ID                 string `gorm:"primaryKey"`
	PathID             string
	ParticipantID      string
	StartedAt          time.Time
	EndedAt            time.Time
	OccurrenceTimeZone string
	Note               *string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type activityRevisionModel struct {
	ActivityID         string `gorm:"primaryKey"`
	Version            int64  `gorm:"primaryKey"`
	StartedAt          time.Time
	EndedAt            time.Time
	OccurrenceTimeZone string
	Note               *string
	PublicChanged      bool
	UpdatedAt          time.Time
	ReplacedAt         time.Time
}

func (activityRevisionModel) TableName() string { return "recorded_activity_revision_models" }

func (activityModel) TableName() string { return "recorded_activity_models" }

type mutationModel struct {
	ParticipantID string `gorm:"primaryKey"`
	Operation     string `gorm:"primaryKey"`
	Key           string `gorm:"primaryKey"`
	RequestHash   []byte

	TimerID                       *string
	PathID                        string
	ResultStartedAt               *time.Time
	ResultTimeZone                string
	ResultActivityID              *string
	ResultEndedAt                 *time.Time
	ResultActivitySaved           bool
	ResultCreatedAt               *time.Time
	ResultUpdatedAt               *time.Time
	ResultNote                    *string
	ResultVersion                 *int64
	ResultActivityDeleted         bool
	ResultAccumulatedSeconds      *int64
	ResultSessionCount            *int64
	ResultUnreadNotificationCount *int64
	ResultRemovedFeedEventIDs     pq.StringArray `gorm:"type:text[]"`
}

func (mutationModel) TableName() string { return "activity_mutation_models" }

type auditModel struct {
	ID, OwnerUserID, ActorUserID, TargetType, TargetID, CorrelationID string
	Action                                                            audit.Action
	Outcome                                                           audit.Outcome
	OccurredAt                                                        time.Time
}

func (auditModel) TableName() string { return "audit_event_models" }

func New(db *gorm.DB) *Repository { return &Repository{DB: db} }

func (r *Repository) StartTimer(ctx context.Context, command application.StartTimerCommand) (application.StartTimerResult, error) {
	if r == nil || r.DB == nil || !validIdempotency(command.Idempotency, command.Timer.ParticipantID, application.StartTimerOperation) || !validTimer(command.Timer) || !validTimerAudit(command.Audit, audit.ActivityTimerStarted, command.Timer) || !validIntervalProgressRequest(command.IntervalProgress) {
		return application.StartTimerResult{}, ports.ErrInvalidArgument
	}

	var result application.StartTimerResult
	var activeTimerConflict bool
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActivePathAt(tx, command.Timer.PathID, command.Timer.ParticipantID, command.Timer.StartedAt); err != nil {
			return err
		}
		canonicalTimer := command.Timer
		canonicalTimer.StartedAt = postgresInstant(command.Timer.StartedAt)
		startedAt := canonicalTimer.StartedAt
		timerID := canonicalTimer.ID
		mutation := mutationModel{
			ParticipantID:   canonicalTimer.ParticipantID,
			Operation:       command.Idempotency.Operation,
			Key:             command.Idempotency.Key,
			RequestHash:     append([]byte(nil), command.Idempotency.RequestHash...),
			TimerID:         &timerID,
			PathID:          canonicalTimer.PathID,
			ResultStartedAt: &startedAt,
			ResultTimeZone:  canonicalTimer.OccurrenceTimeZone,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&mutation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, err := findMutation(tx, command.Idempotency)
			if err != nil {
				return err
			}
			if !sameHash(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			projection, err := currentProjection(tx, command.Timer.ParticipantID, command.Timer.PathID, command.IntervalProgress)
			if err != nil {
				return err
			}
			result = application.StartTimerResult{AccumulatedSeconds: projection.AccumulatedSeconds, IntervalProgress: projection.IntervalProgress, Replayed: true}
			if projection.Timer != nil {
				result.Timer = *projection.Timer
			}
			return nil
		}

		inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(fromTimer(canonicalTimer))
		if inserted.Error != nil {
			return inserted.Error
		}
		if inserted.RowsAffected != 1 {
			// Keep the idempotency reservation durable even though this distinct
			// request found an existing active timer. A delayed retry must surface
			// current state rather than unexpectedly starting a new timer after the
			// original one has stopped.
			projection, err := currentProjection(tx, canonicalTimer.ParticipantID, canonicalTimer.PathID, command.IntervalProgress)
			if err != nil {
				return err
			}
			if projection.Timer == nil {
				return ports.ErrConflict
			}
			viewAudit := command.Audit
			viewAudit.Action = audit.ResourceViewed
			viewAudit.TargetID = projection.Timer.ID
			if err := tx.Create(fromAudit(viewAudit)).Error; err != nil {
				return err
			}
			result = application.StartTimerResult{Timer: *projection.Timer, AccumulatedSeconds: projection.AccumulatedSeconds, IntervalProgress: projection.IntervalProgress}
			activeTimerConflict = true
			return nil
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		projection, err := currentProjection(tx, command.Timer.ParticipantID, command.Timer.PathID, command.IntervalProgress)
		if err != nil {
			return err
		}
		result = application.StartTimerResult{Timer: canonicalTimer, AccumulatedSeconds: projection.AccumulatedSeconds, IntervalProgress: projection.IntervalProgress}
		return nil
	})
	if err == nil && activeTimerConflict {
		return result, ports.ErrConflict
	}
	return result, err
}

func (r *Repository) StopTimer(ctx context.Context, command application.StopTimerCommand) (application.StopTimerResult, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(command.TimerID) == "" || strings.TrimSpace(command.PathID) == "" || strings.TrimSpace(command.ParticipantID) == "" || command.StoppedAt.IsZero() || command.RecordedAt.IsZero() || !validIdempotency(command.Idempotency, command.ParticipantID, application.StopTimerOperation) || !validStopAudit(command.Audit, command) || !validIntervalProgressRequest(command.IntervalProgress) {
		return application.StopTimerResult{}, ports.ErrInvalidArgument
	}

	var result application.StopTimerResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActivePath(tx, command.PathID, command.ParticipantID); err != nil {
			return err
		}
		timerID := command.TimerID
		reservation := mutationModel{
			ParticipantID: command.ParticipantID,
			Operation:     command.Idempotency.Operation,
			Key:           command.Idempotency.Key,
			RequestHash:   append([]byte(nil), command.Idempotency.RequestHash...),
			TimerID:       &timerID,
			PathID:        command.PathID,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, err := findMutation(tx, command.Idempotency)
			if err != nil {
				return err
			}
			if !sameHash(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = mutationStopResult(existing)
			result.Replayed = true
			projection, projectionErr := currentProjection(tx, command.ParticipantID, command.PathID, command.IntervalProgress)
			result.CurrentTimer, result.AccumulatedSeconds, result.IntervalProgress = projection.Timer, projection.AccumulatedSeconds, projection.IntervalProgress
			err = projectionErr
			if err != nil {
				return err
			}
			return nil
		}

		var row timerModel
		deleted := tx.Clauses(clause.Returning{}).
			Where("id = ? AND participant_id = ? AND path_id = ?", command.TimerID, command.ParticipantID, command.PathID).
			Delete(&row)
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		entry, saved, err := toTimer(row).Stop(command.ActivityID, postgresInstant(command.StoppedAt), postgresInstant(command.RecordedAt))
		if err != nil {
			return ports.ErrInvalidArgument
		}
		if saved {
			if err := persistCompletedActivity(tx, entry); err != nil {
				return err
			}
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}

		updates := mutationResultUpdates(row, entry, saved)
		updated := tx.Model(&mutationModel{}).Where("participant_id = ? AND operation = ? AND key = ?", command.ParticipantID, command.Idempotency.Operation, command.Idempotency.Key).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		projection, err := currentProjection(tx, command.ParticipantID, command.PathID, command.IntervalProgress)
		if err != nil {
			return err
		}
		result = application.StopTimerResult{Activity: entry, Saved: saved, CurrentTimer: projection.Timer, AccumulatedSeconds: projection.AccumulatedSeconds, IntervalProgress: projection.IntervalProgress}
		return nil
	})
	return result, err
}

func (r *Repository) CreateManualActivity(ctx context.Context, command application.CreateManualActivityCommand) (application.CreateManualActivityResult, error) {
	if r == nil || r.DB == nil || !validIdempotency(command.Idempotency, command.Activity.ParticipantID, application.CreateManualActivityOperation) || !validActivityAudit(command.Audit, audit.ResourceCreated, command.Activity.ParticipantID, command.Activity.ID) || !validIntervalProgressRequest(command.IntervalProgress) {
		return application.CreateManualActivityResult{}, ports.ErrInvalidArgument
	}
	entry := command.Activity
	entry.StartedAt = postgresInstant(entry.StartedAt)
	entry.EndedAt = postgresInstant(entry.EndedAt)
	entry.CreatedAt = postgresInstant(entry.CreatedAt)
	entry.UpdatedAt = postgresInstant(entry.UpdatedAt)
	canonical, err := domain.RecordManualActivity(domain.ManualActivity{
		ID: entry.ID, PathID: entry.PathID, ParticipantID: entry.ParticipantID,
		StartedAt: entry.StartedAt, DurationSeconds: entry.DurationSeconds(),
		OccurrenceTimeZone: entry.OccurrenceTimeZone, Note: entry.Note,
	}, entry.CreatedAt)
	if err != nil || canonical != entry {
		return application.CreateManualActivityResult{}, ports.ErrInvalidArgument
	}

	var result application.CreateManualActivityResult
	err = r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActivePathAt(tx, entry.PathID, entry.ParticipantID, entry.StartedAt); err != nil {
			return err
		}
		version := int64(1)
		startedAt, endedAt, createdAt, updatedAt, activityID := entry.StartedAt, entry.EndedAt, entry.CreatedAt, entry.UpdatedAt, entry.ID
		reservation := mutationModel{
			ParticipantID: entry.ParticipantID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), PathID: entry.PathID,
			ResultStartedAt: &startedAt, ResultTimeZone: entry.OccurrenceTimeZone, ResultActivityID: &activityID,
			ResultEndedAt: &endedAt, ResultActivitySaved: true, ResultCreatedAt: &createdAt, ResultUpdatedAt: &updatedAt,
			ResultNote: optionalNote(entry.Note), ResultVersion: &version,
		}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			existing, findErr := findMutation(tx, command.Idempotency)
			if findErr != nil {
				return findErr
			}
			if !sameHash(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result, findErr = mutationCreateResult(existing)
			if findErr != nil {
				return findErr
			}
			result.Replayed = true
			projection, projectionErr := currentProjection(tx, entry.ParticipantID, entry.PathID, command.IntervalProgress)
			result.AccumulatedSeconds, result.IntervalProgress = projection.AccumulatedSeconds, projection.IntervalProgress
			return projectionErr
		}
		if err := persistCompletedActivity(tx, entry); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		projection, err := currentProjection(tx, entry.ParticipantID, entry.PathID, command.IntervalProgress)
		if err != nil {
			return err
		}
		result = application.CreateManualActivityResult{Activity: entry, Version: 1, AccumulatedSeconds: projection.AccumulatedSeconds, IntervalProgress: projection.IntervalProgress}
		return nil
	})
	return result, err
}

func (r *Repository) UpdateActivity(ctx context.Context, command application.UpdateActivityCommand) (application.UpdateActivityResult, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(command.ActivityID) == "" || strings.TrimSpace(command.PathID) == "" || strings.TrimSpace(command.ParticipantID) == "" || command.UpdatedAt.IsZero() || !validIdempotency(command.Idempotency, command.ParticipantID, application.UpdateActivityOperation) || !validActivityAudit(command.Audit, audit.ResourceUpdated, command.ParticipantID, command.ActivityID) || !validIntervalProgressRequest(command.IntervalProgress) {
		return application.UpdateActivityResult{}, ports.ErrInvalidArgument
	}
	command.UpdatedAt = postgresInstant(command.UpdatedAt)
	command.Edit.StartedAt = postgresInstant(command.Edit.StartedAt)
	var result application.UpdateActivityResult
	reservationDuplicated := false
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockActivePath(tx, command.PathID, command.ParticipantID); err != nil {
			return err
		}
		if replay, found, err := replayedUpdate(tx, command); err != nil || found {
			result = replay
			return err
		}
		var current activityModel
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND participant_id = ? AND path_id = ?", command.ActivityID, command.ParticipantID, command.PathID).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ports.ErrNotFound
		}
		if err != nil {
			return err
		}
		if replay, found, err := replayedUpdate(tx, command); err != nil || found {
			result = replay
			return err
		}
		prior := toActivity(current)
		edited, revision, err := prior.EditByOwner(command.ParticipantID, command.Edit, command.UpdatedAt)
		if err != nil {
			return ports.ErrInvalidArgument
		}
		var revisionCount int64
		if err := tx.Model(&activityRevisionModel{}).Where("activity_id = ?", command.ActivityID).Count(&revisionCount).Error; err != nil {
			return err
		}
		priorVersion, resultVersion := revisionCount+1, revisionCount+2
		startedAt, endedAt, createdAt, updatedAt, activityID := edited.StartedAt, edited.EndedAt, edited.CreatedAt, edited.UpdatedAt, edited.ID
		reservation := mutationModel{
			ParticipantID: command.ParticipantID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key,
			RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), PathID: command.PathID,
			ResultStartedAt: &startedAt, ResultTimeZone: edited.OccurrenceTimeZone, ResultActivityID: &activityID,
			ResultEndedAt: &endedAt, ResultActivitySaved: true, ResultCreatedAt: &createdAt, ResultUpdatedAt: &updatedAt,
			ResultNote: optionalNote(edited.Note), ResultVersion: &resultVersion,
		}
		if err := tx.Create(&reservation).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				reservationDuplicated = true
			}
			return err
		}
		publicChanged := prior.StartedAt != edited.StartedAt || prior.EndedAt != edited.EndedAt || prior.OccurrenceTimeZone != edited.OccurrenceTimeZone
		revisionRow := fromRevision(revision, priorVersion, publicChanged)
		if err := tx.Create(&revisionRow).Error; err != nil {
			return err
		}
		updates := map[string]any{"started_at": edited.StartedAt, "ended_at": edited.EndedAt, "occurrence_time_zone": edited.OccurrenceTimeZone, "note": optionalNote(edited.Note), "updated_at": edited.UpdatedAt}
		updated := tx.Model(&activityModel{}).Where("id = ? AND participant_id = ? AND path_id = ?", command.ActivityID, command.ParticipantID, command.PathID).Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		if _, err := removeUnsupportedGoalAchievements(tx, command.ParticipantID, command.PathID); err != nil {
			return err
		}
		projection, err := currentProjection(tx, command.ParticipantID, command.PathID, command.IntervalProgress)
		if err != nil {
			return err
		}
		result = application.UpdateActivityResult{Activity: edited, Revision: revision, Version: resultVersion, AccumulatedSeconds: projection.AccumulatedSeconds, IntervalProgress: projection.IntervalProgress}
		return nil
	})
	if reservationDuplicated && errors.Is(err, gorm.ErrDuplicatedKey) {
		replay, found, replayErr := replayedUpdate(r.DB.WithContext(ctx), command)
		if replayErr != nil {
			return application.UpdateActivityResult{}, replayErr
		}
		if found {
			return replay, nil
		}
		return application.UpdateActivityResult{}, ports.ErrIdempotencyConflict
	}
	return result, err
}

func (r *Repository) GetActivity(ctx context.Context, viewerID, pathID, activityID string) (domain.RecordedActivity, int64, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(viewerID) == "" || strings.TrimSpace(pathID) == "" || strings.TrimSpace(activityID) == "" {
		return domain.RecordedActivity{}, 0, ports.ErrInvalidArgument
	}
	var row activityModel
	activityQuery := r.DB.WithContext(ctx).Model(&activityModel{}).
		Select("id, path_id, participant_id, started_at, ended_at, occurrence_time_zone, CASE WHEN participant_id = ? THEN note ELSE NULL END AS note, created_at, updated_at", viewerID).
		Where("id = ? AND path_id = ?", activityID, pathID)
	err := scopeReadableActivity(activityQuery, "recorded_activity_models", viewerID, pathID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.RecordedActivity{}, 0, ports.ErrNotFound
	}
	if err != nil {
		return domain.RecordedActivity{}, 0, err
	}
	var revisionState struct {
		Count       int64
		LastUpdated *time.Time
	}
	query := r.DB.WithContext(ctx).Model(&activityRevisionModel{}).Where("activity_id = ?", activityID)
	if row.ParticipantID != viewerID {
		query = query.Where("public_changed = ?", true)
	}
	if err := query.Select("COUNT(*) AS count, MAX(replaced_at) AS last_updated").Scan(&revisionState).Error; err != nil {
		return domain.RecordedActivity{}, 0, err
	}
	if row.ParticipantID != viewerID {
		if revisionState.LastUpdated == nil {
			row.UpdatedAt = row.CreatedAt
		} else {
			row.UpdatedAt = revisionState.LastUpdated.UTC()
		}
	}
	return toActivity(row), revisionState.Count + 1, nil
}

func (r *Repository) ListActivities(ctx context.Context, viewerID, pathID string, page application.ActivityPageRequest) (application.ActivityPage, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(viewerID) == "" || strings.TrimSpace(pathID) == "" || (page.ParticipantID != "" && strings.TrimSpace(page.ParticipantID) != page.ParticipantID) || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() ||
		(page.AfterID == "") != page.AfterStartedAt.IsZero() || (!page.AfterStartedAt.IsZero() && page.AfterStartedAt.After(page.Snapshot)) {
		return application.ActivityPage{}, ports.ErrInvalidArgument
	}
	type activityProjection struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
		Note                                          *string
		Version                                       int64
	}
	var rows []activityProjection
	futureRevision := r.DB.WithContext(ctx).Table("recorded_activity_revision_models AS future_revision").
		Select("future_revision.activity_id, future_revision.started_at, future_revision.ended_at, future_revision.occurrence_time_zone, future_revision.note, future_revision.updated_at").
		Where("future_revision.activity_id = activity.id AND future_revision.replaced_at > ?", page.Snapshot).
		Order("future_revision.replaced_at ASC, future_revision.version ASC").Limit(1)
	projection := r.DB.WithContext(ctx).Table("recorded_activity_models AS activity").
		Select("activity.id, activity.path_id, activity.participant_id, CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.started_at ELSE activity.started_at END AS started_at, CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.ended_at ELSE activity.ended_at END AS ended_at, CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.occurrence_time_zone ELSE activity.occurrence_time_zone END AS occurrence_time_zone, CASE WHEN activity.participant_id = ? THEN CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.note ELSE activity.note END ELSE NULL END AS note, activity.created_at, CASE WHEN activity.participant_id = ? THEN CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.updated_at ELSE activity.updated_at END ELSE COALESCE((SELECT MAX(replaced_at) FROM recorded_activity_revision_models AS public_revision WHERE public_revision.activity_id = activity.id AND public_revision.public_changed AND public_revision.replaced_at <= ?), activity.created_at) END AS updated_at, 1 + (SELECT COUNT(*) FROM recorded_activity_revision_models AS revision WHERE revision.activity_id = activity.id AND revision.replaced_at <= ? AND (activity.participant_id = ? OR revision.public_changed)) AS version", viewerID, viewerID, page.Snapshot, page.Snapshot, viewerID).
		Joins("LEFT JOIN LATERAL (?) AS snapshot_revision ON TRUE", futureRevision).
		Where("activity.path_id = ? AND activity.created_at <= ?", pathID, page.Snapshot)
	if page.ParticipantID != "" {
		projection = projection.Where("activity.participant_id = ?", page.ParticipantID)
	}
	projection = scopeReadableActivity(projection, "activity", viewerID, pathID)
	query := r.DB.WithContext(ctx).Table("(?) AS activity_snapshot", projection)
	if page.AfterID != "" {
		query = query.Where("activity_snapshot.started_at < ? OR (activity_snapshot.started_at = ? AND activity_snapshot.id < ?)", page.AfterStartedAt, page.AfterStartedAt, page.AfterID)
	}
	err := query.Order("activity_snapshot.started_at DESC, activity_snapshot.id DESC").Limit(page.Limit + 1).Scan(&rows).Error
	if err != nil {
		return application.ActivityPage{}, err
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	result := make([]application.ActivityListRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, application.ActivityListRecord{Activity: toActivity(activityModel{
			ID: row.ID, PathID: row.PathID, ParticipantID: row.ParticipantID,
			StartedAt: row.StartedAt, EndedAt: row.EndedAt, OccurrenceTimeZone: row.OccurrenceTimeZone,
			Note: row.Note, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}), Version: row.Version})
	}
	return application.ActivityPage{Items: result, HasMore: hasMore}, nil
}

func (r *Repository) ListActivityRevisions(ctx context.Context, viewerID, pathID, activityID string, page application.ActivityRevisionPageRequest) (application.ActivityRevisionPage, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(viewerID) == "" || strings.TrimSpace(pathID) == "" || strings.TrimSpace(activityID) == "" || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() || page.BeforeVersion < 0 {
		return application.ActivityRevisionPage{}, ports.ErrInvalidArgument
	}
	type revisionProjection struct {
		ActivityID, PathID, ParticipantID, OccurrenceTimeZone string
		Version                                               int64
		StartedAt, EndedAt, UpdatedAt, ReplacedAt, CreatedAt  time.Time
		Note                                                  *string
	}
	var rows []revisionProjection
	var ownerID string
	lookupQuery := r.DB.WithContext(ctx).Model(&activityModel{}).Select("participant_id").Where("id = ? AND path_id = ?", activityID, pathID)
	lookup := scopeReadableActivity(lookupQuery, "recorded_activity_models", viewerID, pathID).Scan(&ownerID)
	if lookup.Error != nil {
		return application.ActivityRevisionPage{}, lookup.Error
	}
	if lookup.RowsAffected == 0 {
		return application.ActivityRevisionPage{}, ports.ErrNotFound
	}
	var err error
	if ownerID == viewerID {
		query := r.DB.WithContext(ctx).Table("recorded_activity_revision_models AS revision").
			Select("revision.activity_id, revision.version, revision.started_at, revision.ended_at, revision.occurrence_time_zone, revision.note, revision.updated_at, revision.replaced_at, current.path_id, current.participant_id, current.created_at").
			Joins("JOIN recorded_activity_models AS current ON current.id = revision.activity_id").
			Where("revision.activity_id = ? AND current.path_id = ? AND revision.replaced_at <= ?", activityID, pathID, page.Snapshot)
		query = scopeReadableActivity(query, "current", viewerID, pathID)
		if page.BeforeVersion > 0 {
			query = query.Where("revision.version < ?", page.BeforeVersion)
		}
		err = query.Order("revision.version DESC").Limit(page.Limit + 1).Scan(&rows).Error
	} else {
		publicRevisions := r.DB.WithContext(ctx).Table("recorded_activity_revision_models AS revision").
			Select("revision.activity_id, revision.started_at, revision.ended_at, revision.occurrence_time_zone, revision.replaced_at, current.path_id, current.participant_id, current.created_at, ROW_NUMBER() OVER (ORDER BY revision.version) AS public_version, LAG(revision.replaced_at) OVER (ORDER BY revision.version) AS prior_public_updated_at").
			Joins("JOIN recorded_activity_models AS current ON current.id = revision.activity_id").
			Where("revision.activity_id = ? AND current.path_id = ? AND revision.public_changed AND revision.replaced_at <= ?", activityID, pathID, page.Snapshot)
		publicRevisions = scopeReadableActivity(publicRevisions, "current", viewerID, pathID)
		query := r.DB.WithContext(ctx).Table("(?) AS public_revisions", publicRevisions).
			Select("activity_id, public_version AS version, started_at, ended_at, occurrence_time_zone, NULL AS note, COALESCE(prior_public_updated_at, created_at) AS updated_at, replaced_at, path_id, participant_id, created_at").
			Order("public_version DESC").Limit(page.Limit + 1)
		if page.BeforeVersion > 0 {
			query = query.Where("public_version < ?", page.BeforeVersion)
		}
		err = query.Scan(&rows).Error
	}
	if err != nil {
		return application.ActivityRevisionPage{}, err
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	if len(rows) == 0 {
		var count int64
		countQuery := r.DB.WithContext(ctx).Model(&activityModel{}).Where("id = ? AND path_id = ?", activityID, pathID)
		if err := scopeReadableActivity(countQuery, "recorded_activity_models", viewerID, pathID).Count(&count).Error; err != nil {
			return application.ActivityRevisionPage{}, err
		}
		if count == 0 {
			return application.ActivityRevisionPage{}, ports.ErrNotFound
		}
	}
	result := make([]application.ActivityRevisionRecord, 0, len(rows))
	for _, row := range rows {
		prior := domain.RecordedActivity{ID: row.ActivityID, PathID: row.PathID, ParticipantID: row.ParticipantID, StartedAt: row.StartedAt.UTC(), EndedAt: row.EndedAt.UTC(), OccurrenceTimeZone: row.OccurrenceTimeZone, Note: noteValue(row.Note), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
		result = append(result, application.ActivityRevisionRecord{Revision: domain.ActivityRevision{Activity: prior, ReplacedAt: row.ReplacedAt.UTC()}, Version: row.Version})
	}
	return application.ActivityRevisionPage{Items: result, HasMore: hasMore}, nil
}

func (r *Repository) AccumulatedSeconds(ctx context.Context, participantID, pathID string) (int64, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(participantID) == "" || strings.TrimSpace(pathID) == "" {
		return 0, ports.ErrInvalidArgument
	}
	return accumulatedSeconds(r.DB.WithContext(ctx), participantID, pathID)
}

func (r *Repository) GetRunningTimer(ctx context.Context, participantID, pathID string) (domain.RunningTimer, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(participantID) == "" || strings.TrimSpace(pathID) == "" {
		return domain.RunningTimer{}, ports.ErrInvalidArgument
	}
	var row timerModel
	err := r.DB.WithContext(ctx).Where("participant_id = ? AND path_id = ?", participantID, pathID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.RunningTimer{}, ports.ErrNotFound
	}
	if err != nil {
		return domain.RunningTimer{}, err
	}
	return toTimer(row), nil
}

func validIdempotency(value ports.Idempotency, participantID, operation string) bool {
	return value.PrincipalID == participantID && value.Operation == operation && strings.TrimSpace(value.Key) != "" && len(value.RequestHash) == 32
}

func validTimer(timer domain.RunningTimer) bool {
	canonical, err := domain.StartTimer(timer.ID, timer.PathID, timer.ParticipantID, timer.StartedAt, timer.OccurrenceTimeZone, timer.StartedAt)
	return err == nil && canonical == timer
}

func validTimerAudit(event audit.Event, action audit.Action, timer domain.RunningTimer) bool {
	return event.Valid() && event.Action == action && event.Outcome == audit.Succeeded && event.OwnerUserID == timer.ParticipantID && event.ActorUserID == timer.ParticipantID && event.TargetType == "timer" && event.TargetID == timer.ID
}

func validStopAudit(event audit.Event, command application.StopTimerCommand) bool {
	return event.Valid() && event.Action == audit.ActivityTimerStopped && event.Outcome == audit.Succeeded && event.OwnerUserID == command.ParticipantID && event.ActorUserID == command.ParticipantID && event.TargetType == "timer" && event.TargetID == command.TimerID
}

func validActivityAudit(event audit.Event, action audit.Action, participantID, activityID string) bool {
	return event.Valid() && event.Action == action && event.Outcome == audit.Succeeded && event.OwnerUserID == participantID && event.ActorUserID == participantID && event.TargetType == "activity" && event.TargetID == activityID
}

func sameHash(left, right []byte) bool { return string(left) == string(right) }

// PostgreSQL timestamptz stores microseconds. Canonicalizing before constructing
// mutation results makes the first response identical to every persisted replay.
func postgresInstant(value time.Time) time.Time {
	return value.UTC().Truncate(time.Microsecond)
}

func findMutation(db *gorm.DB, idempotency ports.Idempotency) (mutationModel, error) {
	var row mutationModel
	err := db.Where("participant_id = ? AND operation = ? AND key = ?", idempotency.PrincipalID, idempotency.Operation, idempotency.Key).First(&row).Error
	return row, err
}

type currentProjectionRow struct {
	TimerID            *string
	PathID             *string
	ParticipantID      *string
	StartedAt          *time.Time
	OccurrenceTimeZone *string
	AccumulatedSeconds int64
	IntervalSeconds    int64
}

func currentProjection(db *gorm.DB, participantID, pathID string, request *application.IntervalProgressRequest) (application.CurrentTimerResult, error) {
	totals := db.Model(&activityModel{}).Where("participant_id = ? AND path_id = ?", participantID, pathID)
	if request == nil {
		totals = totals.Select("COALESCE(SUM(FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))), 0)::bigint AS accumulated_seconds, 0::bigint AS interval_seconds")
	} else {
		totals = totals.Select(`COALESCE(SUM(FLOOR(EXTRACT(EPOCH FROM (ended_at - started_at)))), 0)::bigint AS accumulated_seconds,
			COALESCE(SUM(CASE WHEN started_at < ? AND ended_at > ? THEN
				FLOOR(EXTRACT(EPOCH FROM (LEAST(ended_at, ?) - started_at))) -
				FLOOR(EXTRACT(EPOCH FROM (GREATEST(started_at, ?) - started_at)))
			ELSE 0 END), 0)::bigint AS interval_seconds`, request.Window.EndedAt, request.Window.StartedAt, request.Window.EndedAt, request.Window.StartedAt)
	}
	var row currentProjectionRow
	err := db.Table("(?) AS totals", totals).
		Select("running.id AS timer_id, running.path_id, running.participant_id, running.started_at, running.occurrence_time_zone, totals.accumulated_seconds, totals.interval_seconds").
		Joins("LEFT JOIN running_timer_models AS running ON running.participant_id = ? AND running.path_id = ?", participantID, pathID).
		Scan(&row).Error
	if err != nil {
		return application.CurrentTimerResult{}, err
	}
	if row.AccumulatedSeconds < 0 || row.IntervalSeconds < 0 {
		return application.CurrentTimerResult{}, errors.New("persisted activity duration is invalid")
	}
	result := application.CurrentTimerResult{AccumulatedSeconds: row.AccumulatedSeconds}
	if request != nil {
		result.IntervalProgress = &application.IntervalProgress{TargetSeconds: request.TargetSeconds, AccumulatedSeconds: row.IntervalSeconds, Window: request.Window}
	}
	if row.TimerID == nil {
		return result, nil
	}
	if row.PathID == nil || row.ParticipantID == nil || row.StartedAt == nil || row.OccurrenceTimeZone == nil {
		return application.CurrentTimerResult{}, errors.New("persisted running timer projection is invalid")
	}
	timer := domain.RunningTimer{ID: *row.TimerID, PathID: *row.PathID, ParticipantID: *row.ParticipantID, StartedAt: row.StartedAt.UTC(), OccurrenceTimeZone: *row.OccurrenceTimeZone}
	result.Timer = &timer
	return result, nil
}

func mutationStopResult(row mutationModel) application.StopTimerResult {
	if !row.ResultActivitySaved || row.ResultActivityID == nil || row.ResultStartedAt == nil || row.ResultEndedAt == nil || row.ResultCreatedAt == nil || row.ResultUpdatedAt == nil {
		return application.StopTimerResult{}
	}
	return application.StopTimerResult{Saved: true, Activity: domain.RecordedActivity{
		ID: *row.ResultActivityID, PathID: row.PathID, ParticipantID: row.ParticipantID,
		StartedAt: row.ResultStartedAt.UTC(), EndedAt: row.ResultEndedAt.UTC(), OccurrenceTimeZone: row.ResultTimeZone,
		CreatedAt: row.ResultCreatedAt.UTC(), UpdatedAt: row.ResultUpdatedAt.UTC(),
	}}
}

func mutationCreateResult(row mutationModel) (application.CreateManualActivityResult, error) {
	entry, version, err := mutationActivity(row)
	if err != nil {
		return application.CreateManualActivityResult{}, err
	}
	return application.CreateManualActivityResult{Activity: entry, Version: version}, nil
}

func mutationActivity(row mutationModel) (domain.RecordedActivity, int64, error) {
	if row.ResultActivityDeleted {
		return domain.RecordedActivity{}, 0, ports.ErrNotFound
	}
	if !row.ResultActivitySaved || row.ResultActivityID == nil || row.ResultStartedAt == nil || row.ResultEndedAt == nil || row.ResultCreatedAt == nil || row.ResultUpdatedAt == nil || row.ResultVersion == nil || *row.ResultVersion < 1 {
		return domain.RecordedActivity{}, 0, errors.New("persisted activity mutation result is invalid")
	}
	return domain.RecordedActivity{
		ID: *row.ResultActivityID, PathID: row.PathID, ParticipantID: row.ParticipantID,
		StartedAt: row.ResultStartedAt.UTC(), EndedAt: row.ResultEndedAt.UTC(), OccurrenceTimeZone: row.ResultTimeZone,
		Note: noteValue(row.ResultNote), CreatedAt: row.ResultCreatedAt.UTC(), UpdatedAt: row.ResultUpdatedAt.UTC(),
	}, *row.ResultVersion, nil
}

func replayedUpdate(tx *gorm.DB, command application.UpdateActivityCommand) (application.UpdateActivityResult, bool, error) {
	existing, err := findMutation(tx, command.Idempotency)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.UpdateActivityResult{}, false, nil
	}
	if err != nil {
		return application.UpdateActivityResult{}, false, err
	}
	if !sameHash(existing.RequestHash, command.Idempotency.RequestHash) {
		return application.UpdateActivityResult{}, true, ports.ErrIdempotencyConflict
	}
	entry, version, err := mutationActivity(existing)
	if err != nil {
		return application.UpdateActivityResult{}, true, err
	}
	if version < 2 {
		return application.UpdateActivityResult{}, true, errors.New("persisted activity update version is invalid")
	}
	var row activityRevisionModel
	if err := tx.Where("activity_id = ? AND version = ?", command.ActivityID, version-1).First(&row).Error; err != nil {
		return application.UpdateActivityResult{}, true, err
	}
	prior := domain.RecordedActivity{
		ID: command.ActivityID, PathID: command.PathID, ParticipantID: command.ParticipantID,
		StartedAt: row.StartedAt.UTC(), EndedAt: row.EndedAt.UTC(), OccurrenceTimeZone: row.OccurrenceTimeZone,
		Note: noteValue(row.Note), CreatedAt: entry.CreatedAt, UpdatedAt: row.UpdatedAt.UTC(),
	}
	projection, err := currentProjection(tx, command.ParticipantID, command.PathID, command.IntervalProgress)
	if err != nil {
		return application.UpdateActivityResult{}, true, err
	}
	return application.UpdateActivityResult{Activity: entry, Revision: domain.ActivityRevision{Activity: prior, ReplacedAt: row.ReplacedAt.UTC()}, Version: version, AccumulatedSeconds: projection.AccumulatedSeconds, IntervalProgress: projection.IntervalProgress, Replayed: true}, true, nil
}

func mutationResultUpdates(timer timerModel, entry domain.RecordedActivity, saved bool) map[string]any {
	updates := map[string]any{
		"result_started_at":     timer.StartedAt,
		"result_time_zone":      timer.OccurrenceTimeZone,
		"result_activity_saved": saved,
	}
	if saved {
		updates["result_activity_id"] = entry.ID
		updates["result_ended_at"] = entry.EndedAt
		updates["result_created_at"] = entry.CreatedAt
		updates["result_updated_at"] = entry.UpdatedAt
	}
	return updates
}

func fromRevision(revision domain.ActivityRevision, version int64, publicChanged bool) activityRevisionModel {
	return activityRevisionModel{ActivityID: revision.Activity.ID, Version: version, StartedAt: revision.Activity.StartedAt, EndedAt: revision.Activity.EndedAt, OccurrenceTimeZone: revision.Activity.OccurrenceTimeZone, Note: optionalNote(revision.Activity.Note), PublicChanged: publicChanged, UpdatedAt: revision.Activity.UpdatedAt, ReplacedAt: revision.ReplacedAt}
}

func fromTimer(timer domain.RunningTimer) *timerModel {
	return &timerModel{ID: timer.ID, PathID: timer.PathID, ParticipantID: timer.ParticipantID, StartedAt: timer.StartedAt, OccurrenceTimeZone: timer.OccurrenceTimeZone}
}

func toTimer(row timerModel) domain.RunningTimer {
	return domain.RunningTimer{ID: row.ID, PathID: row.PathID, ParticipantID: row.ParticipantID, StartedAt: row.StartedAt.UTC(), OccurrenceTimeZone: row.OccurrenceTimeZone}
}

func fromActivity(activity domain.RecordedActivity) *activityModel {
	return &activityModel{ID: activity.ID, PathID: activity.PathID, ParticipantID: activity.ParticipantID, StartedAt: activity.StartedAt, EndedAt: activity.EndedAt, OccurrenceTimeZone: activity.OccurrenceTimeZone, Note: optionalNote(activity.Note), CreatedAt: activity.CreatedAt, UpdatedAt: activity.UpdatedAt}
}

func toActivity(row activityModel) domain.RecordedActivity {
	return domain.RecordedActivity{ID: row.ID, PathID: row.PathID, ParticipantID: row.ParticipantID, StartedAt: row.StartedAt.UTC(), EndedAt: row.EndedAt.UTC(), OccurrenceTimeZone: row.OccurrenceTimeZone, Note: noteValue(row.Note), CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC()}
}

func optionalNote(note string) *string {
	if note == "" {
		return nil
	}
	return &note
}

func noteValue(note *string) string {
	if note == nil {
		return ""
	}
	return *note
}
