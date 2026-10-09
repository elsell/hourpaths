package gormstore

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func (s *Store) ReadExportProfile(ctx context.Context, owner string) (app.AccountExportProfile, error) {
	if s == nil || s.DB == nil || owner == "" || strings.TrimSpace(owner) != owner {
		return app.AccountExportProfile{}, ports.ErrInvalidArgument
	}
	var row struct {
		ID, Email, DisplayName, CurrentTimeZone string
		Username, Description                   *string
		ProfileVisibility                       identity.ProfileVisibility
		FirstDayOfWeek                          int
		CreatedAt, UpdatedAt                    time.Time
	}
	err := s.DB.WithContext(ctx).Table("user_models AS users").
		Select("users.id, users.email, users.username, users.display_name, users.description, users.profile_visibility, users.created_at, users.updated_at, preferences.current_time_zone, preferences.first_day_of_week").
		Joins("JOIN user_preference_models AS preferences ON preferences.user_id = users.id").
		Where("users.id = ? AND users.status = ?", owner, identity.StatusActive).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return app.AccountExportProfile{}, ports.ErrNotFound
	}
	if err != nil {
		return app.AccountExportProfile{}, err
	}
	result := app.AccountExportProfile{UserID: row.ID, Email: row.Email, DisplayName: row.DisplayName, Visibility: row.ProfileVisibility, TimeZone: row.CurrentTimeZone, FirstDayOfWeek: row.FirstDayOfWeek, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
	if row.Username != nil {
		result.Username = *row.Username
	}
	if row.Description != nil {
		result.Description = *row.Description
	}
	return result, nil
}
