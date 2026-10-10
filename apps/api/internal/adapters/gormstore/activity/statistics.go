package activitystore

import (
	"context"
	"database/sql"
	"errors"
	application "github.com/elsell/hour-paths/apps/api/internal/app/stats"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

// ReadPath shares the exact current-participant, block and Path visibility
// predicates used by session history. No notes or revision metadata enter stats.
func (r *Repository) ReadPath(ctx context.Context, viewerID, pathID, participantID string) (application.Snapshot, error) {
	empty := application.Snapshot{}
	if r == nil || r.DB == nil || viewerID == "" || pathID == "" || participantID == "" {
		return empty, ports.ErrInvalidArgument
	}
	snapshot := application.Snapshot{Records: []domain.Record{}}
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		subject := tx.Table("(SELECT ?::text AS path_id, ?::text AS participant_id) AS activity", pathID, participantID).Select("1 AS allowed")
		subject = scopeReadableActivity(subject, "activity", viewerID, pathID)
		subject = subject.Where("EXISTS (SELECT 1 FROM user_models participant WHERE participant.id=? AND participant.status='active')", participantID)
		var permission struct{ Allowed int }
		if err := subject.Take(&permission).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		var preferences struct {
			CurrentTimeZone string
			FirstDayOfWeek  int
		}
		if err := tx.Table("user_preference_models p").Select("p.current_time_zone,p.first_day_of_week").Joins("JOIN user_models u ON u.id=p.user_id AND u.status='active'").Where("p.user_id=?", viewerID).Take(&preferences).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		snapshot.TimeZone = preferences.CurrentTimeZone
		snapshot.FirstDayOfWeek = preferences.FirstDayOfWeek
		var rows []struct {
			PathID, OccurrenceTimeZone string
			StartedAt, EndedAt         time.Time
		}
		query := tx.Table("recorded_activity_models AS activity").Select("activity.path_id,activity.started_at,activity.ended_at,activity.occurrence_time_zone").Where("activity.path_id=? AND activity.participant_id=?", pathID, participantID)
		if err := scopeReadableActivity(query, "activity", viewerID, pathID).Order("activity.started_at,activity.id").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			snapshot.Records = append(snapshot.Records, domain.Record{PathID: row.PathID, StartedAt: row.StartedAt, EndedAt: row.EndedAt, TimeZone: row.OccurrenceTimeZone})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return empty, err
	}
	return snapshot, nil
}
