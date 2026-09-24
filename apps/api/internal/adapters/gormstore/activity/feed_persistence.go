package activitystore

import (
	"errors"
	"time"

	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type practiceFeedEventModel struct {
	ID                string `gorm:"primaryKey"`
	SourceActivityID  string
	AchievementID     *string
	ParticipantUserID string
	PathID            string
	PublishedAt       time.Time
}

func (practiceFeedEventModel) TableName() string { return "social_feed_event_models" }

func persistCompletedActivity(tx *gorm.DB, activity activitydomain.RecordedActivity) error {
	if err := tx.Create(fromActivity(activity)).Error; err != nil {
		return err
	}
	if err := publishPracticeFeedEvent(tx, activity); err != nil {
		return err
	}
	return publishGoalAchievements(tx, activity)
}

func publishPracticeFeedEvent(tx *gorm.DB, activity activitydomain.RecordedActivity) error {
	event, err := socialdomain.NewPracticeSessionEvent(activity)
	if err != nil {
		return err
	}
	want := practiceFeedEventModel{
		ID: event.ID, SourceActivityID: event.SourceActivityID,
		ParticipantUserID: event.ParticipantID, PathID: event.PathID,
		PublishedAt: postgresInstant(event.PublishedAt),
	}
	created := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "source_activity_id"}},
		DoNothing: true,
	}).Create(&want)
	if created.Error != nil {
		return created.Error
	}
	if created.RowsAffected == 1 {
		return nil
	}

	var existing practiceFeedEventModel
	if err := tx.Where("source_activity_id = ?", event.SourceActivityID).First(&existing).Error; err != nil {
		return err
	}
	if existing.ID != want.ID || existing.SourceActivityID != want.SourceActivityID || existing.ParticipantUserID != want.ParticipantUserID || existing.PathID != want.PathID || !existing.PublishedAt.Equal(want.PublishedAt) {
		return errors.New("persisted practice feed event conflicts with source activity")
	}
	return nil
}
