package moderation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"

	app "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const SubmitOperation = "moderation.report.submit"

type BlockIdentity struct{ UserID, Username, DisplayName string }
type Receipt struct {
	ID          string
	BlockTarget *BlockIdentity
}

type TargetAccess struct {
	Target        domain.Target
	SubjectUserID string
	PathID        string
}

// ReportCommand contains no caller-supplied actor or evidence. Persistence must
// re-resolve current visibility and capture the target inside the case/audit
// transaction, preserving a retry's original immutable receipt.
type ReportCommand struct {
	ID, ReporterID string
	Access         TargetAccess
	Reason         domain.Reason
	Explanation    string
	At             time.Time
	Idempotency    ports.Idempotency
	Audit          audit.Event
}

type Repository interface {
	FindReportReplay(context.Context, ports.Idempotency) (Receipt, bool, error)
	ResolveReportTarget(context.Context, string, domain.Target, time.Time) (TargetAccess, error)
	SubmitReport(context.Context, ReportCommand) (Receipt, error)
}

type Dependencies struct {
	Auth        ports.Authenticator
	Repository  Repository
	Authorizer  ports.Authorizer
	Audits      ports.Audits
	RateLimiter ports.AuditRateLimiter
	Clock       ports.Clock
	NewID       func() string
}

type Service struct{ Dependencies }

func New(d Dependencies) *Service { return &Service{Dependencies: d} }

func (s *Service) Submit(ctx context.Context, authorization string, target domain.Target, reason domain.Reason, explanation, key string) (Receipt, error) {
	if s == nil || s.Auth == nil || s.Repository == nil || s.Authorizer == nil || s.Audits == nil || s.RateLimiter == nil || s.Clock == nil || s.NewID == nil {
		return Receipt{}, ports.ErrUnavailable
	}
	principal, err := s.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return Receipt{}, err
	}
	if principal.UserID == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:user" {
		return Receipt{}, app.ErrUnauthenticated
	}
	at := s.Clock.Now().UTC().Truncate(time.Microsecond)
	if at.IsZero() {
		return Receipt{}, ports.ErrUnavailable
	}
	if !s.RateLimiter.Allow(principal.UserID, at) {
		return Receipt{}, app.ErrRateLimited
	}
	explanation, err = domain.NormalizeExplanation(explanation)
	if err != nil || !target.Valid() || !reason.Valid() || !validKey(key) {
		return Receipt{}, ports.ErrInvalidArgument
	}
	body, _ := json.Marshal(struct {
		Target      domain.Target
		Reason      domain.Reason
		Explanation string
	}{target, reason, explanation})
	hash := sha256.Sum256(body)
	operation := ports.Idempotency{PrincipalID: principal.UserID, Operation: SubmitOperation, Key: key, RequestHash: hash[:]}
	receipt, found, err := s.Repository.FindReportReplay(ctx, operation)
	if err != nil {
		return Receipt{}, err
	}
	if found {
		if receipt.ID == "" {
			return Receipt{}, ports.ErrUnavailable
		}
		event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "report_receipt", receipt.ID, audit.Succeeded)
		if err = s.Audits.AppendAuditEvent(ctx, event); err != nil {
			return Receipt{}, err
		}
		return receipt, nil
	}
	access, err := s.Repository.ResolveReportTarget(ctx, principal.UserID, target, at)
	if err != nil {
		return Receipt{}, s.reportError(ctx, principal.UserID, err)
	}
	if access.Target != target || access.SubjectUserID == "" || (target.Kind != domain.Profile && access.PathID == "") || (target.Kind == domain.Profile && access.PathID != "") {
		return Receipt{}, ports.ErrUnavailable
	}
	// Profiles use the existing public-profile visibility rule (including blocks).
	// Every content target also requires its containing Path's SpiceDB permission.
	if access.PathID != "" {
		allowed, err := s.Authorizer.Check(ctx, "path", access.PathID, "view", principal.UserID)
		if err != nil {
			return Receipt{}, err
		}
		if !allowed {
			return Receipt{}, s.reportError(ctx, principal.UserID, ports.ErrNotFound)
		}
	}
	id := s.NewID()
	if id == "" {
		return Receipt{}, ports.ErrUnavailable
	}
	event := shared.NewAuditEvent(ctx, s.Clock, principal.UserID, principal.UserID, audit.ResourceCreated, "moderation_report", id, audit.Succeeded)
	event.OccurredAt = at
	receipt, err = s.Repository.SubmitReport(ctx, ReportCommand{ID: id, ReporterID: principal.UserID, Access: access, Reason: reason, Explanation: explanation, At: at, Idempotency: operation, Audit: event})
	if err != nil {
		return Receipt{}, s.reportError(ctx, principal.UserID, err)
	}
	if receipt.ID == "" {
		return Receipt{}, ports.ErrUnavailable
	}
	return receipt, nil
}

func (s *Service) reportError(ctx context.Context, actor string, err error) error {
	if !errors.Is(err, ports.ErrNotFound) {
		return err
	}
	event := shared.NewAuditEvent(ctx, s.Clock, actor, actor, audit.ResourceAccessDenied, "report_target", "hidden", audit.Denied)
	if auditErr := s.Audits.AppendAuditEvent(ctx, event); auditErr != nil {
		return auditErr
	}
	return ports.ErrNotFound
}

func validKey(key string) bool {
	if len(key) < 16 || len(key) > 128 || strings.TrimSpace(key) != key {
		return false
	}
	for i := range len(key) {
		if key[i] < 0x20 || key[i] > 0x7e {
			return false
		}
	}
	return true
}
