package path

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type HomeClassification string

const (
	HomeSolo       HomeClassification = "solo"
	HomeShared     HomeClassification = "shared"
	HomeSupporting HomeClassification = "supporting"
)

type HomeOrganization struct {
	PathID           domain.ID
	Classification   HomeClassification
	PinnedPosition   *int64
	ManualPosition   *int64
	RecentActivityAt *time.Time
}

type HomeOrganizationProjection struct {
	Preferences  domain.HomePreferences
	Organization map[domain.ID]HomeOrganization
}

type UpdateHomePreferencesCommand struct {
	ActorUserID      string
	ExpectedRevision int64
	OrderMethod      domain.HomeOrderMethod
	PinnedPathIDs    []domain.ID
	ManualPathIDs    []domain.ID
	UpdatedAt        time.Time
	Idempotency      ports.Idempotency
	Audit            audit.Event
}

type HomePreferencesRepository interface {
	ProjectHome(context.Context, string, []domain.ID) (HomeOrganizationProjection, error)
	UpdateHomePreferences(context.Context, UpdateHomePreferencesCommand) (domain.HomePreferences, error)
}

const UpdateHomePreferencesOperation = "home.preferences.update"

func (s *Service) ReadHomePreferences(ctx context.Context, authorization string) (domain.HomePreferences, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return domain.HomePreferences{}, err
	}
	if s.Repository == nil {
		return domain.HomePreferences{}, errInvalidPathDependencies
	}
	projection, err := s.Repository.ProjectHome(ctx, principal.UserID, nil)
	if err != nil {
		return domain.HomePreferences{}, err
	}
	if !projection.Preferences.Valid() {
		return domain.HomePreferences{}, errInvalidPathDependencies
	}
	return projection.Preferences, nil
}

func (s *Service) UpdateHomePreferences(ctx context.Context, authorization, idempotencyKey string, expectedRevision int64, orderMethod domain.HomeOrderMethod, pinnedPathIDs, manualPathIDs []domain.ID) (domain.HomePreferences, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return domain.HomePreferences{}, err
	}
	if s.Repository == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || !validIdempotencyKey(idempotencyKey) || expectedRevision < 0 || !orderMethod.Valid() {
		return domain.HomePreferences{}, ports.ErrInvalidArgument
	}
	requested := domain.HomePreferences{OrderMethod: orderMethod, Revision: expectedRevision, PinnedPathIDs: append([]domain.ID(nil), pinnedPathIDs...), ManualPathIDs: append([]domain.ID(nil), manualPathIDs...)}
	if !validHomePreferenceRequest(requested) {
		return domain.HomePreferences{}, ports.ErrInvalidArgument
	}
	now := s.Clock.Now().UTC()
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return domain.HomePreferences{}, platformapp.ErrRateLimited
	}
	payload, err := json.Marshal(struct {
		ExpectedRevision int64                  `json:"expectedRevision"`
		OrderMethod      domain.HomeOrderMethod `json:"orderMethod"`
		PinnedPathIDs    []domain.ID            `json:"pinnedPathIds"`
		ManualPathIDs    []domain.ID            `json:"manualPathIds"`
	}{expectedRevision, orderMethod, pinnedPathIDs, manualPathIDs})
	if err != nil {
		return domain.HomePreferences{}, errInvalidPathDependencies
	}
	digest := sha256.Sum256(payload)
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "home_preferences", principal.UserID, audit.Succeeded)
	event.OccurredAt = now
	command := UpdateHomePreferencesCommand{
		ActorUserID: principal.UserID, ExpectedRevision: expectedRevision, OrderMethod: orderMethod,
		PinnedPathIDs: append([]domain.ID(nil), pinnedPathIDs...), ManualPathIDs: append([]domain.ID(nil), manualPathIDs...), UpdatedAt: now,
		Idempotency: ports.Idempotency{PrincipalID: principal.UserID, Operation: UpdateHomePreferencesOperation, Key: strings.TrimSpace(idempotencyKey), RequestHash: digest[:]},
		Audit:       event,
	}
	result, err := s.Repository.UpdateHomePreferences(ctx, command)
	if err != nil {
		return domain.HomePreferences{}, err
	}
	if !result.Valid() || result.Revision < 1 {
		return domain.HomePreferences{}, errInvalidPathDependencies
	}
	return result, nil
}

func validHomePreferenceRequest(value domain.HomePreferences) bool {
	value.UpdatedAt = time.Now()
	return value.Valid()
}
