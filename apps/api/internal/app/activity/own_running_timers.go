package activity

import (
	"context"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

type RunningTimerCandidate struct {
	Timer    domain.RunningTimer
	PathName string
}
type RunningTimerPageRequest struct {
	Snapshot, AfterStartedAt time.Time
	AfterID                  string
	Limit                    int
}

func (s *Service) ListOwnRunningTimers(ctx context.Context, authorization, cursor string, limit int) ([]RunningTimerCandidate, string, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if s.Repository == nil || s.Authorizer == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || len(s.CursorSigningKey) < 32 {
		return nil, "", errInvalidDependencies
	}
	if limit < 1 || limit > 100 || len(cursor) > 4096 {
		return nil, "", ports.ErrInvalidArgument
	}
	now := durableInstant(s.Clock.Now())
	if now.IsZero() {
		return nil, "", errInvalidDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	page := RunningTimerPageRequest{Snapshot: now, Limit: limit + 1}
	if cursor != "" {
		decoded, err := shared.DecodeCursor(s.CursorSigningKey, cursor)
		if err != nil || decoded.Owner != principal.UserID || decoded.Domain != "own-running-timers" || decoded.Projection != "" || decoded.Snapshot.After(now) {
			return nil, "", ports.ErrInvalidArgument
		}
		page.Snapshot = decoded.Snapshot
		page.AfterStartedAt = decoded.AfterCreated
		page.AfterID = decoded.AfterID
	}
	candidates, err := s.Repository.ListRunningTimerCandidates(ctx, principal.UserID, page)
	if err != nil {
		return nil, "", err
	}
	if len(candidates) > page.Limit {
		return nil, "", errInvalidDependencies
	}
	previousAt, previousID := page.AfterStartedAt, page.AfterID
	for _, candidate := range candidates {
		timer := candidate.Timer
		if !validTimerFor(timer, principal.UserID, timer.PathID) || strings.TrimSpace(candidate.PathName) == "" || timer.StartedAt.After(page.Snapshot) || timer.StartedAt.Before(previousAt) || timer.StartedAt.Equal(previousAt) && timer.ID <= previousID {
			return nil, "", errInvalidDependencies
		}
		previousAt, previousID = timer.StartedAt, timer.ID
	}
	more := len(candidates) > limit
	if more {
		candidates = candidates[:limit]
	}
	visible := make([]RunningTimerCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		allowed, err := s.Authorizer.Check(ctx, "path", candidate.Timer.PathID, "track", principal.UserID)
		if err != nil {
			return nil, "", err
		}
		if !allowed {
			if err = s.Audits.AppendAuditEvent(ctx, s.auditEvent(ctx, principal.UserID, audit.ResourceAccessDenied, candidate.Timer.PathID)); err != nil {
				return nil, "", err
			}
			continue
		}
		visible = append(visible, candidate)
	}
	next := ""
	if more {
		last := candidates[len(candidates)-1].Timer
		next, err = shared.EncodeCursor(s.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: "own-running-timers", AfterID: last.ID, AfterCreated: last.StartedAt, Snapshot: page.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceListed, principal.UserID)
	event.TargetType = "own_running_timers"
	if err = s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return visible, next, nil
}
