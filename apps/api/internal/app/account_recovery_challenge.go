package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type AccountRecoveryAdmission struct {
	UserID, ID           string
	NonceHash            []byte
	SessionHash          []byte
	CreatedAt, ExpiresAt time.Time
	Audit                audit.Event
}

// Admission rechecks the provisional lifecycle and records its exact source
// identity. The returned provider is the other supported provider; email is
// only an eligibility hint and never selects an account to receive the link.
type AccountRecoveryAdmissionRepository interface {
	AdmitAccountRecovery(context.Context, string, AccountRecoveryAdmission) (identity.Provider, error)
}

type AccountRecoveryChallenge struct {
	ID, Nonce string
	Provider  identity.Provider
	ExpiresAt time.Time
}

func (a App) BeginAccountRecovery(ctx context.Context, authorization string) (AccountRecoveryChallenge, error) {
	userID, err := a.recoveryEnrollment(ctx, authorization)
	if err != nil {
		return AccountRecoveryChallenge{}, err
	}
	if a.AccountRecoveryAdmissions == nil {
		return AccountRecoveryChallenge{}, ports.ErrUnavailable
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return AccountRecoveryChallenge{}, ports.ErrUnavailable
	}
	nonce := identityRecoveryNoncePrefix + hex.EncodeToString(random)
	hash := sha256.Sum256([]byte(nonce))
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	challenge := AccountRecoveryChallenge{ID: newID(), Nonce: nonce, ExpiresAt: now.Add(10 * time.Minute)}
	event := a.auditEvent(ctx, userID, userID, audit.ResourceCreated, "account_recovery_challenge", userID, audit.Succeeded)
	event.OccurredAt = now
	provider, err := a.AccountRecoveryAdmissions.AdmitAccountRecovery(ctx, authorization, AccountRecoveryAdmission{
		UserID: userID, ID: challenge.ID, NonceHash: hash[:], CreatedAt: now, ExpiresAt: challenge.ExpiresAt, Audit: event,
	})
	if err != nil {
		return AccountRecoveryChallenge{}, err
	}
	if !provider.Supported() {
		return AccountRecoveryChallenge{}, ports.ErrUnavailable
	}
	challenge.Provider = provider
	return challenge, nil
}

func (a App) recoveryEnrollment(ctx context.Context, authorization string) (string, error) {
	if a.Auth == nil || a.Users == nil || a.Clock == nil || a.Audits == nil {
		return "", ports.ErrUnavailable
	}
	principal, err := a.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return "", err
	}
	if principal.UserID == "" {
		return "", ErrUnauthenticated
	}
	if a.AuditRateLimiter == nil || !a.AuditRateLimiter.Allow(principal.UserID, a.Clock.Now().UTC()) {
		return "", ErrRateLimited
	}
	if len(principal.Scopes) != 1 || principal.Scopes[0] != "api:onboarding" {
		if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "user", principal.UserID, audit.Denied)); err != nil {
			return "", err
		}
		return "", ErrUnauthenticated
	}
	u, err := a.Users.GetProvisionalUser(ctx, principal.UserID)
	if errors.Is(err, ports.ErrNotFound) {
		if auditErr := a.appendDenial(ctx, principal.UserID, "account_recovery", principal.UserID); auditErr != nil {
			return "", auditErr
		}
		return "", ErrUnauthenticated
	}
	if err != nil {
		return "", err
	}
	if u.ID != principal.UserID || u.Status != identity.StatusProvisional {
		if auditErr := a.appendDenial(ctx, principal.UserID, "account_recovery", principal.UserID); auditErr != nil {
			return "", auditErr
		}
		return "", ErrUnauthenticated
	}
	return u.ID, nil
}
