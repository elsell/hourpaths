package moderation

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidEnforcement = errors.New("invalid enforcement")
	ErrAppealUnavailable  = errors.New("appeal unavailable")
	ErrAppealFinal        = errors.New("appeal decision is final")
)

type EnforcementAction string

const (
	ContentRemoval EnforcementAction = "content_removal"
	Warning        EnforcementAction = "warning"
	Suspension     EnforcementAction = "suspension"
	Ban            EnforcementAction = "ban"
)

// Enforcement is the subject-facing decision. Restricted case evidence and the
// reporter's identity deliberately have no representation in this value.
type Enforcement struct {
	ID, SubjectUserID string
	Action            EnforcementAction
	PolicyReason      string
	IssuedAt, Until   time.Time
}

func (e Enforcement) Validate() error {
	if strings.TrimSpace(e.ID) == "" || strings.TrimSpace(e.SubjectUserID) == "" || !meaningful(e.PolicyReason) || e.IssuedAt.IsZero() {
		return ErrInvalidEnforcement
	}
	switch e.Action {
	case Suspension:
		if !e.Until.After(e.IssuedAt) {
			return ErrInvalidEnforcement
		}
	case ContentRemoval, Warning, Ban:
		if !e.Until.IsZero() {
			return ErrInvalidEnforcement
		}
	default:
		return ErrInvalidEnforcement
	}
	return nil
}

func (e Enforcement) RestrictsAccount(at time.Time) bool {
	return e.Validate() == nil && !at.Before(e.IssuedAt) && (e.Action == Ban || (e.Action == Suspension && at.Before(e.Until)))
}

type AppealOutcome string

const (
	AppealUpheld   AppealOutcome = "upheld"
	AppealReversed AppealOutcome = "reversed"
)

type Appeal struct {
	ID, EnforcementID, Explanation string
	SubmittedAt                    time.Time
	Outcome                        AppealOutcome
	Reviewer, DecisionReason       string
	DecidedAt                      time.Time
}

// NewAppeal validates a proposed submission. The repository must additionally
// enforce the one-appeal-per-enforcement invariant atomically with its audit.
func (e Enforcement) NewAppeal(id, explanation string, at time.Time) (Appeal, error) {
	if e.Validate() != nil || strings.TrimSpace(id) == "" || at.Before(e.IssuedAt) || !at.Before(e.IssuedAt.Add(30*24*time.Hour)) || !utf8.ValidString(explanation) {
		return Appeal{}, ErrAppealUnavailable
	}
	return Appeal{ID: id, EnforcementID: e.ID, Explanation: strings.TrimSpace(explanation), SubmittedAt: at}, nil
}

// Decide returns a new value, preserving submission evidence. Persistence owns
// reviewer authorization and concurrency; neither is inferred from this value.
func (a Appeal) Decide(reviewer string, outcome AppealOutcome, reason string, at time.Time) (Appeal, error) {
	if a.Outcome != "" {
		return Appeal{}, ErrAppealFinal
	}
	if a.ID == "" || a.EnforcementID == "" || a.SubmittedAt.IsZero() || at.Before(a.SubmittedAt) || !meaningful(reviewer) || !meaningful(reason) || (outcome != AppealUpheld && outcome != AppealReversed) {
		return Appeal{}, ErrInvalidEnforcement
	}
	a.Outcome = outcome
	a.Reviewer = strings.TrimSpace(reviewer)
	a.DecisionReason = strings.TrimSpace(reason)
	a.DecidedAt = at
	return a, nil
}

func meaningful(value string) bool { return utf8.ValidString(value) && strings.TrimSpace(value) != "" }
