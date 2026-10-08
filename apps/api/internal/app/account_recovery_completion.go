package app

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type AccountRecoveryCompletion struct {
	UserID, ChallengeID, Issuer, Subject   string
	Provider                               identity.Provider
	NonceHash, SessionHash, NewSessionHash []byte
	Now, ExpiresAt                         time.Time
	Completed, Revoked, Linked, Created    audit.Event
}

type RecoveredAccount struct {
	UserID    string
	ExpiresAt time.Time
}

var ErrAccountRecoveryProofInvalid = errors.New("account recovery proof invalid")

type AccountRecoveryCompleter interface {
	CompleteAccountRecovery(context.Context, string, AccountRecoveryCompletion) (Session, error)
}

func (a App) FinishAccountRecovery(ctx context.Context, authorization, challengeID, token string) (Session, error) {
	userID, err := a.recoveryEnrollment(ctx, authorization)
	if err != nil {
		return Session{}, err
	}
	if a.IdentityVerifier == nil || a.AccountRecoveryCompleter == nil || a.SessionTTL < time.Minute || a.SessionTTL > 30*24*time.Hour {
		return Session{}, ports.ErrUnavailable
	}
	if challengeID == "" || len(challengeID) > 64 {
		return Session{}, ports.ErrInvalidArgument
	}
	claims, err := a.IdentityVerifier.Verify(ctx, token)
	if err != nil || claims.Issuer == "" || claims.Subject == "" || !claims.Provider.Supported() || !validIdentityPurposeNonce(claims.Nonce, identityRecoveryNoncePrefix) {
		if auditErr := a.appendDenial(ctx, userID, "account_recovery", userID); auditErr != nil {
			return Session{}, auditErr
		}
		return Session{}, ErrAccountRecoveryProofInvalid
	}
	hash := sha256.Sum256([]byte(claims.Nonce))
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	correlation := correlationID(ctx)
	event := func(action audit.Action, kind string) audit.Event {
		return audit.Event{ID: newID(), OwnerUserID: userID, ActorUserID: userID, Action: action, TargetType: kind, TargetID: userID, Outcome: audit.Succeeded, CorrelationID: correlation, OccurredAt: now}
	}
	session, err := a.AccountRecoveryCompleter.CompleteAccountRecovery(ctx, authorization, AccountRecoveryCompletion{
		UserID: userID, ChallengeID: challengeID, Issuer: claims.Issuer, Subject: claims.Subject, Provider: claims.Provider, NonceHash: hash[:], Now: now, ExpiresAt: now.Add(a.SessionTTL),
		Completed: event(audit.ResourceDeleted, "account_enrollment"), Revoked: event(audit.SessionRevoked, "user"), Linked: event(audit.ResourceCreated, "linked_identity"), Created: event(audit.SessionCreated, "user"),
	})
	if errors.Is(err, ErrAccountRecoveryProofInvalid) || errors.Is(err, ports.ErrConflict) || errors.Is(err, ports.ErrInvalidCredential) || errors.Is(err, ports.ErrNotFound) {
		if auditErr := a.appendDenial(ctx, userID, "account_recovery", userID); auditErr != nil {
			return Session{}, auditErr
		}
	}
	return session, err
}
