package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

var ErrIdentityLinkProofInvalid = errors.New("identity link proof invalid")

const identityLinkNoncePrefix = "hourpaths-link:"
const identityLinkLifetime = 10 * time.Minute

type IdentityLinkChallenge struct {
	ID, Nonce string
	Provider  identity.Provider
	ExpiresAt time.Time
}

type IdentityLinkAdmission struct {
	UserID, ID           string
	Provider             identity.Provider
	NonceHash            []byte
	CreatedAt, ExpiresAt time.Time
	Audit                audit.Event
}

type IdentityLinkCompletion struct {
	UserID, ChallengeID, Issuer, Subject string
	Provider                             identity.Provider
	NonceHash                            []byte
	Now                                  time.Time
	Audit                                audit.Event
}

type IdentityUnlink struct {
	UserID   string
	Provider identity.Provider
	Audit    audit.Event
}

// Implementations serialize account mutations and commit audit evidence atomically.
// No operation may disclose another account's identity associations.
type ProviderIdentityRepository interface {
	ListProviderIdentities(context.Context, string) ([]identity.Provider, error)
	AdmitIdentityLink(context.Context, IdentityLinkAdmission) error
	CompleteIdentityLink(context.Context, IdentityLinkCompletion) error
	UnlinkProviderIdentity(context.Context, IdentityUnlink) error
}

func (a App) LinkedProviders(ctx context.Context, authorization string) ([]identity.Provider, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return nil, err
	}
	if a.ProviderIdentities == nil || a.Audits == nil {
		return nil, ports.ErrUnavailable
	}
	providers, err := a.ProviderIdentities.ListProviderIdentities(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	if len(providers) < 1 || len(providers) > 2 {
		return nil, ports.ErrUnavailable
	}
	seen := map[identity.Provider]bool{}
	for _, provider := range providers {
		if !provider.Supported() || seen[provider] {
			return nil, ports.ErrUnavailable
		}
		seen[provider] = true
	}
	if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, user.ID, user.ID, audit.ResourceListed, "linked_identity", user.ID, audit.Succeeded)); err != nil {
		return nil, err
	}
	return providers, nil
}

func (a App) BeginIdentityLink(ctx context.Context, authorization string, provider identity.Provider) (IdentityLinkChallenge, error) {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return IdentityLinkChallenge{}, err
	}
	if !provider.Supported() {
		return IdentityLinkChallenge{}, ports.ErrInvalidArgument
	}
	if a.ProviderIdentities == nil || a.Clock == nil {
		return IdentityLinkChallenge{}, ports.ErrUnavailable
	}
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return IdentityLinkChallenge{}, ports.ErrUnavailable
	}
	nonce := identityLinkNoncePrefix + hex.EncodeToString(secret)
	hash := sha256.Sum256([]byte(nonce))
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	challenge := IdentityLinkChallenge{ID: newID(), Nonce: nonce, Provider: provider, ExpiresAt: now.Add(identityLinkLifetime)}
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceCreated, "identity_link_challenge", user.ID, audit.Succeeded)
	event.OccurredAt = now
	err = a.ProviderIdentities.AdmitIdentityLink(ctx, IdentityLinkAdmission{UserID: user.ID, ID: challenge.ID, Provider: provider, NonceHash: hash[:], CreatedAt: now, ExpiresAt: challenge.ExpiresAt, Audit: event})
	if err != nil {
		return IdentityLinkChallenge{}, err
	}
	return challenge, nil
}

func (a App) FinishIdentityLink(ctx context.Context, authorization, challengeID, token string) error {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return err
	}
	if a.ProviderIdentities == nil || a.IdentityVerifier == nil || a.Clock == nil {
		return ports.ErrUnavailable
	}
	if challengeID == "" || len(challengeID) > 64 {
		return ports.ErrInvalidArgument
	}
	claims, err := a.IdentityVerifier.Verify(ctx, token)
	if err != nil || !claims.Provider.Supported() || claims.Issuer == "" || claims.Subject == "" || !validIdentityLinkNonce(claims.Nonce) {
		return ErrIdentityLinkProofInvalid
	}
	hash := sha256.Sum256([]byte(claims.Nonce))
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceCreated, "linked_identity", string(claims.Provider), audit.Succeeded)
	event.OccurredAt = now
	err = a.ProviderIdentities.CompleteIdentityLink(ctx, IdentityLinkCompletion{UserID: user.ID, ChallengeID: challengeID, Issuer: claims.Issuer, Subject: claims.Subject, Provider: claims.Provider, NonceHash: hash[:], Now: now, Audit: event})
	if errors.Is(err, ports.ErrInvalidCredential) {
		return ErrIdentityLinkProofInvalid
	}
	return err
}

func validIdentityLinkNonce(nonce string) bool {
	if !strings.HasPrefix(nonce, identityLinkNoncePrefix) {
		return false
	}
	value := strings.TrimPrefix(nonce, identityLinkNoncePrefix)
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func (a App) UnlinkIdentity(ctx context.Context, authorization, reviewedUserID string, provider identity.Provider) error {
	user, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return err
	}
	if a.ProviderIdentities == nil {
		return ports.ErrUnavailable
	}
	if !provider.Supported() {
		return ports.ErrInvalidArgument
	}
	if reviewedUserID != user.ID {
		return ports.ErrConflict
	}
	event := a.auditEvent(ctx, user.ID, user.ID, audit.ResourceDeleted, "linked_identity", string(provider), audit.Succeeded)
	return a.ProviderIdentities.UnlinkProviderIdentity(ctx, IdentityUnlink{UserID: user.ID, Provider: provider, Audit: event})
}
