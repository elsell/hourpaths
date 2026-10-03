package gormstore

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"sync"
	"testing"
	"time"
)

func TestConcurrentFirstIdentityExchangeIsIdempotent(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	claims := ports.Claims{Issuer: "https://concurrent.example/" + newTestID(), Subject: "subject", Email: newTestID() + "@example.com", EmailVerified: true}
	userID := identity.UserID(claims.Issuer, claims.Subject)
	const workers = 8
	errorsByWorker := make([]error, workers)
	usersByWorker := make([]identity.User, workers)
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			provisioned := audit.Event{ID: newTestID(), OwnerUserID: userID, ActorUserID: userID, Action: audit.UserProvisioned, TargetType: "user", TargetID: userID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
			profile := provisioned
			profile.ID, profile.Action = newTestID(), audit.UserProfileSynchronized
			usersByWorker[index], errorsByWorker[index] = store.ResolveOrCreate(context.Background(), claims, provisioned, profile, audit.Event{}, true)
		}(i)
	}
	wait.Wait()
	for index, err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent exchange %d failed: %v", index, err)
		}
	}
	userID = usersByWorker[0].ID
	if userID == "" {
		t.Fatal("provisioned account has no ID")
	}
	for _, user := range usersByWorker {
		if user.ID != userID {
			t.Fatal("concurrent exchanges returned different accounts")
		}
	}
	var users, identities, audits int64
	if err := store.DB.Model(&userModel{}).Where("id = ?", userID).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&identityModel{}).Where("issuer = ? AND subject = ?", claims.Issuer, claims.Subject).Count(&identities).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&auditEventModel{}).Where("owner_user_id = ? AND action = ?", userID, audit.UserProvisioned).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if users != 1 || identities != 1 || audits != 1 {
		t.Fatalf("concurrent exchange duplicated state: users=%d identities=%d audits=%d", users, identities, audits)
	}
}
