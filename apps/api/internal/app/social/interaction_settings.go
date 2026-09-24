package social

import (
	"context"
	"crypto/sha256"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const UpdateInteractionSettingsOperation = "social.interaction_settings.update"

type InteractionSettings struct {
	CommentsEnabled  bool
	ReactionsEnabled bool
}

type InteractionSettingsCommand struct {
	ActorUserID string
	Settings    InteractionSettings
	OccurredAt  time.Time
	Idempotency ports.Idempotency
	Audit       audit.Event
}

type InteractionSettingsResult struct {
	Settings InteractionSettings
	Replayed bool
}

type InteractionSettingsRepository interface {
	GetInteractionSettings(context.Context, string) (InteractionSettings, error)
	UpdateInteractionSettings(context.Context, InteractionSettingsCommand) (InteractionSettingsResult, error)
}

func (service *Service) GetInteractionSettings(ctx context.Context, authorization string) (InteractionSettings, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return InteractionSettings{}, err
	}
	now, err := service.interactionSettingsReady(principal.UserID, true)
	if err != nil {
		return InteractionSettings{}, err
	}
	settings, err := service.InteractionSettings.GetInteractionSettings(ctx, principal.UserID)
	if err != nil {
		return InteractionSettings{}, err
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "social_interaction_settings", principal.UserID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return InteractionSettings{}, err
	}
	return settings, nil
}

func (service *Service) UpdateInteractionSettings(ctx context.Context, authorization, key string, settings InteractionSettings) (InteractionSettings, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return InteractionSettings{}, err
	}
	if !validRelationshipIdempotencyKey(key) {
		return InteractionSettings{}, ports.ErrInvalidArgument
	}
	now, err := service.interactionSettingsReady(principal.UserID, false)
	if err != nil {
		return InteractionSettings{}, err
	}
	digest := sha256.Sum256([]byte{boolByte(settings.CommentsEnabled), boolByte(settings.ReactionsEnabled)})
	command := InteractionSettingsCommand{ActorUserID: principal.UserID, Settings: settings, OccurredAt: now}
	command.Idempotency = ports.Idempotency{PrincipalID: principal.UserID, Operation: UpdateInteractionSettingsOperation, Key: key, RequestHash: digest[:]}
	command.Audit = shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "social_interaction_settings", principal.UserID, audit.Succeeded)
	command.Audit.OccurredAt = now
	result, err := service.InteractionSettings.UpdateInteractionSettings(ctx, command)
	if err != nil {
		return InteractionSettings{}, err
	}
	if result.Settings != settings {
		return InteractionSettings{}, errInvalidDependencies
	}
	return result.Settings, nil
}

func (service *Service) interactionSettingsReady(principal string, read bool) (time.Time, error) {
	if service.InteractionSettings == nil || service.Audits == nil || service.Clock == nil || (read && service.AuditRateLimiter == nil) {
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

func boolByte(value bool) byte {
	if value {
		return 1
	}
	return 0
}
