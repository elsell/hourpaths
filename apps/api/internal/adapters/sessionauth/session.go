package sessionauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"time"
)

const tokenBytes = 32

type Manager struct {
	repository repository
	clock      ports.Clock
}

type repository interface {
	ports.SessionRepository
	ports.OnboardingActivationRepository
}

func New(repository repository, clock ports.Clock) *Manager {
	return &Manager{repository: repository, clock: clock}
}

func (m *Manager) CreateSession(ctx context.Context, userID string, scopes []string, identityTokenHash []byte, expiresAt, absoluteExpiresAt time.Time, event audit.Event) (string, error) {
	if userID == "" || len(scopes) != 1 || !accountLifecycleScope(scopes[0]) || !expiresAt.After(m.clock.Now()) || absoluteExpiresAt.Before(expiresAt) {
		return "", ports.ErrInvalidArgument
	}
	random := make([]byte, tokenBytes)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	hash := sha256.Sum256([]byte(token))
	if err := m.repository.SaveSession(ctx, ports.SessionRecord{TokenHash: hash[:], IdentityTokenHash: append([]byte(nil), identityTokenHash...), UserID: userID, Scopes: append([]string(nil), scopes...), ExpiresAt: expiresAt.UTC(), AbsoluteExpiresAt: absoluteExpiresAt.UTC()}, event); err != nil {
		return "", err
	}
	return token, nil
}

func accountLifecycleScope(scope string) bool {
	return scope == "api:user" || scope == "api:onboarding"
}

func (m *Manager) RotateSession(ctx context.Context, authorization, userID string, scopes []string, expiresAt time.Time, revoked, created audit.Event) (string, time.Time, error) {
	oldToken, err := bearer(authorization)
	if err != nil {
		return "", time.Time{}, err
	}
	if userID == "" || len(scopes) != 1 || scopes[0] != "api:user" || !expiresAt.After(m.clock.Now()) {
		return "", time.Time{}, ports.ErrInvalidArgument
	}
	random := make([]byte, tokenBytes)
	if _, err := rand.Read(random); err != nil {
		return "", time.Time{}, err
	}
	newToken := base64.RawURLEncoding.EncodeToString(random)
	oldHash := sha256.Sum256([]byte(oldToken))
	newHash := sha256.Sum256([]byte(newToken))
	record := ports.SessionRecord{TokenHash: newHash[:], UserID: userID, Scopes: append([]string(nil), scopes...), ExpiresAt: expiresAt.UTC()}
	actualExpiresAt, err := m.repository.RotateSessionHash(ctx, oldHash[:], m.clock.Now().UTC(), record, revoked, created)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return "", time.Time{}, ports.ErrInvalidCredential
		}
		return "", time.Time{}, err
	}
	return newToken, actualExpiresAt, nil
}

func (m *Manager) ActivateOnboarding(ctx context.Context, authorization string, activation identity.OnboardingActivation, expiresAt time.Time, completed, revoked, created audit.Event) (string, time.Time, error) {
	oldToken, err := bearer(authorization)
	if err != nil {
		return "", time.Time{}, err
	}
	now := m.clock.Now().UTC()
	if activation.UserID == "" || !expiresAt.After(now) {
		return "", time.Time{}, ports.ErrInvalidArgument
	}
	random := make([]byte, tokenBytes)
	if _, err := rand.Read(random); err != nil {
		return "", time.Time{}, err
	}
	newToken := base64.RawURLEncoding.EncodeToString(random)
	oldHash := sha256.Sum256([]byte(oldToken))
	newHash := sha256.Sum256([]byte(newToken))
	record := ports.SessionRecord{TokenHash: newHash[:], UserID: activation.UserID, Scopes: []string{"api:user"}, ExpiresAt: expiresAt.UTC()}
	actualExpiresAt, err := m.repository.ActivateOnboarding(ctx, activation, oldHash[:], now, record, completed, revoked, created)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			return "", time.Time{}, ports.ErrInvalidCredential
		}
		return "", time.Time{}, err
	}
	return newToken, actualExpiresAt, nil
}

func (m *Manager) Authenticate(ctx context.Context, authorization string) (ports.Principal, error) {
	token, err := bearer(authorization)
	if err != nil {
		return ports.Principal{}, err
	}
	hash := sha256.Sum256([]byte(token))
	principal, err := m.repository.ResolveSession(ctx, hash[:], m.clock.Now().UTC())
	if errors.Is(err, ports.ErrNotFound) {
		return ports.Principal{}, ports.ErrInvalidCredential
	}
	return principal, err
}

func (m *Manager) RevokeSession(ctx context.Context, authorization string, event audit.Event) error {
	token, err := bearer(authorization)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(token))
	err = m.repository.RevokeSessionHash(ctx, hash[:], m.clock.Now().UTC(), event)
	if errors.Is(err, ports.ErrNotFound) {
		return ports.ErrInvalidCredential
	}
	return err
}

func bearer(header string) (string, error) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", ports.ErrInvalidCredential
	}
	token := strings.TrimPrefix(header, prefix)
	if len(token) != base64.RawURLEncoding.EncodedLen(tokenBytes) || strings.ContainsAny(token, " \t\r\n") {
		return "", ports.ErrInvalidCredential
	}
	if decoded, err := base64.RawURLEncoding.DecodeString(token); err != nil || len(decoded) != tokenBytes {
		return "", ports.ErrInvalidCredential
	}
	return token, nil
}
