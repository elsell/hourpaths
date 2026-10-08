package gormstore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/sessionauth"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type recoveryFixedClock struct{ now time.Time }

func (c recoveryFixedClock) Now() time.Time { return c.now }

func TestAccountRecoveryCompletionIsAtomicAndKeepsTheExistingAccount(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("requires PostgreSQL runtime and migration DSNs")
	}
	ctx := context.Background()
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	recoveryCloseStore(t, store)
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	recoveryCloseStore(t, admin)
	now := time.Now().UTC().Truncate(time.Microsecond)
	source := userModel{ID: newTestID(), Email: newTestID() + "@example.test", ProviderEmailVerified: true, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now}
	target := source
	target.ID = newTestID()
	target.Status = identity.StatusActive
	target.DisplayName = "Retained profile"
	for _, user := range []userModel{source, target} {
		if err := admin.DB.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	issuer := "https://recovery-completion.example/" + newTestID()
	for _, row := range []identityModel{{Issuer: issuer, Subject: "new-apple", UserID: source.ID, Provider: identity.ProviderApple}, {Issuer: issuer, Subject: "existing-google", UserID: target.ID, Provider: identity.ProviderGoogle}} {
		if err := admin.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	hash := func() []byte { sum := sha256.Sum256([]byte(newTestID())); return sum[:] }
	oldToken := base64.RawURLEncoding.EncodeToString(hash())
	oldSum := sha256.Sum256([]byte(oldToken))
	oldHash, siblingHash, targetHash, newHash, nonceHash := oldSum[:], hash(), hash(), hash(), hash()
	consumedProofHash := hash()
	absolute := now.Add(2 * time.Hour)
	for _, row := range []sessionModel{
		{TokenHash: oldHash, IdentityTokenHash: consumedProofHash, UserID: source.ID, Scopes: "api:onboarding", CreatedAt: now, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: absolute},
		{TokenHash: siblingHash, UserID: source.ID, Scopes: "api:onboarding", CreatedAt: now, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: absolute},
		{TokenHash: targetHash, UserID: target.ID, Scopes: "api:user", CreatedAt: now, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: absolute},
	} {
		if err := admin.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	resource := resourceModel{ID: newTestID(), Domain: "example", OwnerUserID: target.ID, Name: "Preserve existing data", CreatedAt: now}
	if err := admin.DB.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	event := func(action audit.Action, kind string) audit.Event {
		return audit.Event{ID: newTestID(), OwnerUserID: source.ID, ActorUserID: source.ID, Action: action, TargetType: kind, TargetID: source.ID, Outcome: audit.Succeeded, CorrelationID: "recovery-fixture", OccurredAt: now}
	}
	admitted := event(audit.ResourceCreated, "account_recovery_challenge")
	challengeID := newTestID()
	if _, err := store.AdmitAccountRecovery(ctx, application.AccountRecoveryAdmission{UserID: source.ID, ID: challengeID, SessionHash: oldHash, NonceHash: nonceHash, CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute), Audit: admitted}); err != nil {
		t.Fatal(err)
	}
	c := application.AccountRecoveryCompletion{UserID: source.ID, ChallengeID: challengeID, Issuer: issuer, Subject: "existing-google", Provider: identity.ProviderGoogle, NonceHash: nonceHash, SessionHash: oldHash, NewSessionHash: newHash, Now: now, ExpiresAt: now.Add(30 * 24 * time.Hour), Completed: event(audit.ResourceDeleted, "account_enrollment"), Revoked: event(audit.SessionRevoked, "user"), Linked: event(audit.ResourceCreated, "linked_identity"), Created: event(audit.SessionCreated, "user")}
	allow := func(context.Context, string) error { return nil }
	for _, tc := range []struct {
		name   string
		change func(*application.AccountRecoveryCompletion)
	}{
		{"wrong nonce", func(c *application.AccountRecoveryCompletion) { c.NonceHash = hash() }},
		{"other onboarding credential", func(c *application.AccountRecoveryCompletion) { c.SessionHash = siblingHash }},
		{"expired challenge", func(c *application.AccountRecoveryCompletion) { c.Now = now.Add(10 * time.Minute) }},
		{"wrong provider", func(c *application.AccountRecoveryCompletion) { c.Provider = identity.ProviderApple }},
		{"unknown identity", func(c *application.AccountRecoveryCompletion) { c.Subject = "unknown" }},
		{"audit failure", func(c *application.AccountRecoveryCompletion) { c.Created.ID = admitted.ID }},
		{"credential collision", func(c *application.AccountRecoveryCompletion) { c.NewSessionHash = targetHash }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := c
			tc.change(&changed)
			if _, err := store.CompleteAccountRecovery(ctx, changed, allow); err == nil {
				t.Fatal("invalid recovery committed")
			}
			var count int64
			if err := admin.DB.Model(&identityModel{}).Where("user_id = ?", source.ID).Count(&count).Error; err != nil || count != 1 {
				t.Fatal("failed recovery mutated source identity")
			}
		})
	}
	if _, err := store.CompleteAccountRecovery(ctx, c, func(_ context.Context, user string) error {
		if user == target.ID {
			return errors.New("deletion pending")
		}
		return nil
	}); err == nil {
		t.Fatal("destination deletion fence bypassed")
	}
	manager := sessionauth.NewWithAccountAccess(store, recoveryFixedClock{now: now}, allow)
	result, err := manager.CompleteAccountRecovery(ctx, "Bearer "+oldToken, c)
	if err != nil || !result.ExpiresAt.Equal(absolute) {
		t.Fatalf("recovery failed or changed absolute expiry: %v", err)
	}
	principal, err := manager.Authenticate(ctx, "Bearer "+result.Token)
	if err != nil || principal.UserID != target.ID {
		t.Fatal("recovered credential cannot authenticate the retained account")
	}
	if _, err := manager.Authenticate(ctx, "Bearer "+oldToken); err == nil {
		t.Fatal("old enrollment credential remains valid")
	}
	replayEvent := event(audit.SessionCreated, "user")
	replayEvent.OwnerUserID, replayEvent.ActorUserID, replayEvent.TargetID = target.ID, target.ID, target.ID
	if err := store.SaveSession(ctx, ports.SessionRecord{TokenHash: hash(), IdentityTokenHash: consumedProofHash, UserID: target.ID, Scopes: []string{"api:user"}, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: absolute}, replayEvent); err == nil {
		t.Fatal("recovery erased consumed identity token replay protection")
	}
	newSum := sha256.Sum256([]byte(result.Token))
	newHash = newSum[:]
	for _, table := range []string{"user_models", "session_models", "account_recovery_challenge_models"} {
		var count int64
		column := "user_id"
		if table == "user_models" {
			column = "id"
		}
		if err := admin.DB.Table(table).Where(column+" = ?", source.ID).Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("retired enrollment remains in %s: %d %v", table, count, err)
		}
	}
	var providers int64
	if err := admin.DB.Model(&identityModel{}).Where("user_id = ?", target.ID).Count(&providers).Error; err != nil || providers != 2 {
		t.Fatal("both identities do not reach retained account")
	}
	var saved userModel
	if err := admin.DB.Where("id = ?", target.ID).Take(&saved).Error; err != nil || saved.DisplayName != target.DisplayName || saved.Email != target.Email {
		t.Fatal("retained profile changed")
	}
	var savedResource resourceModel
	if err := admin.DB.Where("id = ? AND owner_user_id = ?", resource.ID, target.ID).Take(&savedResource).Error; err != nil || savedResource.Name != resource.Name {
		t.Fatal("retained product data changed")
	}
	var session sessionModel
	if err := admin.DB.Where("token_hash = ?", newHash).Take(&session).Error; err != nil || session.UserID != target.ID || session.Scopes != "api:user" || !session.AbsoluteExpiresAt.Equal(absolute) {
		t.Fatal("new credential lost owner/scope/absolute expiry")
	}
	for _, e := range []audit.Event{c.Completed, c.Revoked, c.Linked, c.Created} {
		var count int64
		if err := admin.DB.Model(&auditEventModel{}).Where("id = ?", e.ID).Count(&count).Error; err != nil || count != 1 {
			t.Fatal("recovery audit missing")
		}
	}
	if _, err := store.CompleteAccountRecovery(ctx, c, allow); err == nil {
		t.Fatal("completed recovery replayed")
	}
}
