package moderation

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

type NoticePage struct {
	Items   []Notice
	HasMore bool
}

func (s *Enforcements) List(ctx context.Context, authorization, cursor string, limit int) ([]Notice, string, error) {
	owner, err := s.principal(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if len(s.CursorSigningKey) < 32 {
		return nil, "", ports.ErrUnavailable
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	p := ports.PageRequest{Limit: limit, Snapshot: s.Clock.Now().UTC().Truncate(time.Microsecond)}
	if cursor != "" {
		c, e := shared.DecodeCursor(s.CursorSigningKey, cursor)
		if e != nil || c.Owner != owner || c.Domain != "moderation_notices" || c.Snapshot.After(p.Snapshot) {
			return nil, "", ports.ErrInvalidArgument
		}
		p.Snapshot = c.Snapshot
		p.AfterCreated = c.AfterCreated
		p.AfterID = c.AfterID
	}
	event := shared.NewAuditEvent(ctx, s.Clock, owner, owner, audit.ResourceListed, "enforcement_notice", "owned", audit.Succeeded)
	page, err := s.Repository.ListNotices(ctx, owner, p, event)
	if err != nil {
		return nil, "", err
	}
	if len(page.Items) > limit || (page.HasMore && len(page.Items) == 0) {
		return nil, "", ports.ErrUnavailable
	}
	previousTime, previousID := p.AfterCreated, p.AfterID
	for _, n := range page.Items {
		e := n.Decision
		if e.Validate() != nil || e.SubjectUserID != owner || e.IssuedAt.After(p.Snapshot) || (n.Appeal != nil && n.Appeal.EnforcementID != e.ID) {
			return nil, "", ports.ErrUnavailable
		}
		if !previousTime.IsZero() && (e.IssuedAt.After(previousTime) || (e.IssuedAt.Equal(previousTime) && e.ID >= previousID)) {
			return nil, "", ports.ErrUnavailable
		}
		previousTime, previousID = e.IssuedAt, e.ID
	}
	next := ""
	if page.HasMore {
		next, err = shared.EncodeCursor(s.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: owner, Domain: "moderation_notices", AfterID: previousID, AfterCreated: previousTime, Snapshot: p.Snapshot})
	}
	return page.Items, next, err
}
