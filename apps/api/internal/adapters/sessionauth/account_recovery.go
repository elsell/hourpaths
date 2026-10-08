package sessionauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type recoveryAdmissionRepository interface {
	AdmitAccountRecovery(context.Context, application.AccountRecoveryAdmission) (identity.Provider, error)
}

type recoveryCompletionRepository interface {
	CompleteAccountRecovery(context.Context, application.AccountRecoveryCompletion, func(context.Context, string) error) (application.RecoveredAccount, error)
}

func (m *Manager) CompleteAccountRecovery(ctx context.Context, authorization string, c application.AccountRecoveryCompletion) (application.Session, error) {
	token, err := bearer(authorization)
	if err != nil {
		return application.Session{}, err
	}
	repo, ok := m.repository.(recoveryCompletionRepository)
	if !ok {
		return application.Session{}, ports.ErrUnavailable
	}
	random := make([]byte, tokenBytes)
	if _, err := rand.Read(random); err != nil {
		return application.Session{}, err
	}
	newToken := base64.RawURLEncoding.EncodeToString(random)
	oldHash, newHash := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(newToken))
	c.SessionHash, c.NewSessionHash = oldHash[:], newHash[:]
	c.Now = m.clock.Now().UTC().Truncate(time.Microsecond)
	result, err := repo.CompleteAccountRecovery(ctx, c, m.allowAccount)
	if errors.Is(err, ports.ErrNotFound) || errors.Is(err, ports.ErrInvalidCredential) {
		return application.Session{}, application.ErrAccountRecoveryProofInvalid
	}
	if err != nil {
		return application.Session{}, err
	}
	return application.Session{Token: newToken, ExpiresAt: result.ExpiresAt}, nil
}

func (m *Manager) AdmitAccountRecovery(ctx context.Context, authorization string, c application.AccountRecoveryAdmission) (identity.Provider, error) {
	token, err := bearer(authorization)
	if err != nil {
		return "", err
	}
	if err := m.allowAccount(ctx, c.UserID); err != nil {
		return "", err
	}
	repo, ok := m.repository.(recoveryAdmissionRepository)
	if !ok {
		return "", ports.ErrUnavailable
	}
	hash := sha256.Sum256([]byte(token))
	c.SessionHash = hash[:]
	return repo.AdmitAccountRecovery(ctx, c)
}
