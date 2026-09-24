package gormstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDuplicateEmailRecoveryDeclineIsDurableScopedAtomicAndIdempotent(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	store := &Store{DB: tx}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	claims := ports.Claims{Issuer: "https://decline.example/" + uuid.NewString(), Subject: uuid.NewString(), Email: " Person@Example.COM ", EmailVerified: true}
	provisionalID := identity.UserID(claims.Issuer, claims.Subject)
	activeID := uuid.NewString()
	for _, user := range []userModel{
		{ID: activeID, Email: "person@example.com", ProviderEmailVerified: true, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now},
		{ID: provisionalID, Email: "person@example.com", ProviderEmailVerified: true, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now},
	} {
		if err := tx.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
	}
	event := identityMutationAudit(uuid.NewString(), audit.DuplicateEmailRecoveryDeclined, provisionalID, now)
	if err := store.DeclineDuplicateEmailRecovery(ctx, provisionalID, event); err != nil {
		t.Fatalf("decline recovery: %v", err)
	}

	var declines, audits int64
	if err := tx.Model(&duplicateEmailRecoveryDeclineModel{}).Where("provisional_user_id = ? AND normalized_email = ?", provisionalID, "person@example.com").Count(&declines).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&auditEventModel{}).Where("owner_user_id = ? AND action = ?", provisionalID, audit.DuplicateEmailRecoveryDeclined).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if declines != 1 || audits != 1 {
		t.Fatalf("first decline persistence declines=%d audits=%d", declines, audits)
	}
	if matched, err := store.HasActiveEmailMatch(ctx, claims); err != nil || matched {
		t.Fatalf("same-email sign-in hint after decline = %v, %v; want suppressed", matched, err)
	}
	otherClaims := claims
	otherClaims.Subject = uuid.NewString()
	if matched, err := store.HasActiveEmailMatch(ctx, otherClaims); err != nil || !matched {
		t.Fatalf("another provisional owner's same-email hint = %v, %v; want unaffected", matched, err)
	}

	repeatEvent := identityMutationAudit(uuid.NewString(), audit.DuplicateEmailRecoveryDeclined, provisionalID, now.Add(time.Minute))
	if err := store.DeclineDuplicateEmailRecovery(ctx, provisionalID, repeatEvent); err != nil {
		t.Fatalf("repeat decline: %v", err)
	}
	if err := tx.Model(&duplicateEmailRecoveryDeclineModel{}).Where("provisional_user_id = ?", provisionalID).Count(&declines).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&auditEventModel{}).Where("owner_user_id = ? AND action = ?", provisionalID, audit.DuplicateEmailRecoveryDeclined).Count(&audits).Error; err != nil {
		t.Fatal(err)
	}
	if declines != 1 || audits != 1 {
		t.Fatalf("repeat decline was not idempotent: declines=%d audits=%d", declines, audits)
	}

	changedEmail := "changed-" + uuid.NewString() + "@example.com"
	if err := tx.Model(&userModel{}).Where("id = ?", provisionalID).Update("email", changedEmail).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&userModel{ID: uuid.NewString(), Email: changedEmail, ProviderEmailVerified: true, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	changedClaims := claims
	changedClaims.Email = "  " + strings.ToUpper(changedEmail) + "  "
	if matched, err := store.HasActiveEmailMatch(ctx, changedClaims); err != nil || !matched {
		t.Fatalf("changed-email sign-in hint = %v, %v; want offered again", matched, err)
	}
}

func TestDuplicateEmailRecoveryDeclineCannotMutateOtherAccountLifecycles(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range []identity.Status{identity.StatusActive, identity.StatusDisabled} {
		t.Run(string(status), func(t *testing.T) {
			tx := db.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			t.Cleanup(func() { _ = tx.Rollback().Error })
			userID := uuid.NewString()
			now := time.Now().UTC().Truncate(time.Microsecond)
			if err := tx.Create(&userModel{ID: userID, Email: "private@example.com", Status: status, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				t.Fatal(err)
			}
			event := identityMutationAudit(uuid.NewString(), audit.DuplicateEmailRecoveryDeclined, userID, now)
			if err := (&Store{DB: tx}).DeclineDuplicateEmailRecovery(context.Background(), userID, event); !errors.Is(err, ports.ErrNotFound) {
				t.Fatalf("%s decline error = %v, want ErrNotFound", status, err)
			}
			var declines int64
			if err := tx.Model(&duplicateEmailRecoveryDeclineModel{}).Where("provisional_user_id = ?", userID).Count(&declines).Error; err != nil || declines != 0 {
				t.Fatalf("%s account persisted decline count=%d err=%v", status, declines, err)
			}
		})
	}
}

func TestDuplicateEmailRecoveryDeclineRejectsUntrustedProvisionalEmail(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	now := time.Now().UTC().Truncate(time.Microsecond)
	userID := uuid.NewString()
	if err := tx.Create(&userModel{ID: userID, Email: "legacy@example.com", ProviderEmailVerified: false, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	event := identityMutationAudit(uuid.NewString(), audit.DuplicateEmailRecoveryDeclined, userID, now)
	if err := (&Store{DB: tx}).DeclineDuplicateEmailRecovery(context.Background(), userID, event); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("untrusted provisional decline error = %v, want ErrNotFound", err)
	}
	var declines int64
	if err := tx.Model(&duplicateEmailRecoveryDeclineModel{}).Where("provisional_user_id = ?", userID).Count(&declines).Error; err != nil || declines != 0 {
		t.Fatalf("untrusted provisional persisted decline count=%d err=%v", declines, err)
	}
}

func TestDuplicateEmailRecoveryDeclineRollsBackWhenAuditCannotAppend(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	userID := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := tx.Create(&userModel{ID: userID, Email: "rollback@example.com", ProviderEmailVerified: true, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	event := identityMutationAudit(uuid.NewString(), audit.DuplicateEmailRecoveryDeclined, userID, now)
	if err := appendAuditEvent(tx, event); err != nil {
		t.Fatal(err)
	}
	if err := (&Store{DB: tx}).DeclineDuplicateEmailRecovery(context.Background(), userID, event); err == nil {
		t.Fatal("decline succeeded when its atomic audit could not append")
	}
	var declines int64
	if err := tx.Model(&duplicateEmailRecoveryDeclineModel{}).Where("provisional_user_id = ?", userID).Count(&declines).Error; err != nil || declines != 0 {
		t.Fatalf("failed audit left decline count=%d err=%v", declines, err)
	}
}

func TestDuplicateEmailRecoveryDeclineRuntimePrivilegesAreAppendOnly(t *testing.T) {
	if *migrationPostgresTestDSN == "" {
		t.Skip("-migration-database-dsn is required")
	}
	db, err := gorm.Open(postgres.Open(*migrationPostgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		PrivilegeType string
	}
	if err := db.Table("information_schema.role_table_grants").
		Select("privilege_type").
		Where("grantee = ? AND table_schema = ? AND table_name = ?", "app", "public", "duplicate_email_recovery_declines").
		Order("privilege_type").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	privileges := make([]string, len(rows))
	for index, row := range rows {
		privileges[index] = row.PrivilegeType
	}
	if !reflect.DeepEqual(privileges, []string{"INSERT", "SELECT"}) {
		t.Fatalf("runtime recovery-decline privileges = %v", privileges)
	}
}

func TestVerifiedEmailMatchCreatesDistinctProvisionalRecoveryCandidate(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	store := &Store{DB: tx}
	now := time.Now().UTC().Truncate(time.Microsecond)
	activeID := "active-" + uuid.NewString()
	email := "Recovery-" + uuid.NewString() + "@Example.COM"
	if err := tx.Create(&userModel{ID: activeID, Email: email, ProviderEmailVerified: true, DisplayName: "Existing private profile", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	claims := ports.Claims{Issuer: "https://new-provider.example/" + uuid.NewString(), Subject: uuid.NewString(), Email: "  " + normalizeEmail(email) + "  ", EmailVerified: true, DisplayName: "New provider seed"}

	matched, err := store.HasActiveEmailMatch(context.Background(), claims)
	if err != nil || !matched {
		t.Fatalf("verified active email match = %v, %v; want true", matched, err)
	}
	provisionalID := identity.UserID(claims.Issuer, claims.Subject)
	provisioned, profile := recoveryAuditEvents(provisionalID, now)
	created, err := store.ResolveOrCreate(context.Background(), claims, provisioned, profile, audit.Event{}, true)
	if err != nil {
		t.Fatalf("create distinct provisional recovery candidate: %v", err)
	}
	if created.ID != provisionalID || created.ID == activeID || created.Status != identity.StatusProvisional {
		t.Fatalf("recovery candidate = %+v, want distinct provisional %q", created, provisionalID)
	}

	var active userModel
	if err := tx.Where("id = ?", activeID).First(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active.DisplayName != "Existing private profile" || active.Status != identity.StatusActive {
		t.Fatalf("matching active account was mutated: %+v", active)
	}
	var users, providerIdentities int64
	if err := tx.Model(&userModel{}).Where("id IN ?", []string{activeID, provisionalID}).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&identityModel{}).Where("issuer = ? AND subject = ? AND user_id = ?", claims.Issuer, claims.Subject, provisionalID).Count(&providerIdentities).Error; err != nil {
		t.Fatal(err)
	}
	if users != 2 || providerIdentities != 1 {
		t.Fatalf("recovery persistence users=%d new-provider-identities=%d", users, providerIdentities)
	}

	matchedAgain, err := store.HasActiveEmailMatch(context.Background(), claims)
	if err != nil || !matchedAgain {
		t.Fatalf("repeat sign-in recovery hint = %v, %v; want true", matchedAgain, err)
	}
	repeated, err := store.ResolveOrCreate(context.Background(), claims, provisioned, profile, audit.Event{}, true)
	if err != nil || repeated.ID != provisionalID {
		t.Fatalf("repeat sign-in recovery candidate = %+v, %v", repeated, err)
	}
	if err := tx.Model(&userModel{}).Where("id IN ?", []string{activeID, provisionalID}).Count(&users).Error; err != nil || users != 2 {
		t.Fatalf("repeat recovery duplicated users: count=%d err=%v", users, err)
	}
}

func TestActiveEmailRecoveryHintIsVerifiedAndStatusBound(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required")
	}
	db, err := gorm.Open(postgres.Open(*postgresTestDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name           string
		storedStatus   identity.Status
		incomingEmail  bool
		verified       bool
		storedVerified bool
		matching       bool
	}{
		{name: "active verified normalized match", storedStatus: identity.StatusActive, incomingEmail: true, verified: true, storedVerified: true, matching: true},
		{name: "legacy active email has no trusted provenance", storedStatus: identity.StatusActive, incomingEmail: true, verified: true},
		{name: "unverified active match", storedStatus: identity.StatusActive, incomingEmail: true},
		{name: "empty verified email", storedStatus: identity.StatusActive, verified: true},
		{name: "nonmatching verified email", storedStatus: identity.StatusActive, incomingEmail: false, verified: true},
		{name: "provisional accounts are not disclosed", storedStatus: identity.StatusProvisional, incomingEmail: true, verified: true},
		{name: "disabled accounts are not disclosed", storedStatus: identity.StatusDisabled, incomingEmail: true, verified: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := db.Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			t.Cleanup(func() { _ = tx.Rollback().Error })
			now := time.Now().UTC().Truncate(time.Microsecond)
			email := "hint-" + uuid.NewString() + "@example.com"
			if err := tx.Create(&userModel{ID: uuid.NewString(), Email: email, ProviderEmailVerified: test.storedVerified, Status: test.storedStatus, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				t.Fatal(err)
			}
			incoming := "different-" + uuid.NewString() + "@example.com"
			if test.incomingEmail {
				incoming = "  " + email + "  "
			}
			if test.name == "empty verified email" {
				incoming = ""
			}
			matched, err := (&Store{DB: tx}).HasActiveEmailMatch(context.Background(), ports.Claims{Issuer: "issuer", Subject: uuid.NewString(), Email: incoming, EmailVerified: test.verified})
			if err != nil || matched != test.matching {
				t.Fatalf("HasActiveEmailMatch() = %v, %v; want %v", matched, err, test.matching)
			}
		})
	}
}

func recoveryAuditEvents(userID string, now time.Time) (audit.Event, audit.Event) {
	provisioned := audit.Event{ID: uuid.NewString(), OwnerUserID: userID, ActorUserID: userID, Action: audit.UserProvisioned, TargetType: "user", TargetID: userID, Outcome: audit.Succeeded, CorrelationID: uuid.NewString(), OccurredAt: now}
	profile := provisioned
	profile.ID, profile.Action = uuid.NewString(), audit.UserProfileSynchronized
	return provisioned, profile
}
