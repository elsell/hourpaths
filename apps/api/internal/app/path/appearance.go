package path

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

type AppearanceCommand struct {
	UserID      string
	PathID      domain.ID
	Appearance  domain.Appearance
	UpdatedAt   time.Time
	Idempotency ports.Idempotency
	Audit       audit.Event
}

type AppearanceRepository interface {
	ReadAppearance(context.Context, string, domain.ID) (domain.Appearance, error)
	SaveAppearance(context.Context, AppearanceCommand) (domain.Appearance, error)
}

func (s *Service) appearanceActor(ctx context.Context, authorization string, id domain.ID) (ports.Principal, time.Time, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, time.Time{}, err
	}
	if id == "" || s.Repository == nil || s.Authorizer == nil || s.Clock == nil || s.Audits == nil || s.AuditRateLimiter == nil {
		return ports.Principal{}, time.Time{}, errInvalidPathDependencies
	}
	now := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return principal, now, platformapp.ErrRateLimited
	}
	allowed, err := s.Authorizer.Check(ctx, "path", string(id), "view", principal.UserID)
	if err != nil {
		return principal, now, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "path_appearance", string(id), audit.Denied)
		event.OccurredAt = now
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return principal, now, err
		}
		return principal, now, ports.ErrNotFound
	}
	return principal, now, nil
}

func (s *Service) ReadAppearance(ctx context.Context, authorization string, id domain.ID) (domain.Appearance, error) {
	principal, now, err := s.appearanceActor(ctx, authorization, id)
	if err != nil {
		return domain.Appearance{}, err
	}
	result, err := s.Repository.ReadAppearance(ctx, principal.UserID, id)
	if err != nil {
		return domain.Appearance{}, err
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "path_appearance", string(id), audit.Succeeded)
	event.OccurredAt = now
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return domain.Appearance{}, err
	}
	return result, nil
}

func (s *Service) SaveAppearance(ctx context.Context, authorization, key string, id domain.ID, value domain.Appearance) (domain.Appearance, error) {
	principal, now, err := s.appearanceActor(ctx, authorization, id)
	if err != nil {
		return domain.Appearance{}, err
	}
	if !validIdempotencyKey(key) || value.Revision < 0 || !value.ValidChoice() {
		return domain.Appearance{}, ports.ErrInvalidArgument
	}
	payload, err := json.Marshal(struct {
		PathID     domain.ID
		Appearance domain.Appearance
	}{id, value})
	if err != nil {
		return domain.Appearance{}, err
	}
	digest := sha256.Sum256(payload)
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "path_appearance", string(id), audit.Succeeded)
	event.OccurredAt = now
	return s.Repository.SaveAppearance(ctx, AppearanceCommand{UserID: principal.UserID, PathID: id, Appearance: value, UpdatedAt: now,
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: "path.appearance.update", Key: key, RequestHash: digest[:]}, Audit: event})
}
