package moderation

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	app "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type Notice struct {
	Decision domain.Enforcement
	Appeal   *domain.Appeal
}

type AppealCommand struct {
	OwnerID string
	Appeal  domain.Appeal
	Audit   audit.Event
}

type EnforcementRepository interface {
	ListNotices(context.Context, string, ports.PageRequest, audit.Event) (NoticePage, error)
	ReadNotice(context.Context, string, string, audit.Event) (Notice, error)
	// SubmitAppeal must atomically recheck ownership and the deadline, enforce one
	// immutable appeal per decision, and commit its audit with the submission.
	SubmitAppeal(context.Context, AppealCommand) (domain.Appeal, error)
}

type EnforcementDependencies struct {
	CursorSigningKey []byte
	Auth             ports.Authenticator
	Repository       EnforcementRepository
	Audits           ports.Audits
	RateLimiter      ports.AuditRateLimiter
	Clock            ports.Clock
}
type Enforcements struct{ EnforcementDependencies }

func NewEnforcements(d EnforcementDependencies) *Enforcements { return &Enforcements{d} }

func (s *Enforcements) principal(ctx context.Context, authorization string) (string, error) {
	if s == nil || s.Auth == nil || s.Repository == nil || s.Audits == nil || s.RateLimiter == nil || s.Clock == nil {
		return "", ports.ErrUnavailable
	}
	p, err := s.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return "", err
	}
	if p.UserID == "" || len(p.Scopes) != 1 || p.Scopes[0] != "api:user" {
		return "", app.ErrUnauthenticated
	}
	if s.Clock.Now().IsZero() {
		return "", ports.ErrUnavailable
	}
	if !s.RateLimiter.Allow(p.UserID, s.Clock.Now()) {
		return "", app.ErrRateLimited
	}
	return p.UserID, nil
}

func (s *Enforcements) read(ctx context.Context, owner, id string) (Notice, error) {
	if id == "" || len(id) > 200 || strings.TrimSpace(id) != id {
		return Notice{}, ports.ErrInvalidArgument
	}
	event := shared.NewAuditEvent(ctx, s.Clock, owner, owner, audit.ResourceViewed, "enforcement_notice", id, audit.Succeeded)
	n, err := s.Repository.ReadNotice(ctx, owner, id, event)
	if errors.Is(err, ports.ErrNotFound) {
		denial := shared.NewAuditEvent(ctx, s.Clock, owner, owner, audit.ResourceAccessDenied, "enforcement_notice", "hidden", audit.Denied)
		if e := s.Audits.AppendAuditEvent(ctx, denial); e != nil {
			return Notice{}, e
		}
	}
	if err != nil {
		return Notice{}, err
	}
	if n.Decision.ID != id || n.Decision.SubjectUserID != owner || n.Decision.Validate() != nil {
		return Notice{}, ports.ErrUnavailable
	}
	if n.Appeal != nil && n.Appeal.EnforcementID != id {
		return Notice{}, ports.ErrUnavailable
	}
	return n, nil
}

func (s *Enforcements) Get(ctx context.Context, authorization, id string) (Notice, error) {
	owner, err := s.principal(ctx, authorization)
	if err != nil {
		return Notice{}, err
	}
	return s.read(ctx, owner, id)
}

func (s *Enforcements) Appeal(ctx context.Context, authorization, id, key, explanation string) (domain.Appeal, error) {
	owner, err := s.principal(ctx, authorization)
	if err != nil {
		return domain.Appeal{}, err
	}
	if !validKey(key) || !utf8.ValidString(explanation) {
		return domain.Appeal{}, ports.ErrInvalidArgument
	}
	explanation = strings.TrimSpace(explanation)
	n, err := s.read(ctx, owner, id)
	if err != nil {
		return domain.Appeal{}, err
	}
	// A lost-response retry remains a read of the accepted immutable submission,
	// including after the submission window has closed or review has completed.
	if n.Appeal != nil {
		if n.Appeal.ID == key && n.Appeal.Explanation == explanation {
			return *n.Appeal, nil
		}
		return domain.Appeal{}, ports.ErrConflict
	}
	proposed, err := n.Decision.NewAppeal(key, explanation, s.Clock.Now().UTC())
	if err != nil {
		return domain.Appeal{}, ports.ErrConflict
	}
	event := shared.NewAuditEvent(ctx, s.Clock, owner, owner, audit.ResourceCreated, "moderation_appeal", id, audit.Succeeded)
	// The audit and immutable submission represent the same event instant.
	event.OccurredAt = proposed.SubmittedAt
	result, err := s.Repository.SubmitAppeal(ctx, AppealCommand{OwnerID: owner, Appeal: proposed, Audit: event})
	if err != nil {
		return domain.Appeal{}, err
	}
	if result.ID != key || result.EnforcementID != id || result.Explanation != explanation {
		return domain.Appeal{}, ports.ErrUnavailable
	}
	return result, nil
}
