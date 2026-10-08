package social

import (
	"context"
	"crypto/sha256"
	"strconv"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/notification"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type NotificationChannelPreference struct {
	Channel notification.Channel
	NudgeNotificationChannelPreference
}
type NotificationChannelCommand struct {
	Channel notification.Channel
	NudgeNotificationChannelCommand
}
type NotificationChannelRepository interface {
	GetNotificationChannel(context.Context, string, notification.Channel) (NudgeNotificationChannelPreference, bool, error)
	UpdateNotificationChannel(context.Context, NotificationChannelCommand) (NudgeNotificationChannelMutationResult, error)
}

func (service *Service) ListNotificationChannels(ctx context.Context, authorization string) ([]NotificationChannelPreference, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, err
	}
	if service.NotificationChannels == nil || service.Clock == nil || service.Audits == nil || service.AuditRateLimiter == nil {
		return nil, errInvalidDependencies
	}
	now := service.Clock.Now().UTC()
	if now.IsZero() {
		return nil, errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, platformapp.ErrRateLimited
	}
	result := make([]NotificationChannelPreference, 0, 10)
	for _, channel := range notification.Channels() {
		preference, found, err := service.NotificationChannels.GetNotificationChannel(ctx, principal.UserID, channel)
		if err != nil {
			return nil, err
		}
		if !preference.valid(found) {
			return nil, errInvalidDependencies
		}
		if !found {
			preference.Enabled = true
		}
		result = append(result, NotificationChannelPreference{Channel: channel, NudgeNotificationChannelPreference: preference})
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "notification_channel", "all", audit.Succeeded)
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, err
	}
	return result, nil
}

func (service *Service) UpdateNotificationChannel(ctx context.Context, authorization, rawChannel, key string, expectedRevision int64, enabled bool) (NotificationChannelPreference, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return NotificationChannelPreference{}, err
	}
	channel := notification.Channel(rawChannel)
	if !channel.Valid() || !validRelationshipIdempotencyKey(key) || expectedRevision < 0 {
		return NotificationChannelPreference{}, ports.ErrInvalidArgument
	}
	if service.NotificationChannels == nil || service.Clock == nil || service.Audits == nil || service.AuditRateLimiter == nil {
		return NotificationChannelPreference{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return NotificationChannelPreference{}, errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return NotificationChannelPreference{}, platformapp.ErrRateLimited
	}
	digest := sha256.Sum256([]byte(UpdateNudgeNotificationChannelOperation + "\x00" + rawChannel + "\x00" + strconv.FormatInt(expectedRevision, 10) + "\x00" + strconv.FormatBool(enabled)))
	command := NotificationChannelCommand{Channel: channel, NudgeNotificationChannelCommand: NudgeNotificationChannelCommand{
		ActorUserID: principal.UserID, Enabled: enabled, ExpectedRevision: expectedRevision, OccurredAt: now,
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: UpdateNudgeNotificationChannelOperation, Key: key, RequestHash: digest[:]},
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "notification_channel", rawChannel, audit.Succeeded),
	}}
	command.Audit.OccurredAt = now
	result, err := service.NotificationChannels.UpdateNotificationChannel(ctx, command)
	if err != nil {
		return NotificationChannelPreference{}, err
	}
	if !result.Preference.valid(true) || result.Preference.Enabled != enabled || result.Preference.Revision != expectedRevision+1 {
		return NotificationChannelPreference{}, errInvalidDependencies
	}
	return NotificationChannelPreference{Channel: channel, NudgeNotificationChannelPreference: result.Preference}, nil
}
