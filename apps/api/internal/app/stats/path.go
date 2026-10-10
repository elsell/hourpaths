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

func (s *Service) GetPath(ctx context.Context, authorization, pathID, participantID string) (domain.PathSummary, error) {
	empty := domain.PathSummary{}
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
	if s.PathRepository == nil || s.Authorizer == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil {
		return empty, errors.New("path statistics dependencies unavailable")
	}
	if !s.AuditRateLimiter.Allow(p.UserID, s.Clock.Now()) {
		return empty, platform.ErrRateLimited
	}
	if pathID == "" || strings.TrimSpace(pathID) != pathID || strings.TrimSpace(participantID) != participantID {
		return empty, ports.ErrInvalidArgument
	}
	if participantID == "" {
		participantID = p.UserID
	}
	allowed, err := s.Authorizer.Check(ctx, "path", pathID, "view", p.UserID)
	if err != nil {
		return empty, err
	}
	if !allowed {
		event := shared.NewAuditEvent(ctx, s.Clock, p.UserID, p.UserID, audit.ResourceAccessDenied, "path_statistics", pathID, audit.Denied)
		if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return empty, err
		}
		return empty, ports.ErrNotFound
	}
	snapshot, err := s.PathRepository.ReadPath(ctx, p.UserID, pathID, participantID)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			event := shared.NewAuditEvent(ctx, s.Clock, p.UserID, p.UserID, audit.ResourceAccessDenied, "path_statistics", pathID, audit.Denied)
			if auditErr := s.Audits.AppendAuditEvent(ctx, event); auditErr != nil {
				return empty, auditErr
			}
		}
		return empty, err
	}
	for _, record := range snapshot.Records {
		if record.PathID != pathID {
			return empty, errors.New("invalid path statistics scope")
		}
	}
	loc, err := time.LoadLocation(snapshot.TimeZone)
	if err != nil {
		return empty, err
	}
	result, err := domain.AggregatePath(snapshot.Records, s.Clock.Now().In(loc), snapshot.FirstDayOfWeek)
	if err != nil {
		return empty, err
	}
	event := shared.NewAuditEvent(ctx, s.Clock, p.UserID, p.UserID, audit.ResourceViewed, "path_statistics", pathID, audit.Succeeded)
	if err := s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return empty, err
	}
	return result, nil
}
