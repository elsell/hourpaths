package app

import (
	"context"
	"crypto/sha256"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const UpdateTimeZonePreferenceOperation = "account.time_zone.update"

type TimeZonePreference struct {
	TimeZone    string
	EffectiveAt time.Time
}

type TimeZonePreferenceUpdate struct {
	ReviewedTimeZone string
	ProposedTimeZone string
	Confirmed        bool
}

type TimeZonePreferenceCommand struct {
	ActorUserID, ReviewedTimeZone, ProposedTimeZone string
	ChangedAt                                       time.Time
	Idempotency                                     ports.Idempotency
	Audit                                           audit.Event
}

type TimeZonePreferenceResult struct {
	Preference TimeZonePreference
	Changed    bool
	Replayed   bool
}

type TimeZonePreferenceRepository interface {
	GetTimeZonePreference(context.Context, string) (TimeZonePreference, error)
	UpdateTimeZonePreference(context.Context, TimeZonePreferenceCommand) (TimeZonePreferenceResult, error)
}

func (a App) ConfiguredTimeZone(ctx context.Context, authorization string) (TimeZonePreference, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return TimeZonePreference{}, err
	}
	if a.TimeZonePreferences == nil || a.Audits == nil {
		return TimeZonePreference{}, ports.ErrUnavailable
	}
	preference, err := a.TimeZonePreferences.GetTimeZonePreference(ctx, user.ID)
	if err != nil {
		return TimeZonePreference{}, err
	}
	if !validTimeZonePreference(preference) {
		return TimeZonePreference{}, ports.ErrUnavailable
	}
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceViewed, "time_zone_preference", user.ID, audit.Succeeded)
	if err := a.Audits.AppendAuditEvent(ctx, event); err != nil {
		return TimeZonePreference{}, err
	}
	return preference, nil
}

func (a App) UpdateConfiguredTimeZone(ctx context.Context, authorization, idempotencyKey string, input TimeZonePreferenceUpdate) (TimeZonePreferenceResult, error) {
	user, err := a.authenticateCurrentUser(ctx, authorization)
	if err != nil {
		return TimeZonePreferenceResult{}, err
	}
	if a.TimeZonePreferences == nil || a.Clock == nil || !input.Confirmed || !validTimeZoneMutationKey(idempotencyKey) ||
		identity.ValidateIANATimeZone(identity.IANATimeZone(input.ReviewedTimeZone)) != nil ||
		identity.ValidateIANATimeZone(identity.IANATimeZone(input.ProposedTimeZone)) != nil {
		return TimeZonePreferenceResult{}, ports.ErrInvalidArgument
	}
	changedAt := a.Clock.Now().UTC().Truncate(time.Microsecond)
	if changedAt.IsZero() {
		return TimeZonePreferenceResult{}, ports.ErrUnavailable
	}
	hash := sha256.Sum256([]byte(input.ReviewedTimeZone + "\x00" + input.ProposedTimeZone + "\x00confirmed"))
	command := TimeZonePreferenceCommand{
		ActorUserID: user.ID, ReviewedTimeZone: input.ReviewedTimeZone, ProposedTimeZone: input.ProposedTimeZone, ChangedAt: changedAt,
		Idempotency: ports.Idempotency{PrincipalID: user.ID, Operation: UpdateTimeZonePreferenceOperation, Key: idempotencyKey, RequestHash: hash[:]},
		Audit:       a.auditEvent(ctx, user.ID, user.ID, audit.ResourceUpdated, "time_zone_preference", user.ID, audit.Succeeded),
	}
	command.Audit.OccurredAt = changedAt
	result, err := a.TimeZonePreferences.UpdateTimeZonePreference(ctx, command)
	if err != nil {
		return TimeZonePreferenceResult{}, err
	}
	if !validTimeZonePreference(result.Preference) || result.Preference.TimeZone != input.ProposedTimeZone ||
		(result.Changed && result.Preference.EffectiveAt != changedAt) {
		return TimeZonePreferenceResult{}, ports.ErrUnavailable
	}
	return result, nil
}

func validTimeZonePreference(preference TimeZonePreference) bool {
	return !preference.EffectiveAt.IsZero() && preference.EffectiveAt.Location() == time.UTC &&
		identity.ValidateIANATimeZone(identity.IANATimeZone(preference.TimeZone)) == nil
}

func validTimeZoneMutationKey(value string) bool {
	if len(value) < 16 || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
