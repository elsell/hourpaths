package social

import (
	"context"
	"crypto/sha256"
	"strconv"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	NudgeNotificationChannel                = "nudges"
	UpdateNudgeNotificationChannelOperation = "notification.channel_preference.update"
)

type NudgeNotificationChannelPreference struct {
	Enabled  bool
	Revision int64
}

func (preference NudgeNotificationChannelPreference) valid(found bool) bool {
	if preference.Revision < 0 {
		return false
	}
	if !found {
		return preference == (NudgeNotificationChannelPreference{})
	}
	return preference.Revision > 0
}

type NudgeNotificationChannelCommand struct {
	ActorUserID      string
	Enabled          bool
	ExpectedRevision int64
	OccurredAt       time.Time
	Idempotency      ports.Idempotency
	Audit            audit.Event
}

type NudgeNotificationChannelMutationResult struct {
	Preference NudgeNotificationChannelPreference
	Replayed   bool
}

type NudgeNotificationChannelRepository interface {
	GetNudgeNotificationChannel(context.Context, string) (NudgeNotificationChannelPreference, bool, error)
	UpdateNudgeNotificationChannel(context.Context, NudgeNotificationChannelCommand) (NudgeNotificationChannelMutationResult, error)
}

func (service *Service) GetNudgeNotificationChannel(ctx context.Context, authorization string) (NudgeNotificationChannelPreference, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return NudgeNotificationChannelPreference{}, err
	}
	now, err := service.nudgeNotificationChannelReady(principal.UserID, true)
	if err != nil {
		return NudgeNotificationChannelPreference{}, err
	}
	preference, found, err := service.NudgeNotificationChannels.GetNudgeNotificationChannel(ctx, principal.UserID)
	if err != nil {
		return NudgeNotificationChannelPreference{}, err
	}
	if !preference.valid(found) {
		return NudgeNotificationChannelPreference{}, errInvalidDependencies
	}
	if !found {
		preference.Enabled = true
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "notification_channel", NudgeNotificationChannel, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return NudgeNotificationChannelPreference{}, err
	}
	return preference, nil
}

func (service *Service) UpdateNudgeNotificationChannel(ctx context.Context, authorization, key string, expectedRevision int64, enabled bool) (NudgeNotificationChannelPreference, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return NudgeNotificationChannelPreference{}, err
	}
	if !validRelationshipIdempotencyKey(key) || expectedRevision < 0 {
		return NudgeNotificationChannelPreference{}, ports.ErrInvalidArgument
	}
	now, err := service.nudgeNotificationChannelReady(principal.UserID, false)
	if err != nil {
		return NudgeNotificationChannelPreference{}, err
	}
	command := NudgeNotificationChannelCommand{
		ActorUserID: principal.UserID, Enabled: enabled, ExpectedRevision: expectedRevision, OccurredAt: now,
		Idempotency: nudgeNotificationChannelIdempotency(principal.UserID, key, expectedRevision, enabled),
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "notification_channel", NudgeNotificationChannel, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	result, err := service.NudgeNotificationChannels.UpdateNudgeNotificationChannel(ctx, command)
	if err != nil {
		return NudgeNotificationChannelPreference{}, err
	}
	if !result.Preference.valid(true) || result.Preference.Enabled != enabled || result.Preference.Revision != expectedRevision+1 {
		return NudgeNotificationChannelPreference{}, errInvalidDependencies
	}
	return result.Preference, nil
}

func (service *Service) nudgeNotificationChannelReady(principal string, read bool) (time.Time, error) {
	if service.NudgeNotificationChannels == nil || service.Audits == nil || service.Clock == nil || (read && service.AuditRateLimiter == nil) {
		return time.Time{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return time.Time{}, errInvalidDependencies
	}
	if read && !service.AuditRateLimiter.Allow(principal, now) {
		return time.Time{}, platformapp.ErrRateLimited
	}
	return now, nil
}

func nudgeNotificationChannelIdempotency(actor, key string, expectedRevision int64, enabled bool) ports.Idempotency {
	digest := sha256.Sum256([]byte(UpdateNudgeNotificationChannelOperation + "\x00" + NudgeNotificationChannel + "\x00" + strconv.FormatInt(expectedRevision, 10) + "\x00" + strconv.FormatBool(enabled)))
	return ports.Idempotency{PrincipalID: actor, Operation: UpdateNudgeNotificationChannelOperation, Key: key, RequestHash: digest[:]}
}
