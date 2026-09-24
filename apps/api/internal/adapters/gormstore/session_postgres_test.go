package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestSessionScopeMustMatchAccountLifecycleStatus(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	tx := store.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	txStore := &Store{DB: tx}
	now := time.Now().UTC()

	tests := []struct {
		name      string
		status    identity.Status
		scope     string
		wantSaved bool
	}{
		{name: "provisional onboarding", status: identity.StatusProvisional, scope: "api:onboarding", wantSaved: true},
		{name: "active application", status: identity.StatusActive, scope: "api:user", wantSaved: true},
		{name: "provisional application escalation", status: identity.StatusProvisional, scope: "api:user"},
		{name: "active onboarding regression", status: identity.StatusActive, scope: "api:onboarding"},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := newTestID()
			if err := tx.Create(&userModel{ID: userID, Status: test.status, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				t.Fatal(err)
			}
			tokenHash := make([]byte, 32)
			tokenHash[0] = byte(index + 1)
			identityTokenHash := make([]byte, 32)
			identityTokenHash[0] = byte(index + 11)
			event := audit.Event{ID: newTestID(), OwnerUserID: userID, ActorUserID: userID, Action: audit.SessionCreated, TargetType: "user", TargetID: userID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
			record := ports.SessionRecord{TokenHash: tokenHash, IdentityTokenHash: identityTokenHash, UserID: userID, Scopes: []string{test.scope}, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(12 * time.Hour)}

			err := txStore.SaveSession(ctx, record, event)
			if !test.wantSaved {
				if !errors.Is(err, ports.ErrInvalidArgument) {
					t.Fatalf("SaveSession(%s, %s) returned %v, want ErrInvalidArgument", test.status, test.scope, err)
				}
				var count int64
				if countErr := tx.Model(&sessionModel{}).Where("token_hash = ?", tokenHash).Count(&count).Error; countErr != nil {
					t.Fatal(countErr)
				}
				if count != 0 {
					t.Fatalf("cross-status scope persisted %d session rows", count)
				}
				return
			}
			if err != nil {
				t.Fatalf("SaveSession(%s, %s) returned %v", test.status, test.scope, err)
			}
			principal, err := txStore.ResolveSession(ctx, tokenHash, now)
			if err != nil {
				t.Fatalf("ResolveSession(%s, %s) returned %v", test.status, test.scope, err)
			}
			if principal.UserID != userID || len(principal.Scopes) != 1 || principal.Scopes[0] != test.scope {
				t.Fatalf("resolved principal = %+v, want user %q with only %q", principal, userID, test.scope)
			}
		})
	}
}
