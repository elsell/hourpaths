package stats

import (
	"context"
	"errors"
	platform "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/stats"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

type Snapshot struct {
	Paths          []domain.StatsPath
	Records        []domain.Record
	TimeZone       string
	FirstDayOfWeek int
}
type Repository interface {
	Read(context.Context, string) (Snapshot, error)
}
type Service struct {
	Auth             ports.Authenticator
	Repository       Repository
	Authorizer       ports.Authorizer
	Audits           ports.Audits
	AuditRateLimiter ports.AuditRateLimiter
	Clock            ports.Clock
}

func (s *Service) Get(ctx context.Context, authorization, kind, anchor, pathIDs string) (domain.StatsSummary, error) {
	empty := domain.StatsSummary{}
	if s.Auth == nil {
		return empty, errors.New("stats authentication unavailable")
	}
	p, err := s.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return empty, err
	}
	if p.UserID == "" || len(p.Scopes) != 1 || p.Scopes[0] != "api:user" {
		return empty, platform.ErrUnauthenticated
	}
	if s.Repository == nil || s.Authorizer == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil {
		return empty, errors.New("stats dependencies unavailable")
	}
	if !s.AuditRateLimiter.Allow(p.UserID, s.Clock.Now()) {
		return empty, platform.ErrRateLimited
	}
	snapshot, err := s.Repository.Read(ctx, p.UserID)
	if err != nil {
		return empty, err
	}
	selected := map[string]bool{}
	if pathIDs != "" {
		for _, id := range strings.Split(pathIDs, ",") {
			if id == "" {
				return empty, ports.ErrInvalidArgument
			}
			selected[id] = true
		}
	}
	visible := []domain.StatsPath{}
	allowedIDs := map[string]bool{}
	for _, path := range snapshot.Paths {
		allowed, e := s.Authorizer.Check(ctx, "path", path.ID, "view", p.UserID)
		if e != nil {
			return empty, e
		}
		if !allowed {
			event := shared.NewAuditEvent(ctx, s.Clock, p.UserID, p.UserID, audit.ResourceAccessDenied, "stats", "stats", audit.Denied)
			if e = s.Audits.AppendAuditEvent(ctx, event); e != nil {
				return empty, e
			}
			return empty, platform.ErrForbidden
		}
		visible = append(visible, path)
		allowedIDs[path.ID] = true
	}
	for id := range selected {
		if !allowedIDs[id] {
			event := shared.NewAuditEvent(ctx, s.Clock, p.UserID, p.UserID, audit.ResourceAccessDenied, "stats", "stats", audit.Denied)
			if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
				return empty, err
			}
			return empty, ports.ErrNotFound
		}
	}
	records := []domain.Record{}
	for _, r := range snapshot.Records {
		if allowedIDs[r.PathID] && (len(selected) == 0 || selected[r.PathID]) {
			records = append(records, r)
		}
	}
	loc, err := time.LoadLocation(snapshot.TimeZone)
	if err != nil {
		return empty, err
	}
	if kind == "" {
		kind = "week"
	}
	result, err := domain.Aggregate(kind, anchor, visible, records, s.Clock.Now().In(loc), snapshot.FirstDayOfWeek)
	if err != nil {
		return empty, ports.ErrInvalidArgument
	}
	event := shared.NewAuditEvent(ctx, s.Clock, p.UserID, p.UserID, audit.ResourceListed, "stats", "stats", audit.Succeeded)
	if err = s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return empty, err
	}
	return result, nil
}
