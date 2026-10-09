package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

const UpdateWeekStartPreferenceOperation = "account.week_start.update"

type WeekStartPreference struct{ FirstDayOfWeek int }
type WeekStartPreferenceUpdate struct{ ReviewedFirstDayOfWeek, ProposedFirstDayOfWeek int }
type WeekStartPreferenceCommand struct {
	ActorUserID                                    string
	ReviewedFirstDayOfWeek, ProposedFirstDayOfWeek int
	ChangedAt                                      time.Time
	Idempotency                                    ports.Idempotency
	Audit                                          audit.Event
}
type WeekStartPreferenceResult struct {
	Preference        WeekStartPreference
	Changed, Replayed bool
}
type WeekStartPreferenceRepository interface {
	GetWeekStartPreference(context.Context, string) (WeekStartPreference, error)
	UpdateWeekStartPreference(context.Context, WeekStartPreferenceCommand) (WeekStartPreferenceResult, error)
}

func validWeekStart(value int) bool {
	return value >= 1 && value <= 7 && identity.ValidateFirstDayOfWeek(identity.FirstDayOfWeek(value)) == nil
}
func (a App) ConfiguredWeekStart(ctx context.Context, authorization string) (WeekStartPreference, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return WeekStartPreference{}, err
	}
	if a.WeekStartPreferences == nil || a.Audits == nil {
		return WeekStartPreference{}, ports.ErrUnavailable
	}
	preference, err := a.WeekStartPreferences.GetWeekStartPreference(ctx, user.ID)
	if err != nil {
		return WeekStartPreference{}, err
	}
	if !validWeekStart(preference.FirstDayOfWeek) {
		return WeekStartPreference{}, ports.ErrUnavailable
	}
	if err = a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.ResourceViewed, "week_start_preference", user.ID, audit.Succeeded)); err != nil {
		return WeekStartPreference{}, err
	}
	return preference, nil
}
func (a App) UpdateConfiguredWeekStart(ctx context.Context, authorization, key string, input WeekStartPreferenceUpdate) (WeekStartPreferenceResult, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return WeekStartPreferenceResult{}, err
	}
	if a.WeekStartPreferences == nil || a.Clock == nil {
		return WeekStartPreferenceResult{}, ports.ErrUnavailable
	}
	if !validTimeZoneMutationKey(key) || !validWeekStart(input.ReviewedFirstDayOfWeek) || !validWeekStart(input.ProposedFirstDayOfWeek) {
		return WeekStartPreferenceResult{}, ports.ErrInvalidArgument
	}
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return WeekStartPreferenceResult{}, ports.ErrUnavailable
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", input.ReviewedFirstDayOfWeek, input.ProposedFirstDayOfWeek)))
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceUpdated, "week_start_preference", user.ID, audit.Succeeded)
	event.OccurredAt = now
	result, err := a.WeekStartPreferences.UpdateWeekStartPreference(ctx, WeekStartPreferenceCommand{ActorUserID: user.ID, ReviewedFirstDayOfWeek: input.ReviewedFirstDayOfWeek, ProposedFirstDayOfWeek: input.ProposedFirstDayOfWeek, ChangedAt: now, Idempotency: ports.Idempotency{PrincipalID: user.ID, Operation: UpdateWeekStartPreferenceOperation, Key: key, RequestHash: hash[:]}, Audit: event})
	if err != nil {
		return WeekStartPreferenceResult{}, err
	}
	if !validWeekStart(result.Preference.FirstDayOfWeek) || result.Preference.FirstDayOfWeek != input.ProposedFirstDayOfWeek {
		return WeekStartPreferenceResult{}, ports.ErrUnavailable
	}
	return result, nil
}
