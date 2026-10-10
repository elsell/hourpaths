package activity

import (
	"context"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

type OwnHistoryPageRequest struct {
	ActivityPageRequest
	EndedSince time.Time
}

func (s *Service) ListOwnHistory(ctx context.Context, authorization, window, cursor string, limit int) ([]ActivityListRecord, string, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if s.Repository == nil || s.Authorizer == nil || s.Audits == nil || s.AuditRateLimiter == nil || s.Clock == nil || s.NewID == nil || len(s.CursorSigningKey) < 32 {
		return nil, "", errInvalidDependencies
	}
	if (window != "all" && window != "retained") || limit < 1 || limit > 100 || len(cursor) > 4096 {
		return nil, "", ports.ErrInvalidArgument
	}
	now := durableInstant(s.Clock.Now())
	if now.IsZero() {
		return nil, "", errInvalidDependencies
	}
	if !s.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	domainName := "own-activity-history:" + window
	page := OwnHistoryPageRequest{ActivityPageRequest: ActivityPageRequest{ParticipantID: principal.UserID, Snapshot: now, Limit: limit}}
	if cursor != "" {
		c, e := shared.DecodeCursor(s.CursorSigningKey, cursor)
		if e != nil || c.Owner != principal.UserID || c.Domain != domainName || c.Projection != "" || c.Snapshot.After(now) {
			return nil, "", ports.ErrInvalidArgument
		}
		page.Snapshot = c.Snapshot
		page.AfterStartedAt = c.AfterCreated
		page.AfterID = c.AfterID
	}
	if window == "retained" {
		page.EndedSince = page.Snapshot.Add(-90 * 24 * time.Hour)
	}
	result, err := s.Repository.ListOwnActivityCandidates(ctx, principal.UserID, page)
	if err != nil {
		return nil, "", err
	}
	if len(result.Items) > limit || (result.HasMore && len(result.Items) == 0) {
		return nil, "", errInvalidDependencies
	}
	previousAt, previousID := page.AfterStartedAt, page.AfterID
	for _, record := range result.Items {
		entry := record.Activity
		if entry.ParticipantID != principal.UserID || !validActivityView(entry, record.Version, principal.UserID, entry.PathID, entry.ID) || strings.TrimSpace(record.PathName) == "" || entry.CreatedAt.After(page.Snapshot) || entry.StartedAt.After(page.Snapshot) || (!page.EndedSince.IsZero() && entry.EndedAt.Before(page.EndedSince)) || (!previousAt.IsZero() && (entry.StartedAt.After(previousAt) || entry.StartedAt.Equal(previousAt) && entry.ID >= previousID)) {
			return nil, "", errInvalidDependencies
		}
		previousAt, previousID = entry.StartedAt, entry.ID
	}
	visible := make([]ActivityListRecord, 0, len(result.Items))
	decisions := map[string]bool{}
	for _, record := range result.Items {
		allowed, known := decisions[record.Activity.PathID]
		if !known {
			allowed, err = s.Authorizer.Check(ctx, "path", record.Activity.PathID, "view", principal.UserID)
			if err != nil {
				return nil, "", err
			}
			decisions[record.Activity.PathID] = allowed
			if !allowed {
				if err = s.Audits.AppendAuditEvent(ctx, s.auditEvent(ctx, principal.UserID, audit.ResourceAccessDenied, record.Activity.PathID)); err != nil {
					return nil, "", err
				}
			}
		}
		if allowed {
			visible = append(visible, record)
		}
	}
	next := ""
	if result.HasMore {
		last := result.Items[len(result.Items)-1].Activity
		next, err = shared.EncodeCursor(s.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: domainName, AfterID: last.ID, AfterCreated: last.StartedAt, Snapshot: page.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	event := s.auditEvent(ctx, principal.UserID, audit.ResourceListed, principal.UserID)
	event.TargetType = "own_activity_history"
	if err = s.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	return visible, next, nil
}
