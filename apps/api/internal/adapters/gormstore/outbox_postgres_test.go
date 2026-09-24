package gormstore

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var postgresTestDSN = flag.String("database-dsn", "", "PostgreSQL integration test DSN")
var migrationPostgresTestDSN = flag.String("migration-database-dsn", "", "migration-role PostgreSQL integration test DSN")

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
	var wait sync.WaitGroup
	for i := 0; i < workers; i++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			provisioned := audit.Event{ID: newTestID(), OwnerUserID: userID, ActorUserID: userID, Action: audit.UserProvisioned, TargetType: "user", TargetID: userID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
			profile := provisioned
			profile.ID, profile.Action = newTestID(), audit.UserProfileSynchronized
			_, errorsByWorker[index] = store.ResolveOrCreate(context.Background(), claims, provisioned, profile, audit.Event{}, true)
		}(i)
	}
	wait.Wait()
	for index, err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent exchange %d failed: %v", index, err)
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

func newTestID() string { return uuid.NewString() }

func TestDeadLettersAreOwnerScopedAndRequeuedWithAudit(t *testing.T) {
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
	now := time.Now().UTC()
	for _, id := range []string{"dead-owner", "dead-subject", "dead-other"} {
		if err := tx.Create(&userModel{ID: id, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	deadAt := now
	row := authorizationOutboxModel{ID: "dead-change", ResourceType: "resource", ResourceID: "example/id", Relation: "viewer", SubjectType: "user", SubjectID: "dead-subject", OwnerUserID: "dead-owner", ActorUserID: "dead-owner", Operation: ports.AuthorizationTouch, Attempts: 5, FailureCode: "dependency_failure", DeadLetteredAt: &deadAt, CreatedAt: now}
	if err := tx.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	txStore := &Store{DB: tx}
	request := ports.PageRequest{Limit: 10, Snapshot: now.Add(time.Second)}
	visible, err := txStore.ListAuthorizationDeadLetters(ctx, "dead-owner", request)
	if err != nil || len(visible.Items) != 1 || visible.Items[0].ID != row.ID {
		t.Fatalf("owner did not see dead letter: %+v %v", visible, err)
	}
	hidden, err := txStore.ListAuthorizationDeadLetters(ctx, "dead-other", request)
	if err != nil || len(hidden.Items) != 0 {
		t.Fatalf("dead letter leaked cross-user: %+v %v", hidden, err)
	}
	subjectView, err := txStore.ListAuthorizationDeadLetters(ctx, "dead-subject", request)
	if err != nil || len(subjectView.Items) != 0 {
		t.Fatalf("relationship subject received the owner's recovery item: %+v %v", subjectView, err)
	}
	event := audit.Event{ID: "dead-requeue-event", OwnerUserID: "dead-owner", ActorUserID: "dead-owner", Action: audit.AuthorizationDeadLetterRequeued, TargetType: "authorization_change", TargetID: row.ID, Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	wrongEvent := event
	wrongEvent.ID, wrongEvent.OwnerUserID, wrongEvent.ActorUserID = "wrong-requeue-event", "dead-subject", "dead-subject"
	if _, err := txStore.RequeueAuthorizationDeadLetter(ctx, row.ID, "dead-subject", "worker-other", time.Minute, wrongEvent); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("relationship subject requeue returned %v", err)
	}
	claimed, err := txStore.RequeueAuthorizationDeadLetter(ctx, row.ID, "dead-owner", "worker-owner", time.Minute, event)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.ID != row.ID || claimed.LockedBy != "worker-owner" || claimed.LockedUntil.IsZero() {
		t.Fatalf("requeued change was not atomically claimed: %+v", claimed)
	}
	var recovered authorizationOutboxModel
	if err := tx.Where("id = ?", row.ID).First(&recovered).Error; err != nil {
		t.Fatal(err)
	}
	if recovered.DeadLetteredAt != nil || recovered.Attempts != 0 || recovered.FailureCode != "" || recovered.LockedBy != "worker-owner" || recovered.LockedUntil == nil {
		t.Fatalf("dead-letter state not cleared: %+v", recovered)
	}
	var auditCount int64
	if err := tx.Model(&auditEventModel{}).Where("id = ?", event.ID).Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("requeue audit missing: count=%d err=%v", auditCount, err)
	}
}

func TestAccountDeactivationAtomicallyRevokesSessionsAndAudits(t *testing.T) {
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
	if err := tx.Create(&userModel{ID: "deactivate-user", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	tokenHash := make([]byte, 32)
	tokenHash[0] = 1
	createdEvent := audit.Event{ID: "deactivate-session-created", OwnerUserID: "deactivate-user", ActorUserID: "deactivate-user", Action: audit.SessionCreated, TargetType: "user", TargetID: "deactivate-user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	absoluteExpiresAt := now.Add(90 * time.Minute)
	if err := txStore.SaveSession(ctx, ports.SessionRecord{TokenHash: tokenHash, IdentityTokenHash: make([]byte, 32), UserID: "deactivate-user", Scopes: []string{"api:user"}, ExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: absoluteExpiresAt}, createdEvent); err != nil {
		t.Fatal(err)
	}
	rotatedHash := make([]byte, 32)
	rotatedHash[0] = 2
	revokedEvent := audit.Event{ID: "rotate-session-revoked", OwnerUserID: "deactivate-user", ActorUserID: "deactivate-user", Action: audit.SessionRevoked, TargetType: "user", TargetID: "deactivate-user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	rotatedEvent := audit.Event{ID: "rotate-session-created", OwnerUserID: "deactivate-user", ActorUserID: "deactivate-user", Action: audit.SessionCreated, TargetType: "user", TargetID: "deactivate-user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	actualExpiry, err := txStore.RotateSessionHash(ctx, tokenHash, now, ports.SessionRecord{TokenHash: rotatedHash, UserID: "deactivate-user", Scopes: []string{"api:user"}, ExpiresAt: now.Add(2 * time.Hour)}, revokedEvent, rotatedEvent)
	if err != nil {
		t.Fatal(err)
	}
	if delta := actualExpiry.Sub(absoluteExpiresAt); delta < -time.Microsecond || delta > time.Microsecond {
		t.Fatalf("rotation extended the absolute session lifetime: %v", actualExpiry)
	}
	if _, err := txStore.ResolveSession(ctx, tokenHash, now); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("rotated credential remained valid: %v", err)
	}
	if principal, err := txStore.ResolveSession(ctx, rotatedHash, now); err != nil || principal.UserID != "deactivate-user" {
		t.Fatalf("replacement credential invalid: %+v %v", principal, err)
	}
	event := audit.Event{ID: "deactivate-event", OwnerUserID: "deactivate-user", ActorUserID: "deactivate-user", Action: audit.UserDeactivated, TargetType: "user", TargetID: "deactivate-user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: now}
	if err := txStore.DisableUser(ctx, "deactivate-user", event); err != nil {
		t.Fatal(err)
	}
	if _, err := txStore.GetUser(ctx, "deactivate-user"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("disabled user remained active: %v", err)
	}
	if _, err := txStore.ResolveSession(ctx, tokenHash, now); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("disabled account session remained usable: %v", err)
	}
	var auditCount int64
	if err := tx.Model(&auditEventModel{}).Where("id = ? AND action = ?", event.ID, audit.UserDeactivated).Count(&auditCount).Error; err != nil || auditCount != 1 {
		t.Fatalf("deactivation audit missing: count=%d err=%v", auditCount, err)
	}
}

func TestAuditEventsAreAppendOnlyAndVisibleOnlyToActorOrOwner(t *testing.T) {
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
	for _, id := range []string{"audit-owner", "audit-actor", "audit-stranger"} {
		if err := tx.Create(&userModel{ID: id, CreatedAt: time.Now(), UpdatedAt: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	event := audit.Event{ID: "audit-event", OwnerUserID: "audit-owner", ActorUserID: "audit-actor", Action: audit.ResourceViewed, TargetType: "habit", TargetID: "habit-id", Outcome: audit.Succeeded, CorrelationID: "request-id", OccurredAt: time.Now().UTC()}
	txStore := &Store{DB: tx}
	claims := ports.Claims{Issuer: "https://audit.example", Subject: "new-user"}
	provisionedID := identity.UserID(claims.Issuer, claims.Subject)
	wrongProvisioning := audit.Event{ID: "wrong-provision", OwnerUserID: provisionedID, ActorUserID: provisionedID, Action: audit.UserProvisioned, TargetType: "user", TargetID: "other-user", Outcome: audit.Succeeded, CorrelationID: "request", OccurredAt: event.OccurredAt}
	wrongProfile := wrongProvisioning
	wrongProfile.Action = audit.UserProfileSynchronized
	if _, err := txStore.ResolveOrCreate(ctx, claims, wrongProvisioning, wrongProfile, audit.Event{}, true); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("mismatched provisioning audit accepted: %v", err)
	}
	var rejectedProvisioningCount int64
	if err := tx.Model(&userModel{}).Where("id = ?", provisionedID).Count(&rejectedProvisioningCount).Error; err != nil || rejectedProvisioningCount != 0 {
		t.Fatalf("user committed without coherent audit: count=%d err=%v", rejectedProvisioningCount, err)
	}
	var rejectedAuditCount int64
	if err := tx.Model(&auditEventModel{}).Where("id = ?", wrongProvisioning.ID).Count(&rejectedAuditCount).Error; err != nil || rejectedAuditCount != 0 {
		t.Fatalf("invalid provisioning audit committed: count=%d err=%v", rejectedAuditCount, err)
	}
	provisioning := wrongProvisioning
	provisioning.ID, provisioning.TargetID = "correct-provision", provisionedID
	profile := wrongProfile
	profile.ID, profile.TargetID = "profile-unused", provisionedID
	if _, err := txStore.ResolveOrCreate(ctx, claims, provisioning, profile, audit.Event{}, true); err != nil {
		t.Fatal(err)
	}
	changedClaims := claims
	changedClaims.Email, changedClaims.DisplayName = "changed@example.com", "Changed"
	changedClaims.EmailVerified = true
	profile.ID = "profile-changed"
	if user, err := txStore.ResolveOrCreate(ctx, changedClaims, provisioning, profile, audit.Event{}, true); err != nil || user.Email != changedClaims.Email || user.DisplayName != changedClaims.DisplayName {
		t.Fatalf("profile synchronization failed: %+v %v", user, err)
	}
	var profileAuditCount int64
	if err := tx.Model(&auditEventModel{}).Where("id = ? AND action = ?", profile.ID, audit.UserProfileSynchronized).Count(&profileAuditCount).Error; err != nil || profileAuditCount != 1 {
		t.Fatalf("profile update audit missing: count=%d err=%v", profileAuditCount, err)
	}
	var provisionedCount int64
	if err := tx.Model(&userModel{}).Where("id = ?", provisionedID).Count(&provisionedCount).Error; err != nil || provisionedCount != 1 {
		t.Fatalf("successfully audited user provisioning missing: count=%d err=%v", provisionedCount, err)
	}
	resource := ports.Resource{ID: "unaudited-resource", Domain: "habit", OwnerUserID: "audit-owner", Name: "Walk", CreatedAt: event.OccurredAt}
	change := ports.AuthorizationChange{ID: "unaudited-change", ResourceType: "resource", ResourceID: "habit/unaudited-resource", Relation: "owner", SubjectType: "user", SubjectID: "audit-owner", OwnerUserID: "audit-owner", ActorUserID: "audit-owner", Operation: ports.AuthorizationTouch, LockedBy: "worker", Lease: time.Minute}
	mismatched := event
	mismatched.ID = "mismatched-event"
	mismatched.Action = audit.ResourceViewed
	mismatched.OwnerUserID = "audit-owner"
	mismatched.ActorUserID = "audit-owner"
	mismatched.TargetID = resource.ID
	if _, _, err := txStore.CreateResource(ctx, resource, change, mismatched, ports.Idempotency{PrincipalID: resource.OwnerUserID, Operation: "resource.create:habit", Key: "request-key-0001", RequestHash: make([]byte, 32)}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("invalid audit did not reject mutation: %v", err)
	}
	var resourceCount int64
	if err := tx.Model(&resourceModel{}).Where("id = ?", resource.ID).Count(&resourceCount).Error; err != nil || resourceCount != 0 {
		t.Fatalf("resource committed without audit: count=%d err=%v", resourceCount, err)
	}
	var rejectedIdempotencyCount, rejectedOutboxCount, rejectedResourceAuditCount int64
	if err := tx.Model(&idempotencyModel{}).Where("principal_id = ? AND operation = ? AND key = ?", resource.OwnerUserID, "resource.create:habit", "request-key-0001").Count(&rejectedIdempotencyCount).Error; err != nil || rejectedIdempotencyCount != 0 {
		t.Fatalf("idempotency reservation committed without audit: count=%d err=%v", rejectedIdempotencyCount, err)
	}
	if err := tx.Model(&authorizationOutboxModel{}).Where("id = ?", change.ID).Count(&rejectedOutboxCount).Error; err != nil || rejectedOutboxCount != 0 {
		t.Fatalf("authorization change committed without audit: count=%d err=%v", rejectedOutboxCount, err)
	}
	if err := tx.Model(&auditEventModel{}).Where("id = ?", mismatched.ID).Count(&rejectedResourceAuditCount).Error; err != nil || rejectedResourceAuditCount != 0 {
		t.Fatalf("invalid resource audit committed: count=%d err=%v", rejectedResourceAuditCount, err)
	}
	createdAudit := mismatched
	createdAudit.ID = "created-event"
	createdAudit.Action = audit.ResourceCreated
	createdAudit.TargetType = resource.Domain
	if _, _, err := txStore.CreateResource(ctx, resource, change, createdAudit, ports.Idempotency{PrincipalID: resource.OwnerUserID, Operation: "resource.create:habit", Key: "request-key-0002", RequestHash: make([]byte, 32)}); err != nil {
		t.Fatal(err)
	}
	wrongUpdate := createdAudit
	wrongUpdate.ID = "wrong-update"
	wrongUpdate.Action = audit.ResourceUpdated
	wrongUpdate.TargetID = "other-resource"
	if _, err := txStore.UpdateResource(ctx, resource.Domain, resource.ID, "Changed", wrongUpdate); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("mismatched update audit accepted: %v", err)
	}
	unchanged, err := txStore.GetResource(ctx, resource.Domain, resource.ID)
	if err != nil || unchanged.Name != resource.Name {
		t.Fatalf("update committed without coherent audit: %+v %v", unchanged, err)
	}
	var rejectedUpdateAuditCount int64
	if err := tx.Model(&auditEventModel{}).Where("id = ?", wrongUpdate.ID).Count(&rejectedUpdateAuditCount).Error; err != nil || rejectedUpdateAuditCount != 0 {
		t.Fatalf("invalid update audit committed: count=%d err=%v", rejectedUpdateAuditCount, err)
	}
	wrongDelete := createdAudit
	wrongDelete.ID = "wrong-delete"
	wrongDelete.Action = audit.ResourceDeleted
	wrongDelete.OwnerUserID = "audit-actor"
	deleteChange := change
	deleteChange.ID = "delete-change"
	if err := txStore.DeleteResource(ctx, resource.Domain, resource.ID, deleteChange, wrongDelete); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("mismatched delete audit accepted: %v", err)
	}
	if _, err := txStore.GetResource(ctx, resource.Domain, resource.ID); err != nil {
		t.Fatalf("delete committed without coherent audit: %v", err)
	}
	var rejectedDeleteOutboxCount, rejectedDeleteAuditCount int64
	if err := tx.Model(&authorizationOutboxModel{}).Where("id = ?", deleteChange.ID).Count(&rejectedDeleteOutboxCount).Error; err != nil || rejectedDeleteOutboxCount != 0 {
		t.Fatalf("authorization delete committed without audit: count=%d err=%v", rejectedDeleteOutboxCount, err)
	}
	if err := tx.Model(&auditEventModel{}).Where("id = ?", wrongDelete.ID).Count(&rejectedDeleteAuditCount).Error; err != nil || rejectedDeleteAuditCount != 0 {
		t.Fatalf("invalid delete audit committed: count=%d err=%v", rejectedDeleteAuditCount, err)
	}
	if err := txStore.AppendAuditEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	secondEvent := event
	secondEvent.ID = "audit-event-z"
	if err := txStore.AppendAuditEvent(ctx, secondEvent); err != nil {
		t.Fatal(err)
	}
	page := ports.PageRequest{Limit: 10, Snapshot: event.OccurredAt.Add(time.Second)}
	for userID, expected := range map[string]int{"audit-owner": 3, "audit-actor": 2} {
		visible, err := txStore.ListAuditEvents(ctx, userID, page)
		if err != nil || len(visible.Events) != expected {
			t.Fatalf("event not visible to %s: %+v %v", userID, visible, err)
		}
	}
	hidden, err := txStore.ListAuditEvents(ctx, "audit-stranger", page)
	if err != nil || len(hidden.Events) != 0 {
		t.Fatalf("event leaked cross-user: %+v %v", hidden, err)
	}
	firstPage, err := txStore.ListAuditEvents(ctx, "audit-owner", ports.PageRequest{Limit: 1, Snapshot: page.Snapshot})
	if err != nil || len(firstPage.Events) != 1 || !firstPage.HasMore {
		t.Fatalf("first same-time audit page is invalid: %+v %v", firstPage, err)
	}
	secondPage, err := txStore.ListAuditEvents(ctx, "audit-owner", ports.PageRequest{Limit: 1, Snapshot: page.Snapshot, AfterID: firstPage.Events[0].ID, AfterCreated: firstPage.Events[0].OccurredAt})
	if err != nil || len(secondPage.Events) != 1 || secondPage.Events[0].ID == firstPage.Events[0].ID {
		t.Fatalf("same-time audit traversal skipped or duplicated an event: %+v %v", secondPage, err)
	}
	if err := tx.SavePoint("before_update").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&auditEventModel{}).Where("id = ?", event.ID).Update("outcome", audit.Denied).Error; err == nil {
		t.Fatal("database accepted audit update")
	}
	if err := tx.RollbackTo("before_update").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.SavePoint("before_delete").Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Where("id = ?", event.ID).Delete(&auditEventModel{}).Error; err == nil {
		t.Fatal("database accepted audit delete")
	}
	if err := tx.RollbackTo("before_delete").Error; err != nil {
		t.Fatal(err)
	}
}

func TestAuthorizationFailuresAreBoundedAndBlockLaterResourceWork(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resourceID := "bounded-failure-" + newTestID()
	ownerID := "bounded-owner-" + newTestID()
	now := time.Now().UTC()
	if err := store.DB.WithContext(ctx).Create(&userModel{ID: ownerID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Delete(&authorizationOutboxModel{}).Error
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Delete(&authorizationResourceLockModel{}).Error
		_ = store.DB.WithContext(ctx).Where("id = ?", ownerID).Delete(&userModel{}).Error
	})
	for _, row := range []authorizationOutboxModel{
		{ID: "bounded-touch-" + newTestID(), ResourceType: "resource", ResourceID: resourceID, Relation: "owner", SubjectType: "user", SubjectID: ownerID, OwnerUserID: ownerID, ActorUserID: ownerID, Operation: ports.AuthorizationTouch, CreatedAt: now},
		{ID: "bounded-delete-" + newTestID(), ResourceType: "resource", ResourceID: resourceID, Relation: "owner", SubjectType: "user", SubjectID: ownerID, OwnerUserID: ownerID, ActorUserID: ownerID, Operation: ports.AuthorizationDelete, CreatedAt: now.Add(time.Second)},
	} {
		if err := store.DB.WithContext(ctx).Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}

	for attempt := 1; attempt <= 3; attempt++ {
		claimed, err := store.ClaimAuthorizationChanges(ctx, "bounded-worker", time.Minute, 10)
		if err != nil || len(claimed) != 1 || claimed[0].Operation != ports.AuthorizationTouch {
			t.Fatalf("attempt %d claimed %+v, err=%v", attempt, claimed, err)
		}
		deadLettered, err := store.FailAuthorizationChange(ctx, claimed[0].ID, "bounded-worker", 3, "dependency_failure")
		if err != nil || deadLettered != (attempt == 3) {
			t.Fatalf("attempt %d deadLettered=%t err=%v", attempt, deadLettered, err)
		}
	}
	blocked, err := store.ClaimAuthorizationChanges(ctx, "later-worker", time.Minute, 10)
	if err != nil || len(blocked) != 0 {
		t.Fatalf("later DELETE bypassed the dead-lettered TOUCH: %+v err=%v", blocked, err)
	}
	page, err := store.ListAuthorizationDeadLetters(ctx, ownerID, ports.PageRequest{Limit: 10, Snapshot: time.Now().UTC().Add(time.Second)})
	if err != nil || len(page.Items) != 1 || page.Items[0].Attempts != 3 || page.Items[0].FailureCode != "dependency_failure" {
		t.Fatalf("bounded failure was not owner-recoverable: %+v err=%v", page, err)
	}
}

func TestAuthorizationOutboxProducersSerializeBeforePublishingOrder(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	sqlDB, err := store.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	secondConnection, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = secondConnection.Close() })
	observerConnection, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = observerConnection.Close() })
	var secondBackendPID int
	if err := secondConnection.QueryRowContext(ctx, "SELECT pg_catalog.pg_backend_pid()").Scan(&secondBackendPID); err != nil {
		t.Fatal(err)
	}
	resourceID := "producer-order-" + newTestID()
	t.Cleanup(func() {
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Delete(&authorizationOutboxModel{}).Error
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Delete(&authorizationResourceLockModel{}).Error
	})
	firstTransaction := store.DB.WithContext(ctx).Begin()
	if firstTransaction.Error != nil {
		t.Fatal(firstTransaction.Error)
	}
	t.Cleanup(func() { _ = firstTransaction.Rollback().Error })
	first := authorizationOutboxModel{ID: "producer-touch-" + newTestID(), ResourceType: "resource", ResourceID: resourceID, Relation: "owner", SubjectType: "user", SubjectID: "owner", OwnerUserID: "owner", ActorUserID: "owner", Operation: ports.AuthorizationTouch, CreatedAt: time.Now().Add(time.Hour)}
	if err := firstTransaction.Create(&first).Error; err != nil {
		t.Fatal(err)
	}

	secondDone := make(chan error, 1)
	second := authorizationOutboxModel{ID: "producer-delete-" + newTestID(), ResourceType: "resource", ResourceID: resourceID, Relation: "owner", SubjectType: "user", SubjectID: "owner", OwnerUserID: "owner", ActorUserID: "owner", Operation: ports.AuthorizationDelete, CreatedAt: time.Now().Add(-time.Hour)}
	go func() {
		_, err := secondConnection.ExecContext(ctx, `
			INSERT INTO authorization_outbox_models (
				id, resource_type, resource_id, relation, subject_type, subject_id, owner_user_id, actor_user_id, operation, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
			second.ID, second.ResourceType, second.ResourceID, second.Relation,
			second.SubjectType, second.SubjectID, second.OwnerUserID, second.ActorUserID,
			second.Operation, second.CreatedAt,
		)
		secondDone <- err
	}()
	waitForPostgresLock(t, ctx, observerConnection, secondBackendPID, secondDone)
	if err := firstTransaction.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-secondDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatalf("second producer remained blocked after the first commit: %v", context.Cause(ctx))
	}

	var rows []authorizationOutboxModel
	if err := store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].ID != first.ID || rows[1].ID != second.ID || !rows[0].CreatedAt.Before(rows[1].CreatedAt) {
		t.Fatalf("serialized producer order = %+v", rows)
	}
}

func waitForPostgresLock(t *testing.T, ctx context.Context, observer *sql.Conn, backendPID int, completed <-chan error) {
	t.Helper()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		var state, waitEventType, waitEvent sql.NullString
		err := observer.QueryRowContext(ctx,
			"SELECT state, wait_event_type, wait_event FROM pg_catalog.pg_stat_activity WHERE pid = $1", backendPID).Scan(&state, &waitEventType, &waitEvent)
		if err != nil {
			t.Fatalf("observe second producer backend %d: %v", backendPID, err)
		}
		if state.String == "active" && waitEventType.String == "Lock" && waitEvent.Valid {
			return
		}
		select {
		case err := <-completed:
			t.Fatalf("second producer completed without an observed database lock wait: %v", err)
		case <-ctx.Done():
			t.Fatalf("second producer did not enter a database lock wait: %v (state=%q wait_event_type=%q wait_event=%q)", context.Cause(ctx), state.String, waitEventType.String, waitEvent.String)
		case <-poll.C:
		}
	}
}

func TestAuthorizationOutboxOrderAdvancesPastCompletedHistory(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	resourceID := "completed-order-" + newTestID()
	ownerID := "completed-owner-" + newTestID()
	now := time.Now().UTC()
	if err := store.DB.WithContext(ctx).Create(&userModel{ID: ownerID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Delete(&authorizationOutboxModel{}).Error
		_ = store.DB.WithContext(ctx).Where("resource_type = ? AND resource_id = ?", "resource", resourceID).Delete(&authorizationResourceLockModel{}).Error
		_ = store.DB.WithContext(ctx).Where("id = ?", ownerID).Delete(&userModel{}).Error
	})

	first := authorizationOutboxModel{ID: "completed-first-" + newTestID(), ResourceType: "resource", ResourceID: resourceID, Relation: "owner", SubjectType: "user", SubjectID: ownerID, OwnerUserID: ownerID, ActorUserID: ownerID, Operation: ports.AuthorizationTouch, CreatedAt: now}
	if err := store.DB.WithContext(ctx).Create(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.WithContext(ctx).Model(&authorizationOutboxModel{}).Where("id = ?", first.ID).Updates(map[string]any{
		"created_at":   gorm.Expr("CURRENT_TIMESTAMP + INTERVAL '1 hour'"),
		"completed_at": gorm.Expr("CURRENT_TIMESTAMP"),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.WithContext(ctx).Where("id = ?", first.ID).First(&first).Error; err != nil {
		t.Fatal(err)
	}

	second := authorizationOutboxModel{ID: "completed-second-" + newTestID(), ResourceType: "resource", ResourceID: resourceID, Relation: "owner", SubjectType: "user", SubjectID: ownerID, OwnerUserID: ownerID, ActorUserID: ownerID, Operation: ports.AuthorizationDelete, CreatedAt: now.Add(-time.Hour)}
	if err := store.DB.WithContext(ctx).Create(&second).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.WithContext(ctx).Where("id = ?", second.ID).First(&second).Error; err != nil {
		t.Fatal(err)
	}
	if !first.CreatedAt.Before(second.CreatedAt) {
		t.Fatalf("later outbox timestamp %v did not advance past completed history %v", second.CreatedAt, first.CreatedAt)
	}
}

func TestAuthorizationOutboxOrderingMigrationIsInvokerSafeAndIndexed(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var function struct {
		SecurityDefiner bool   `gorm:"column:security_definer"`
		FixedSearchPath bool   `gorm:"column:fixed_search_path"`
		Configuration   string `gorm:"column:configuration"`
		Definition      string `gorm:"column:definition"`
	}
	if err := store.DB.WithContext(ctx).
		Table("pg_catalog.pg_proc AS p").
		Select(`p.prosecdef AS security_definer,
			COALESCE((SELECT count(*) = 1 AND bool_and(setting = 'search_path=pg_catalog, public') FROM unnest(p.proconfig) AS configured(setting) WHERE setting LIKE 'search_path=%'), false) AS fixed_search_path,
			COALESCE(array_to_string(p.proconfig, ','), '') AS configuration,
			pg_get_functiondef(p.oid) AS definition`).
		Joins("JOIN pg_catalog.pg_namespace AS n ON n.oid = p.pronamespace").
		Where("n.nspname = ? AND p.proname = ?", "public", "order_authorization_outbox_change").
		Scan(&function).Error; err != nil {
		t.Fatal(err)
	}
	if function.SecurityDefiner {
		t.Fatal("authorization outbox ordering function must use invoker privileges")
	}
	if !function.FixedSearchPath {
		t.Fatalf("authorization outbox ordering function search_path = %q", function.Configuration)
	}
	for _, object := range []string{"public.authorization_resource_lock_models", "public.authorization_outbox_models"} {
		if !strings.Contains(function.Definition, object) {
			t.Fatalf("authorization outbox ordering function does not qualify %s", object)
		}
	}
	var indexes []struct {
		Name      string `gorm:"column:name"`
		Method    string `gorm:"column:method"`
		KeySpec   string `gorm:"column:key_spec"`
		Predicate string `gorm:"column:predicate"`
		Unique    bool   `gorm:"column:unique_index"`
	}
	// The column form of pg_get_indexdef identifies each key but may omit its
	// ordering. indoption is zero-based, and its low bit is the btree DESC flag.
	if err := store.DB.WithContext(ctx).
		Table("pg_catalog.pg_index AS i").
		Select(`index_class.relname AS name, access_method.amname AS method,
			string_agg(
				pg_get_indexdef(i.indexrelid, key.position, true) ||
				CASE WHEN (i.indoption[key.position - 1]::integer & 1) = 1 THEN ' DESC' ELSE '' END,
				'|' ORDER BY key.position
			) AS key_spec,
			COALESCE(pg_get_expr(i.indpred, i.indrelid, true), '') AS predicate,
			i.indisunique AS unique_index`).
		Joins("JOIN pg_catalog.pg_class AS table_class ON table_class.oid = i.indrelid").
		Joins("JOIN pg_catalog.pg_namespace AS namespace ON namespace.oid = table_class.relnamespace").
		Joins("JOIN pg_catalog.pg_class AS index_class ON index_class.oid = i.indexrelid").
		Joins("JOIN pg_catalog.pg_am AS access_method ON access_method.oid = index_class.relam").
		Joins("JOIN LATERAL generate_series(1, i.indnkeyatts) AS key(position) ON true").
		Where("namespace.nspname = ? AND table_class.relname = ? AND index_class.relname IN ?", "public", "authorization_outbox_models", []string{"authorization_outbox_owner_dead_letter_idx", "authorization_outbox_resource_created_idx"}).
		Group("index_class.relname, access_method.amname, i.indisunique, pg_get_expr(i.indpred, i.indrelid, true)").
		Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]struct {
		Method    string
		KeySpec   string
		Predicate string
		Unique    bool
	}, len(indexes))
	for _, index := range indexes {
		byName[index.Name] = struct {
			Method    string
			KeySpec   string
			Predicate string
			Unique    bool
		}{index.Method, index.KeySpec, index.Predicate, index.Unique}
	}
	expected := map[string]struct {
		Keys      string
		Predicate string
	}{
		"authorization_outbox_owner_dead_letter_idx": {Keys: "owner_user_id|dead_lettered_at DESC|id", Predicate: "dead_lettered_atISNOTNULLANDcompleted_atISNULL"},
		"authorization_outbox_resource_created_idx":  {Keys: "resource_type|resource_id|created_at DESC"},
	}
	for name, contract := range expected {
		index, ok := byName[name]
		if !ok {
			t.Fatalf("authorization outbox index %s is missing", name)
		}
		predicate := strings.NewReplacer(" ", "", "(", "", ")", "").Replace(index.Predicate)
		if index.Method != "btree" || index.Unique || index.KeySpec != contract.Keys || predicate != contract.Predicate {
			t.Fatalf("authorization outbox index %s has method=%q unique=%t keys=%q predicate=%q", name, index.Method, index.Unique, index.KeySpec, index.Predicate)
		}
	}
}

func TestAuthorizationCompletionAttributesCrossUserRelationshipToOwnerAndActor(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	tx := store.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	store = &Store{DB: tx}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	ownerID, actorID, viewerID := "owner-"+uuid.NewString(), "actor-"+uuid.NewString(), "viewer-"+uuid.NewString()
	for _, id := range []string{ownerID, actorID, viewerID} {
		if err := tx.Create(&userModel{ID: id, Email: id + "@example.com", Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	row := authorizationOutboxModel{ID: uuid.NewString(), ResourceType: "resource", ResourceID: uuid.NewString(), Relation: "viewer", SubjectType: "user", SubjectID: viewerID, OwnerUserID: ownerID, ActorUserID: actorID, Operation: ports.AuthorizationTouch, CreatedAt: now}
	if err := tx.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	claimed, err := store.ClaimAuthorizationChanges(ctx, "sharing-worker", time.Minute, 10)
	if err != nil || len(claimed) != 1 || claimed[0].ID != row.ID {
		t.Fatalf("cross-user relationship was not claimed: changes=%+v err=%v", claimed, err)
	}
	event := audit.Event{ID: uuid.NewString(), OwnerUserID: ownerID, ActorUserID: actorID, Action: audit.AuthorizationApplied, TargetType: row.ResourceType, TargetID: row.ResourceID, Outcome: audit.Succeeded, CorrelationID: uuid.NewString(), OccurredAt: now}
	if err := store.CompleteAuthorizationChangeWithAudit(ctx, row.ID, "sharing-worker", event); err != nil {
		t.Fatalf("cross-user relationship could not complete: %v", err)
	}
	var persisted auditEventModel
	if err := tx.Where("id = ?", event.ID).First(&persisted).Error; err != nil || persisted.OwnerUserID != ownerID || persisted.ActorUserID != actorID {
		t.Fatalf("relationship audit attribution drifted: %+v err=%v", persisted, err)
	}
}

func TestHealthRejectsBehindOrDirtyMigrationLedger(t *testing.T) {
	if *migrationPostgresTestDSN == "" {
		t.Skip("-migration-database-dsn is required for migration-ledger integration")
	}
	store, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	var ledger schemaMigrationModel
	if err := store.DB.WithContext(ctx).Table("schema_migrations").First(&ledger).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.DB.WithContext(ctx).Table("schema_migrations").Where("version IN ?", []uint{2, ledger.Version}).Updates(map[string]any{"version": ledger.Version, "dirty": ledger.Dirty}).Error; err != nil {
			t.Error(err)
		}
	})
	if err := store.DB.WithContext(ctx).Table("schema_migrations").Where("version = ?", ledger.Version).Updates(map[string]any{"version": 2, "dirty": false}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Health(ctx); err == nil {
		t.Fatal("health accepted an outdated migration ledger")
	}
	if err := store.DB.WithContext(ctx).Table("schema_migrations").Where("version = ?", 2).Updates(map[string]any{"version": ledger.Version, "dirty": true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.Health(ctx); err == nil {
		t.Fatal("health accepted a dirty migration ledger")
	}
	if err := store.DB.WithContext(ctx).Table("schema_migrations").Where("version = ?", ledger.Version).Update("dirty", false).Error; err != nil {
		t.Fatal(err)
	}
	if err := store.DB.WithContext(ctx).Table("schema_migrations").Create(&schemaMigrationModel{Version: 999, Dirty: false}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.DB.WithContext(ctx).Table("schema_migrations").Where("version = ?", 999).Delete(&schemaMigrationModel{}).Error; err != nil {
			t.Error(err)
		}
	})
	if err := store.Health(ctx); err == nil {
		t.Fatal("health accepted multiple migration ledger rows")
	}
}

func TestAuthorizationResourceWritesAreSerializedAcrossConnections(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("-database-dsn is required for PostgreSQL integration")
	}
	store, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	enteredFirst := make(chan struct{})
	releaseFirst := make(chan struct{})
	enteredSecond := make(chan struct{})
	firstDone := make(chan error, 1)
	secondDone := make(chan error, 1)
	go func() {
		firstDone <- store.WithinResource(ctx, "resource", "serialized-test", func(context.Context) error { close(enteredFirst); <-releaseFirst; return nil })
	}()
	select {
	case <-enteredFirst:
	case <-time.After(5 * time.Second):
		t.Fatal("first writer did not acquire serializer")
	}
	go func() {
		secondDone <- store.WithinResource(ctx, "resource", "serialized-test", func(context.Context) error { close(enteredSecond); return nil })
	}()
	select {
	case <-enteredSecond:
		t.Fatal("second writer entered before first released resource lock")
	case <-time.After(250 * time.Millisecond):
	}
	close(releaseFirst)
	if err := <-firstDone; err != nil {
		t.Fatal(err)
	}
	select {
	case <-enteredSecond:
	case <-time.After(5 * time.Second):
		t.Fatal("second writer did not enter after release")
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
}
