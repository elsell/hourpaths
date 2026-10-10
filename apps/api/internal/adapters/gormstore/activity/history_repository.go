package activitystore

import (
	"context"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

// ListOwnActivityCandidates avoids enumerating empty Paths. Current source
// membership implies the same own-participant Path visibility as scoped history;
// the application still requires a SpiceDB view decision for every returned Path.
func (r *Repository) ListOwnActivityCandidates(ctx context.Context, owner string, page application.OwnHistoryPageRequest) (application.ActivityPage, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(owner) == "" || page.ParticipantID != owner || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() || page.EndedSince.After(page.Snapshot) || (page.AfterID == "") != page.AfterStartedAt.IsZero() || page.AfterStartedAt.After(page.Snapshot) {
		return application.ActivityPage{}, ports.ErrInvalidArgument
	}
	return r.listActivitySnapshots(ctx, owner, "", page.ActivityPageRequest, page.EndedSince, true)
}

func (r *Repository) listActivitySnapshots(ctx context.Context, viewerID, pathID string, page application.ActivityPageRequest, endedSince time.Time, ownedOnly bool) (application.ActivityPage, error) {
	type activityProjection struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone, PathName string
		StartedAt, EndedAt, CreatedAt, UpdatedAt                time.Time
		Note                                                    *string
		Version                                                 int64
	}
	var rows []activityProjection
	futureRevision := r.DB.WithContext(ctx).Table("recorded_activity_revision_models AS future_revision").
		Select("future_revision.activity_id, future_revision.started_at, future_revision.ended_at, future_revision.occurrence_time_zone, future_revision.note, future_revision.updated_at").
		Where("future_revision.activity_id = activity.id AND NOT future_revision.superseded AND future_revision.replaced_at > ?", page.Snapshot).
		Order("future_revision.replaced_at ASC, future_revision.version ASC").Limit(1)
	projection := r.DB.WithContext(ctx).Table("recorded_activity_models AS activity").
		Select("source_path.name AS path_name, activity.id, activity.path_id, activity.participant_id, CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.started_at ELSE activity.started_at END AS started_at, CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.ended_at ELSE activity.ended_at END AS ended_at, CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.occurrence_time_zone ELSE activity.occurrence_time_zone END AS occurrence_time_zone, CASE WHEN activity.participant_id = ? THEN CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.note ELSE activity.note END ELSE NULL END AS note, activity.created_at, CASE WHEN activity.participant_id = ? THEN CASE WHEN snapshot_revision.activity_id IS NOT NULL THEN snapshot_revision.updated_at ELSE activity.updated_at END ELSE COALESCE((SELECT MAX(replaced_at) FROM recorded_activity_revision_models AS public_revision WHERE public_revision.activity_id = activity.id AND public_revision.public_changed AND public_revision.replaced_at <= ?), activity.created_at) END AS updated_at, 1 + (SELECT COUNT(*) FROM recorded_activity_revision_models AS revision WHERE revision.activity_id = activity.id AND revision.replaced_at <= ? AND (activity.participant_id = ? OR revision.public_changed)) AS version", viewerID, viewerID, page.Snapshot, page.Snapshot, viewerID).
		Joins("LEFT JOIN LATERAL (?) AS snapshot_revision ON TRUE", futureRevision).
		Joins("JOIN path_models AS source_path ON source_path.id = activity.path_id").
		Where("activity.created_at <= ?", page.Snapshot)
	if !ownedOnly {
		projection = projection.Where("activity.path_id = ?", pathID)
	}
	if page.ParticipantID != "" {
		projection = projection.Where("activity.participant_id = ?", page.ParticipantID)
	}
	if ownedOnly {
		projection = scopeCurrentActivitySource(projection, "activity")
		projection = scopeActivityParticipantBlock(projection, "activity", viewerID)
	} else {
		projection = scopeReadableActivity(projection, "activity", viewerID, pathID)
	}
	query := r.DB.WithContext(ctx).Table("(?) AS activity_snapshot", projection)
	if !endedSince.IsZero() {
		query = query.Where("activity_snapshot.ended_at >= ?", endedSince)
	}
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
		result = append(result, application.ActivityListRecord{PathName: row.PathName, Activity: toActivity(activityModel{
			ID: row.ID, PathID: row.PathID, ParticipantID: row.ParticipantID,
			StartedAt: row.StartedAt, EndedAt: row.EndedAt, OccurrenceTimeZone: row.OccurrenceTimeZone,
			Note: row.Note, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}), Version: row.Version})
	}
	ids := make([]string, 0, len(result))
	for _, record := range result {
		if record.Activity.ParticipantID == viewerID {
			ids = append(ids, record.Activity.ID)
		}
	}
	orders, err := loadOwnerEditOrders(r.DB.WithContext(ctx), viewerID, ids)
	if err != nil {
		return application.ActivityPage{}, err
	}
	for index := range result {
		result[index].Activity.EditOrder = orders[result[index].Activity.ID]
	}
	return application.ActivityPage{Items: result, HasMore: hasMore}, nil
}
