package app

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type PushInstallationRepository interface {
	UpsertPushInstallation(context.Context, ports.PushInstallation, audit.Event) error
	DeletePushInstallation(context.Context, string, string, time.Time, audit.Event) error
}

func (a App) UpsertPushInstallation(
	ctx context.Context,
	authorization, installationID, provider, platform, locale, token string,
) error {
	principal, err := a.pushPrincipal(ctx, authorization)
	if err != nil {
		return err
	}
	if a.PushInstallations == nil || a.Clock == nil || a.AuditRateLimiter == nil ||
		!validPushInstallationID(installationID) ||
		provider != "expo" ||
		(platform != "ios" && platform != "android") ||
		(locale != "en" && locale != "es") ||
		!validExpoPushToken(token) {
		return ports.ErrInvalidArgument
	}
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return ports.ErrInvalidArgument
	}
	if !a.AuditRateLimiter.Allow(principal.UserID, now) {
		return ErrRateLimited
	}
	event := a.auditEvent(
		ctx, principal.UserID, principal.UserID,
		audit.ResourceUpdated, "push_installation", installationID, audit.Succeeded,
	)
	event.OccurredAt = now
	err = a.PushInstallations.UpsertPushInstallation(ctx, ports.PushInstallation{
		ID: installationID, OwnerUserID: principal.UserID,
		Provider: provider, Platform: platform, Locale: locale, Token: token,
		CreatedAt: now, UpdatedAt: now,
	}, event)
	if errors.Is(err, ports.ErrNotFound) {
		return a.auditPushInstallationDenial(ctx, principal.UserID, installationID, now)
	}
	return err
}

func (a App) DeletePushInstallation(
	ctx context.Context,
	authorization, installationID string,
) error {
	principal, err := a.pushPrincipal(ctx, authorization)
	if err != nil {
		return err
	}
	if a.PushInstallations == nil || a.Clock == nil || a.AuditRateLimiter == nil ||
		!validPushInstallationID(installationID) {
		return ports.ErrInvalidArgument
	}
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return ports.ErrInvalidArgument
	}
	if !a.AuditRateLimiter.Allow(principal.UserID, now) {
		return ErrRateLimited
	}
	event := a.auditEvent(
		ctx, principal.UserID, principal.UserID,
		audit.ResourceDeleted, "push_installation", installationID, audit.Succeeded,
	)
	event.OccurredAt = now
	err = a.PushInstallations.DeletePushInstallation(
		ctx, principal.UserID, installationID, now, event,
	)
	if errors.Is(err, ports.ErrNotFound) {
		return a.auditPushInstallationDenial(ctx, principal.UserID, installationID, now)
	}
	return err
}

func (a App) pushPrincipal(ctx context.Context, authorization string) (ports.Principal, error) {
	if a.Auth == nil {
		return ports.Principal{}, ErrUnauthenticated
	}
	principal, err := a.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, err
	}
	if principal.UserID == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:user" {
		return ports.Principal{}, ErrUnauthenticated
	}
	return principal, nil
}

func (a App) auditPushInstallationDenial(
	ctx context.Context,
	actorUserID, installationID string,
	occurredAt time.Time,
) error {
	if a.Audits == nil {
		return ports.ErrNotFound
	}
	event := a.auditEvent(
		ctx, actorUserID, actorUserID,
		audit.ResourceAccessDenied, "push_installation", installationID, audit.Denied,
	)
	event.OccurredAt = occurredAt
	if err := a.Audits.AppendAuditEvent(ctx, event); err != nil {
		return err
	}
	return ports.ErrNotFound
}

func validPushInstallationID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validExpoPushToken(value string) bool {
	if !utf8.ValidString(value) || strings.TrimSpace(value) != value || len(value) > 2048 {
		return false
	}
	for _, prefix := range []string{"ExpoPushToken[", "ExponentPushToken["} {
		if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, "]") &&
			len(value) > len(prefix)+1 {
			inside := value[len(prefix) : len(value)-1]
			return !strings.ContainsAny(inside, "[]\r\n\t ")
		}
	}
	return false
}
