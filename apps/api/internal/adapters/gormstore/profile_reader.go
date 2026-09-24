package gormstore

import (
	"context"
	"errors"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

// TimeZone returns the active participant's effective IANA time zone through a
// user-scoped preference read.
func (s *Store) TimeZone(ctx context.Context, userID string) (string, error) {
	if s == nil || s.DB == nil || userID == "" {
		return "", ports.ErrInvalidArgument
	}
	var preference struct{ CurrentTimeZone string }
	result := s.DB.WithContext(ctx).
		Table("user_preference_models AS preferences").
		Select("preferences.current_time_zone").
		Joins("JOIN user_models AS users ON users.id = preferences.user_id AND users.status = ?", identity.StatusActive).
		Where("preferences.user_id = ?", userID).
		Take(&preference)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return "", ports.ErrNotFound
	}
	if result.Error != nil {
		return "", result.Error
	}
	if identity.ValidateIANATimeZone(identity.IANATimeZone(preference.CurrentTimeZone)) != nil {
		return "", errors.New("active profile time zone is invalid")
	}
	return preference.CurrentTimeZone, nil
}

var errInvalidPersistedPathCreationProfile = errors.New("active path creation profile is invalid")

// PathCreationProfile returns the active user's path-creation preferences. The
// user identifier is always part of the single joined query so callers cannot
// perform an unscoped profile or preference read through this adapter.
func (s *Store) PathCreationProfile(ctx context.Context, userID string) (pathapp.PathCreationProfile, error) {
	if s == nil || s.DB == nil || userID == "" {
		return pathapp.PathCreationProfile{}, ports.ErrInvalidArgument
	}
	var persisted struct {
		ProfileVisibility *identity.ProfileVisibility
		FirstDayOfWeek    int16
	}
	result := s.DB.WithContext(ctx).
		Table("user_models AS users").
		Select("users.profile_visibility AS profile_visibility, preferences.first_day_of_week AS first_day_of_week").
		Joins("JOIN user_preference_models AS preferences ON preferences.user_id = users.id").
		Where("users.id = ? AND users.status = ?", userID, identity.StatusActive).
		Take(&persisted)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return pathapp.PathCreationProfile{}, ports.ErrNotFound
	}
	if result.Error != nil {
		return pathapp.PathCreationProfile{}, result.Error
	}
	return pathCreationProfileFromPersistence(persisted.ProfileVisibility, persisted.FirstDayOfWeek)
}

func pathCreationProfileFromPersistence(profileVisibility *identity.ProfileVisibility, persistedFirstDayOfWeek int16) (pathapp.PathCreationProfile, error) {
	if profileVisibility == nil || identity.ValidateProfileVisibility(*profileVisibility) != nil ||
		persistedFirstDayOfWeek < int16(identity.FirstDayMonday) || persistedFirstDayOfWeek > int16(identity.FirstDaySunday) {
		return pathapp.PathCreationProfile{}, errInvalidPersistedPathCreationProfile
	}
	firstDayOfWeek := identity.FirstDayOfWeek(persistedFirstDayOfWeek)
	if identity.ValidateFirstDayOfWeek(firstDayOfWeek) != nil {
		return pathapp.PathCreationProfile{}, errInvalidPersistedPathCreationProfile
	}
	return pathapp.PathCreationProfile{ProfileVisibility: *profileVisibility, FirstDayOfWeek: firstDayOfWeek}, nil
}
