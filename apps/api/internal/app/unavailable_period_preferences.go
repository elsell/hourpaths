package app

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/preferences"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"math"
	"time"
)

const UpdateUnavailablePeriodOperation = "account.unavailable_period.update"

type UnavailablePeriodPreference struct {
	Period   preferences.UnavailablePeriod
	Revision int64
	TimeZone string
}
type UnavailablePeriodCommand struct {
	ActorUserID      string
	ExpectedRevision int64
	ReviewedTimeZone string
	Period           preferences.UnavailablePeriod
	ChangedAt        time.Time
	Idempotency      ports.Idempotency
	Audit            audit.Event
}
type UnavailablePeriodResult struct {
	Preference UnavailablePeriodPreference
	Replayed   bool
}
type UnavailablePeriodRepository interface {
	GetUnavailablePeriod(context.Context, string) (UnavailablePeriodPreference, error)
	UpdateUnavailablePeriod(context.Context, UnavailablePeriodCommand) (UnavailablePeriodResult, error)
}

type UnavailablePeriodUpdate struct {
	ExpectedRevision int64
	ReviewedTimeZone string
	Period           preferences.UnavailablePeriod
}

func validUnavailablePeriodValue(value UnavailablePeriodPreference) bool {
	p := value.Period
	return value.Revision >= 0 && identity.ValidateIANATimeZone(identity.IANATimeZone(value.TimeZone)) == nil && p.StartMinute >= 0 && p.StartMinute < 1440 && p.EndMinute >= 0 && p.EndMinute < 1440 && (!p.Enabled || p.StartMinute != p.EndMinute)
}
func (a App) ConfiguredUnavailablePeriod(ctx context.Context, authorization string) (UnavailablePeriodPreference, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return UnavailablePeriodPreference{}, err
	}
	if a.UnavailablePeriods == nil || a.Audits == nil {
		return UnavailablePeriodPreference{}, ports.ErrUnavailable
	}
	value, err := a.UnavailablePeriods.GetUnavailablePeriod(ctx, user.ID)
	if err != nil {
		return UnavailablePeriodPreference{}, err
	}
	if !validUnavailablePeriodValue(value) {
		return UnavailablePeriodPreference{}, ports.ErrUnavailable
	}
	if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.ResourceViewed, "unavailable_period", user.ID, audit.Succeeded)); err != nil {
		return UnavailablePeriodPreference{}, err
	}
	return value, nil
}
func (a App) UpdateConfiguredUnavailablePeriod(ctx context.Context, authorization, key string, input UnavailablePeriodUpdate) (UnavailablePeriodResult, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return UnavailablePeriodResult{}, err
	}
	if a.UnavailablePeriods == nil || a.Clock == nil {
		return UnavailablePeriodResult{}, ports.ErrUnavailable
	}
	p := input.Period
	if !validTimeZoneMutationKey(key) || input.ExpectedRevision < 0 || input.ExpectedRevision == math.MaxInt64 || identity.ValidateIANATimeZone(identity.IANATimeZone(input.ReviewedTimeZone)) != nil || p.StartMinute < 0 || p.StartMinute >= 1440 || p.EndMinute < 0 || p.EndMinute >= 1440 {
		return UnavailablePeriodResult{}, ports.ErrInvalidArgument
	}
	// This draft route must not be released before equal-endpoint policy is resolved.
	if p.Enabled && p.StartMinute == p.EndMinute {
		return UnavailablePeriodResult{}, ports.ErrUnavailable
	}
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return UnavailablePeriodResult{}, ports.ErrUnavailable
	}
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%t:%d:%d", input.ExpectedRevision, input.ReviewedTimeZone, p.Enabled, p.StartMinute, p.EndMinute)))
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceUpdated, "unavailable_period", user.ID, audit.Succeeded)
	event.OccurredAt = now
	result, err := a.UnavailablePeriods.UpdateUnavailablePeriod(ctx, UnavailablePeriodCommand{ActorUserID: user.ID, ExpectedRevision: input.ExpectedRevision, ReviewedTimeZone: input.ReviewedTimeZone, Period: p, ChangedAt: now, Idempotency: ports.Idempotency{PrincipalID: user.ID, Operation: UpdateUnavailablePeriodOperation, Key: key, RequestHash: hash[:]}, Audit: event})
	if err != nil {
		return UnavailablePeriodResult{}, err
	}
	if !validUnavailablePeriodValue(result.Preference) || result.Preference.Period != p || result.Preference.Revision != input.ExpectedRevision+1 || result.Preference.TimeZone != input.ReviewedTimeZone {
		return UnavailablePeriodResult{}, ports.ErrUnavailable
	}
	return result, nil
}
