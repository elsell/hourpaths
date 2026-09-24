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

func TestIdentityUserFromModelRedactsEmailWithoutVerifiedProvenance(t *testing.T) {
	legacy := userModel{ID: "legacy", Email: "legacy@example.com", ProviderEmailVerified: false, Status: identity.StatusActive}
	if got := identityUserFromModel(legacy); got.Email != "" {
		t.Fatalf("legacy provider email leaked through identity boundary: %+v", got)
	}
	verified := legacy
	verified.ProviderEmailVerified = true
	if got := identityUserFromModel(verified); got.Email != verified.Email {
		t.Fatalf("verified provider email was redacted: %+v", got)
	}
}

func TestIdentityExchangeSeedsProvisionalAndPreservesAppOwnedActiveProfile(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
	}
	ctx := context.Background()
	seedActivationPolicyAuthority(t, ctx)
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	tx := store.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	txStore := &Store{DB: tx}

	now := time.Now().UTC()
	claims := ports.Claims{
		Issuer:          "https://provisional.example/" + newTestID(),
		Subject:         newTestID(),
		Email:           " First." + newTestID() + "@Example.COM ",
		DisplayName:     "Provider Seed",
		EmailVerified:   true,
		InvitationAdmin: false,
	}
	userID := identity.UserID(claims.Issuer, claims.Subject)
	provisioned := identityMutationAudit(newTestID(), audit.UserProvisioned, userID, now)
	initialProfile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now)

	created, err := txStore.ResolveOrCreate(ctx, claims, provisioned, initialProfile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != identity.StatusProvisional {
		t.Fatalf("first identity exchange returned status %q, want %q", created.Status, identity.StatusProvisional)
	}
	if created.Email != normalizeEmail(claims.Email) || created.DisplayName != claims.DisplayName {
		t.Fatalf("provisional profile did not preserve provider seeds: %+v", created)
	}
	var persisted userModel
	if err := tx.Where("id = ?", userID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if !persisted.ProviderEmailVerified {
		t.Fatal("verified provider email was persisted without verification provenance")
	}
	assertIdentityStateCount(t, txStore, claims, userID, 1, 1)

	refreshedClaims := claims
	refreshedClaims.Email = " refreshed." + newTestID() + "@Example.COM "
	refreshedClaims.DisplayName = "Refreshed Provider Seed"
	refreshedProfile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now.Add(time.Second))
	refreshed, err := txStore.ResolveOrCreate(ctx, refreshedClaims, provisioned, refreshedProfile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Status != identity.StatusProvisional || refreshed.Email != normalizeEmail(refreshedClaims.Email) || refreshed.DisplayName != refreshedClaims.DisplayName {
		t.Fatalf("repeat provisional exchange did not refresh provider seeds: %+v", refreshed)
	}
	assertIdentityStateCount(t, txStore, claims, userID, 1, 1)

	if err := tx.Where("id = ?", userID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Status != identity.StatusProvisional || persisted.Email != normalizeEmail(refreshedClaims.Email) || persisted.DisplayName != refreshedClaims.DisplayName {
		t.Fatalf("provisional identity was not preserved after provider refresh: %+v", persisted)
	}
	assertIdentityStateCount(t, txStore, claims, userID, 1, 1)

	activationTime := now.Add(2 * time.Second).Truncate(time.Microsecond)
	onboardingHash := activationHash(71)
	if err := tx.Create(&sessionModel{TokenHash: onboardingHash, UserID: userID, Scopes: "api:onboarding", ExpiresAt: activationTime.Add(time.Hour), AbsoluteExpiresAt: activationTime.Add(4 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	activation := validActivation(userID, "identity_refresh_user", activationTime)
	activation.DisplayName = "App-owned display name"
	completed, revoked, sessionCreated := activationAudits(userID, activationTime)
	if _, err := txStore.ActivateOnboarding(ctx, activation, onboardingHash, activationTime, ports.SessionRecord{TokenHash: activationHash(72), UserID: userID, Scopes: []string{"api:user"}, ExpiresAt: activationTime.Add(time.Hour)}, completed, revoked, sessionCreated); err != nil {
		t.Fatalf("activate identity fixture through lifecycle boundary: %v", err)
	}

	activeClaims := refreshedClaims
	activeClaims.Email = "provider-overwrite." + newTestID() + "@example.com"
	activeClaims.DisplayName = "Provider Overwrite"
	activeClaims.InvitationAdmin = true
	activeProfile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now.Add(3*time.Second))
	returned, err := txStore.ResolveOrCreate(ctx, activeClaims, provisioned, activeProfile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if returned.Status != identity.StatusActive || returned.Email != normalizeEmail(activeClaims.Email) || returned.DisplayName != activation.DisplayName || !returned.InvitationAdmin {
		t.Fatalf("active profile violated provider/app ownership: %+v", returned)
	}
	if err := tx.Where("id = ?", userID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Email != normalizeEmail(activeClaims.Email) || persisted.DisplayName != activation.DisplayName || !persisted.InvitationAdmin {
		t.Fatalf("persisted active profile violated provider/app ownership: %+v", persisted)
	}
	unverifiedActiveClaims := activeClaims
	unverifiedActiveClaims.Email = "unverified-active-replacement@example.com"
	unverifiedActiveClaims.EmailVerified = false
	returned, err = txStore.ResolveOrCreate(ctx, unverifiedActiveClaims, provisioned, identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now.Add(4*time.Second)), audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if returned.Email != normalizeEmail(activeClaims.Email) {
		t.Fatalf("unverified active refresh overwrote trusted provider email: %+v", returned)
	}

	var provisionedAudits, refreshedAudits, activeRefreshAudits int64
	if err := tx.Model(&auditEventModel{}).Where("id = ? AND action = ?", provisioned.ID, audit.UserProvisioned).Count(&provisionedAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&auditEventModel{}).Where("id = ? AND action = ?", refreshedProfile.ID, audit.UserProfileSynchronized).Count(&refreshedAudits).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&auditEventModel{}).Where("id = ? AND action = ?", activeProfile.ID, audit.UserProfileSynchronized).Count(&activeRefreshAudits).Error; err != nil {
		t.Fatal(err)
	}
	if provisionedAudits != 1 || refreshedAudits != 1 || activeRefreshAudits != 1 {
		t.Fatalf("identity audit state is incoherent: provisioned=%d provisional_refresh=%d active_refresh=%d", provisionedAudits, refreshedAudits, activeRefreshAudits)
	}
}

func TestIdentityExchangeNeverTrustsAnUnverifiedOrEmptyProviderEmail(t *testing.T) {
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
	claims := ports.Claims{
		Issuer: "https://broker.example/" + newTestID(), Subject: newTestID(),
		Email: "unverified@example.com",
	}
	userID := identity.UserID(claims.Issuer, claims.Subject)
	provisioned := identityMutationAudit(newTestID(), audit.UserProvisioned, userID, now)
	profile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now)

	created, err := txStore.ResolveOrCreate(ctx, claims, provisioned, profile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if created.Email != "" || created.DisplayName != claims.DisplayName {
		t.Fatalf("first unverified claims persisted trusted email data: %+v", created)
	}
	var persisted userModel
	if err := tx.Where("id = ?", userID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.ProviderEmailVerified {
		t.Fatal("first unverified claims persisted verified-email provenance")
	}

	verified := claims
	verified.Email = "opaque@privaterelay.appleid.com"
	verified.EmailVerified = true
	verifiedProfile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now.Add(time.Second))
	refreshed, err := txStore.ResolveOrCreate(ctx, verified, provisioned, verifiedProfile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Email != verified.Email {
		t.Fatalf("verified private-relay email was not retained: %+v", refreshed)
	}

	unverified := verified
	unverified.Email = "replacement-unverified@example.com"
	unverified.EmailVerified = false
	unverified.DisplayName = "Later provider name"
	unverifiedProfile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now.Add(2*time.Second))
	refreshed, err = txStore.ResolveOrCreate(ctx, unverified, provisioned, unverifiedProfile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Email != verified.Email || refreshed.DisplayName != unverified.DisplayName {
		t.Fatalf("unverified refresh overwrote trusted email or lost editable name seed: %+v", refreshed)
	}

	verifiedWithoutEmail := verified
	verifiedWithoutEmail.Email = " \t"
	verifiedWithoutEmail.DisplayName = "Provider omitted email"
	missingEmailProfile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now.Add(3*time.Second))
	refreshed, err = txStore.ResolveOrCreate(ctx, verifiedWithoutEmail, provisioned, missingEmailProfile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.Email != verified.Email || refreshed.DisplayName != verifiedWithoutEmail.DisplayName {
		t.Fatalf("blank verified claim cleared trusted email or lost editable name seed: %+v", refreshed)
	}

	if err := tx.Where("id = ?", userID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Email != verified.Email || !persisted.ProviderEmailVerified || persisted.DisplayName != verifiedWithoutEmail.DisplayName {
		t.Fatalf("persisted profile accepted an unverified replacement email: %+v", persisted)
	}
}

func TestLegacyProviderEmailRemainsUntrustedUntilVerifiedRefresh(t *testing.T) {
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

	now := time.Now().UTC().Truncate(time.Microsecond)
	claims := ports.Claims{Issuer: "https://legacy.example/" + newTestID(), Subject: newTestID(), Email: "legacy@example.com"}
	userID := identity.UserID(claims.Issuer, claims.Subject)
	if err := tx.Create(&userModel{ID: userID, Email: claims.Email, ProviderEmailVerified: false, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&identityModel{Issuer: claims.Issuer, Subject: claims.Subject, UserID: userID}).Error; err != nil {
		t.Fatal(err)
	}
	provisioned := identityMutationAudit(newTestID(), audit.UserProvisioned, userID, now)
	profile := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now)

	resolved, err := txStore.ResolveOrCreate(ctx, claims, provisioned, profile, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Email != "" {
		t.Fatalf("ResolveOrCreate exposed legacy email without verification provenance: %+v", resolved)
	}
	loaded, err := txStore.GetProvisionalUser(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Email != "" {
		t.Fatalf("GetProvisionalUser exposed legacy email without verification provenance: %+v", loaded)
	}
	var persisted userModel
	if err := tx.Where("id = ?", userID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.ProviderEmailVerified {
		t.Fatal("unverified refresh promoted a legacy email to verified")
	}
	activeLegacyID := newTestID()
	if err := tx.Create(&userModel{ID: activeLegacyID, Email: claims.Email, ProviderEmailVerified: false, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	activeLegacy, err := txStore.GetUser(ctx, activeLegacyID)
	if err != nil {
		t.Fatal(err)
	}
	if activeLegacy.Email != "" {
		t.Fatalf("GetUser exposed legacy email without verification provenance: %+v", activeLegacy)
	}
	if matched, err := txStore.HasActiveEmailMatch(ctx, ports.Claims{Issuer: "other", Subject: newTestID(), Email: claims.Email, EmailVerified: true}); err != nil || matched {
		t.Fatalf("legacy email drove duplicate recovery: matched=%v err=%v", matched, err)
	}

	verified := claims
	verified.Email = "verified@example.com"
	verified.EmailVerified = true
	if _, err := txStore.ResolveOrCreate(ctx, verified, provisioned, identityMutationAudit(newTestID(), audit.UserProfileSynchronized, userID, now.Add(time.Second)), audit.Event{}, true); err != nil {
		t.Fatal(err)
	}
	if err := tx.Where("id = ?", userID).First(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted.Email != verified.Email || !persisted.ProviderEmailVerified {
		t.Fatalf("verified refresh did not establish trusted provenance: %+v", persisted)
	}
}

func TestGetProvisionalUserIsLifecycleAndOwnerScoped(t *testing.T) {
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

	provisionalID := newTestID()
	otherProvisionalID := newTestID()
	activeID := newTestID()
	disabledID := newTestID()
	for _, user := range []userModel{
		{ID: provisionalID, Email: "owner-private@example.com", ProviderEmailVerified: true, DisplayName: "Owner Seed", Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now},
		{ID: otherProvisionalID, Email: "other-private@example.com", ProviderEmailVerified: true, DisplayName: "Other Seed", Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now},
		{ID: activeID, Email: "active-private@example.com", ProviderEmailVerified: true, DisplayName: "Active User", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: disabledID, Email: "disabled-private@example.com", ProviderEmailVerified: true, DisplayName: "Disabled User", Status: identity.StatusDisabled, CreatedAt: now, UpdatedAt: now},
	} {
		if err := tx.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}

	got, err := txStore.GetProvisionalUser(ctx, provisionalID)
	if err != nil {
		t.Fatalf("get provisional owner: %v", err)
	}
	if got.ID != provisionalID || got.Email != "owner-private@example.com" || got.DisplayName != "Owner Seed" || got.Status != identity.StatusProvisional {
		t.Fatalf("provisional owner = %+v", got)
	}
	if got.ID == otherProvisionalID || got.Email == "other-private@example.com" || got.DisplayName == "Other Seed" {
		t.Fatalf("owner-scoped lookup leaked another provisional user: %+v", got)
	}

	for _, userID := range []string{activeID, disabledID, newTestID()} {
		if _, err := txStore.GetProvisionalUser(ctx, userID); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("GetProvisionalUser(%q) error = %v, want ErrNotFound", userID, err)
		}
	}
}

func identityMutationAudit(id string, action audit.Action, userID string, occurredAt time.Time) audit.Event {
	return audit.Event{
		ID:            id,
		OwnerUserID:   userID,
		ActorUserID:   userID,
		Action:        action,
		TargetType:    "user",
		TargetID:      userID,
		Outcome:       audit.Succeeded,
		CorrelationID: newTestID(),
		OccurredAt:    occurredAt,
	}
}

func assertIdentityStateCount(t *testing.T, store *Store, claims ports.Claims, userID string, wantUsers, wantIdentities int64) {
	t.Helper()
	var users, identities int64
	if err := store.DB.Model(&userModel{}).Where("id = ?", userID).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.Model(&identityModel{}).Where("issuer = ? AND subject = ? AND user_id = ?", claims.Issuer, claims.Subject, userID).Count(&identities).Error; err != nil {
		t.Fatal(err)
	}
	if users != wantUsers || identities != wantIdentities {
		t.Fatalf("identity state count: users=%d identities=%d, want users=%d identities=%d", users, identities, wantUsers, wantIdentities)
	}
}
