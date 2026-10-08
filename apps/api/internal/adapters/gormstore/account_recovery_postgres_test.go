package gormstore

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

func TestAccountRecoveryAdmissionPreservesAccountsAndRejectsIneligibleEnrollment(t *testing.T) {
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
	for _, user := range []userModel{source, target} {
		if err := admin.DB.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	issuer := "https://recovery.example/" + newTestID()
	sessionHash := sha256.Sum256([]byte(newTestID()))
	if err := admin.DB.Create(&sessionModel{TokenHash: sessionHash[:], UserID: source.ID, Scopes: "api:onboarding", CreatedAt: now, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(30 * 24 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range []identityModel{
		{Issuer: issuer, Subject: "new-apple", UserID: source.ID, Provider: identity.ProviderApple},
		{Issuer: issuer, Subject: "existing-google", UserID: target.ID, Provider: identity.ProviderGoogle},
	} {
		if err := admin.DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	admission := func() application.AccountRecoveryAdmission {
		hash := sha256.Sum256([]byte(newTestID()))
		return application.AccountRecoveryAdmission{UserID: source.ID, ID: newTestID(), SessionHash: sessionHash[:], NonceHash: hash[:], CreatedAt: now, ExpiresAt: now.Add(10 * time.Minute),
			Audit: audit.Event{ID: newTestID(), OwnerUserID: source.ID, ActorUserID: source.ID, Action: audit.ResourceCreated, TargetType: "account_recovery_challenge", TargetID: source.ID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}}
	}
	first := admission()
	wrongCredential := admission()
	wrongHash := sha256.Sum256([]byte("another onboarding credential"))
	wrongCredential.SessionHash = wrongHash[:]
	if _, err := store.AdmitAccountRecovery(ctx, wrongCredential); err == nil {
		t.Fatal("challenge admitted with an unproven onboarding credential")
	}
	provider, err := store.AdmitAccountRecovery(ctx, first)
	if err != nil || provider != identity.ProviderGoogle {
		t.Fatalf("eligible offer: %s %v", provider, err)
	}
	second := admission()
	if _, err := store.AdmitAccountRecovery(ctx, second); err != nil {
		t.Fatal(err)
	}
	var pending accountRecoveryChallengeModel
	if err := admin.DB.Where("user_id = ?", source.ID).Take(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if pending.ID != second.ID || string(pending.NonceHash) != string(second.NonceHash) || pending.SourceIssuer != issuer || pending.SourceSubject != "new-apple" {
		t.Fatal("challenge did not replace and bind the exact enrollment identity")
	}
	for _, change := range []struct {
		column  string
		value   any
		restore any
	}{
		{"provider_email_verified", false, true},
		{"email", "nonmatching@example.test", source.Email},
	} {
		if err := admin.DB.Model(&userModel{}).Where("id = ?", source.ID).Update(change.column, change.value).Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.AdmitAccountRecovery(ctx, admission()); err == nil {
			t.Fatalf("ineligible %s admitted", change.column)
		}
		if err := admin.DB.Model(&userModel{}).Where("id = ?", source.ID).Update(change.column, change.restore).Error; err != nil {
			t.Fatal(err)
		}
	}
	completed := admission()
	completed.UserID = target.ID
	completed.Audit.OwnerUserID, completed.Audit.ActorUserID, completed.Audit.TargetID = target.ID, target.ID, target.ID
	if _, err := store.AdmitAccountRecovery(ctx, completed); err == nil {
		t.Fatal("completed account admitted for provisional recovery")
	}
	failed := admission()
	failed.Audit.ID = first.Audit.ID
	if _, err := store.AdmitAccountRecovery(ctx, failed); err == nil {
		t.Fatal("audit collision accepted")
	}
	if err := admin.DB.Where("user_id = ?", source.ID).Take(&pending).Error; err != nil || pending.ID != second.ID {
		t.Fatal("failed audit consumed the previous challenge")
	}
	var linked identityModel
	if err := admin.DB.Where("issuer = ? AND subject = ?", issuer, "new-apple").Take(&linked).Error; err != nil || linked.UserID != source.ID {
		t.Fatal("admission changed an identity association")
	}
	if err := admin.DB.Model(&sessionModel{}).Where("token_hash = ?", sessionHash[:]).Update("revoked_at", now).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.AdmitAccountRecovery(ctx, admission()); err == nil {
		t.Fatal("revoked onboarding credential admitted a new challenge")
	}
}

func recoveryCloseStore(t *testing.T, store *Store) {
	t.Helper()
	connection, err := store.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
}
