package gormstore

import (
	"context"
	"database/sql"
	application "github.com/elsell/hour-paths/apps/api/internal/app/stats"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

type StatsRepository struct{ DB *gorm.DB }

func (r StatsRepository) Read(ctx context.Context, userID string) (application.Snapshot, error) {
	s := application.Snapshot{Paths: []domain.StatsPath{}, Records: []domain.Record{}}
	if userID == "" || r.DB == nil {
		return s, ports.ErrInvalidArgument
	}
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var prefs struct {
			CurrentTimeZone string
			FirstDayOfWeek  int
		}
		if err := tx.Table("user_preference_models AS p").Select("p.current_time_zone,p.first_day_of_week").Joins("JOIN user_models u ON u.id=p.user_id AND u.status='active'").Where("p.user_id=?", userID).Take(&prefs).Error; err != nil {
			return err
		}
		s.TimeZone = prefs.CurrentTimeZone
		s.FirstDayOfWeek = prefs.FirstDayOfWeek
		if err := tx.Table("path_models p").Select("p.id,p.name,p.archived_at IS NOT NULL AS archived").Joins("JOIN path_membership_models m ON m.path_id=p.id AND m.user_id=?", userID).Order("p.name,p.id").Scan(&s.Paths).Error; err != nil {
			return err
		}
		var rows []struct {
			PathID             string
			StartedAt, EndedAt time.Time
			OccurrenceTimeZone string
		}
		if err := tx.Table("recorded_activity_models a").Select("a.path_id,a.started_at,a.ended_at,a.occurrence_time_zone").Joins("JOIN path_membership_models m ON m.path_id=a.path_id AND m.user_id=?", userID).Where("a.participant_id=?", userID).Order("a.started_at,a.id").Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			s.Records = append(s.Records, domain.Record{PathID: row.PathID, StartedAt: row.StartedAt, EndedAt: row.EndedAt, TimeZone: row.OccurrenceTimeZone})
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return s, err
}
