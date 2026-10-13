package moderation

import (
	"context"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

// AccountEnforcementCommand belongs to restricted operational tooling, never an
// ordinary application-user endpoint. NoticeID is the immutable retry identity.
type AccountEnforcementCommand struct {
	CaseID, NoticeID, PolicyReason string
	Action                         domain.EnforcementAction
	Until                          time.Time
}

func (c AccountEnforcementCommand) Validate() error {
	if strings.TrimSpace(c.CaseID) != c.CaseID || c.CaseID == "" || len(c.CaseID) > 200 || strings.TrimSpace(c.NoticeID) != c.NoticeID || c.NoticeID == "" || len(c.NoticeID) > 200 || strings.TrimSpace(c.PolicyReason) != c.PolicyReason || c.PolicyReason == "" || len([]rune(c.PolicyReason)) > 1000 {
		return ports.ErrInvalidArgument
	}
	if c.Action != domain.Suspension && c.Action != domain.Ban || c.Action == domain.Ban && !c.Until.IsZero() || c.Action == domain.Suspension && c.Until.IsZero() {
		return ports.ErrInvalidArgument
	}
	return nil
}

type AccountEnforcementPort interface {
	RestrictAccount(context.Context, AccountEnforcementCommand) (domain.Enforcement, error)
}
