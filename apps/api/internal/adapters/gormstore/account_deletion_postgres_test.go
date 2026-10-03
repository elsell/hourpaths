package gormstore

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresAccountDeletionPreservesUnrelatedSharedActivity(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("runtime and migration PostgreSQL DSNs required")
	}
	runtime, err := openAccountDeletionStore(t, *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := openAccountDeletionStore(t, *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	owner, survivor := newTestID(), newTestID()
	cleanupDeletionOutbox(t, seed, owner)
	owned, joined := newTestID(), newTestID()
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, id := range []string{owner, survivor} {
		if err := seed.DB.Create(&userModel{ID: id, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []struct{ id, creator string }{{owned, owner}, {joined, survivor}} {
		if err := seed.DB.Table("path_models").Create(map[string]any{"id": path.id, "owner_user_id": path.creator, "name": "Deletion acceptance", "visibility": "private", "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
		for _, member := range []string{owner, survivor} {
			if err := seed.DB.Table("path_membership_models").Create(map[string]any{"path_id": path.id, "user_id": member, "role": "participant"}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	retainedActivity := newTestID()
	for _, activity := range []struct{ id, path, person string }{{newTestID(), owned, survivor}, {newTestID(), joined, owner}, {retainedActivity, joined, survivor}} {
		if err := seed.DB.Table("recorded_activity_models").Create(map[string]any{"id": activity.id, "path_id": activity.path, "participant_id": activity.person, "started_at": now.Add(-time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := seed.DB.Table("running_timer_models").Create(map[string]any{"id": newTestID(), "path_id": joined, "participant_id": owner, "started_at": now, "occurrence_time_zone": "Etc/UTC"}).Error; err != nil {
		t.Fatal(err)
	}
	event := audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceDeleted, TargetType: "account", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
	// A duplicate immutable audit ID forces failure after the removal work has
	// started. Neither personal data nor a deletion marker may escape rollback.
	if err := runtime.AppendAuditEvent(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	command := application.AccountDeletionCommand{UserID: owner, DeletedAt: now, Audit: event, NewID: newTestID, ReceiptHash: bytes.Repeat([]byte{1}, 32)}
	if err := runtime.DeleteAccount(context.Background(), command); err == nil {
		t.Fatal("deletion succeeded despite rejected audit evidence")
	}
	for _, check := range []struct {
		table, where string
		count        int64
	}{
		{"user_models", "id = ?", 1},
		{"path_models", "owner_user_id = ?", 1},
		{"running_timer_models", "participant_id = ?", 1},
		{"account_deletion_models", "user_id = ?", 0},
		{"authorization_outbox_models", "actor_user_id = ?", 0},
	} {
		var count int64
		if err := seed.DB.Table(check.table).Where(check.where, owner).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != check.count {
			t.Fatalf("failed deletion changed %s: got %d, want %d", check.table, count, check.count)
		}
	}
	event.ID = newTestID()
	if err := runtime.DeleteAccount(context.Background(), application.AccountDeletionCommand{UserID: owner, DeletedAt: now, Audit: event, NewID: newTestID, ReceiptHash: bytes.Repeat([]byte{1}, 32)}); err != nil {
		t.Fatal(err)
	}
	for _, receipt := range []struct {
		owner string
		hash  []byte
		at    time.Time
		want  bool
	}{
		{owner, bytes.Repeat([]byte{1}, 32), now, true},
		{survivor, bytes.Repeat([]byte{1}, 32), now, false},
		{owner, bytes.Repeat([]byte{2}, 32), now, false},
		{owner, bytes.Repeat([]byte{1}, 32), now.Add(30 * 24 * time.Hour), false},
	} {
		found, err := runtime.DeletionReceipt(context.Background(), receipt.owner, receipt.hash, receipt.at)
		if err != nil || found != receipt.want {
			t.Fatalf("deletion receipt found=%v want=%v error=%v", found, receipt.want, err)
		}
		viewed := audit.Event{ID: newTestID(), OwnerUserID: receipt.owner, ActorUserID: receipt.owner, Action: audit.ResourceViewed, TargetType: "account_deletion", TargetID: receipt.owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: receipt.at}
		confirmed, err := runtime.ConfirmDeletionReceipt(context.Background(), receipt.owner, receipt.hash, receipt.at, viewed)
		if err != nil || confirmed != receipt.want {
			t.Fatalf("atomic receipt confirmed=%v want=%v error=%v", confirmed, receipt.want, err)
		}
		var audited int64
		if err := seed.DB.Model(&auditEventModel{}).Where("id = ?", viewed.ID).Count(&audited).Error; err != nil {
			t.Fatal(err)
		}
		if (audited == 1) != receipt.want {
			t.Fatalf("receipt audit count=%d want confirmation=%v", audited, receipt.want)
		}
		if receipt.want {
			confirmed, err = runtime.ConfirmDeletionReceipt(context.Background(), receipt.owner, receipt.hash, receipt.at, viewed)
			if err == nil || confirmed {
				t.Fatal("receipt succeeded despite duplicate audit rejection")
			}
		}
	}
	for _, check := range []struct {
		table, where string
		args         []any
		count        int64
	}{
		{"user_models", "id = ?", []any{owner}, 0},
		{"path_models", "id = ?", []any{owned}, 0},
		{"recorded_activity_models", "participant_id = ? OR path_id = ?", []any{owner, owned}, 0},
		{"running_timer_models", "participant_id = ?", []any{owner}, 0},
		{"path_membership_models", "user_id = ?", []any{owner}, 0},
		{"recorded_activity_models", "id = ? AND participant_id = ?", []any{retainedActivity, survivor}, 1},
		{"path_models", "id = ?", []any{joined}, 1},
		{"audit_event_models", "id = ?", []any{event.ID}, 1},
	} {
		var count int64
		if err := seed.DB.Table(check.table).Where(check.where, check.args...).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != check.count {
			t.Errorf("%s: got %d rows, want %d", check.table, count, check.count)
		}
	}
}

func TestPostgresConcurrentFirstSignInDoesNotLeaveUnusedAccounts(t *testing.T) {
	if *postgresTestDSN == "" {
		t.Skip("runtime PostgreSQL DSN required")
	}
	runtime, err := openAccountDeletionStore(t, *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	claims := ports.Claims{Issuer: "https://account-deletion.example", Subject: newTestID(), DisplayName: "concurrent-" + newTestID()}
	hint := identity.UserID(claims.Issuer, claims.Subject)
	var accounts [2]identity.User
	var failures [2]error
	start := make(chan struct{})
	var group sync.WaitGroup
	for index := range accounts {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-start
			now := time.Now().UTC()
			accounts[index], failures[index] = runtime.ResolveOrCreate(context.Background(), claims, identityMutationAudit(newTestID(), audit.UserProvisioned, hint, now), identityMutationAudit(newTestID(), audit.UserProfileSynchronized, hint, now), audit.Event{}, true)
		}(index)
	}
	close(start)
	group.Wait()
	for _, err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if accounts[0].ID == "" || accounts[0].ID != accounts[1].ID {
		t.Fatalf("concurrent identities diverged: %+v", accounts)
	}
	var count int64
	if err := runtime.DB.Model(&userModel{}).Where("display_name = ?", claims.DisplayName).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("concurrent provisioning retained %d accounts", count)
	}
}

func TestPostgresDeletedIdentityCreatesAFreshAccountAndKeepsItsAuditOwner(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("runtime and migration PostgreSQL DSNs required")
	}
	runtime, err := openAccountDeletionStore(t, *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := openAccountDeletionStore(t, *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	claims := ports.Claims{Issuer: "https://account-deletion.example", Subject: newTestID(), DisplayName: "New profile"}
	oldID := identity.UserID(claims.Issuer, claims.Subject)
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := seed.DB.Create(&userModel{ID: oldID, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := seed.DB.Create(&identityModel{Issuer: claims.Issuer, Subject: claims.Subject, UserID: oldID}).Error; err != nil {
		t.Fatal(err)
	}
	deleted := audit.Event{ID: newTestID(), OwnerUserID: oldID, ActorUserID: oldID, Action: audit.ResourceDeleted, TargetType: "account", TargetID: oldID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
	if err := runtime.DeleteAccount(context.Background(), application.AccountDeletionCommand{UserID: oldID, DeletedAt: now, Audit: deleted, NewID: newTestID, ReceiptHash: bytes.Repeat([]byte{1}, 32)}); err != nil {
		t.Fatal(err)
	}
	provisioned := identityMutationAudit(newTestID(), audit.UserProvisioned, oldID, now)
	synchronized := identityMutationAudit(newTestID(), audit.UserProfileSynchronized, oldID, now)
	renewed, err := runtime.ResolveOrCreate(context.Background(), claims, provisioned, synchronized, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.ID == oldID || renewed.ID == "" || renewed.Status != identity.StatusProvisional {
		t.Fatalf("deleted account identity reused: %+v", renewed)
	}
	claims.DisplayName = "Updated provider seed"
	returned, err := runtime.ResolveOrCreate(context.Background(), claims, provisioned, synchronized, audit.Event{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if returned.ID != renewed.ID {
		t.Fatal("returning identity changed accounts")
	}
	var count int64
	if err := seed.DB.Table("audit_event_models").Where("id IN ? AND owner_user_id = ? AND actor_user_id = ? AND target_id = ?", []string{provisioned.ID, synchronized.ID}, renewed.ID, renewed.ID, renewed.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("new-account audit used the deleted account identity")
	}
}

func TestPostgresDeletionRestoreRemovesProvisionalBackupAndIsIdempotent(t *testing.T) {
	if *migrationPostgresTestDSN == "" {
		t.Skip("migration PostgreSQL DSN required")
	}
	store, err := openAccountDeletionStore(t, *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	owner := newTestID()
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.DB.Create(&userModel{ID: owner, Status: identity.StatusProvisional, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	records := []application.DeletionRecord{{UserID: owner, DeletedAt: now, AuditEventID: newTestID(), ReceiptHash: strings.Repeat("01", 32)}}
	if err := application.ReapplyDeletionRecords(context.Background(), records, store, newTestID); err != nil {
		t.Fatal(err)
	}
	if err := application.ReapplyDeletionRecords(context.Background(), records, store, newTestID); err != nil {
		t.Fatalf("repeat: %v", err)
	}
	// A recovery may reintroduce rows while keeping newer deletion evidence.
	if err := store.DB.Create(&userModel{ID: owner, Status: identity.StatusProvisional, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := application.ReapplyDeletionRecords(context.Background(), records, store, newTestID); err != nil {
		t.Fatalf("resurrected row: %v", err)
	}

	var count int64
	if err := store.DB.Model(&userModel{}).Where("id = ?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("restored deleted account remains: %d %v", count, err)
	}
	confirmed, err := store.DeletionReceipt(context.Background(), owner, bytes.Repeat([]byte{1}, 32), now)
	if err != nil || !confirmed {
		t.Fatalf("receipt lost during restore: %v %v", confirmed, err)
	}
}

func TestPostgresDeletionRetirementRequiresCompletedRemoval(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL required")
	}
	runtime, err := openAccountDeletionStore(t, *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	seed, err := openAccountDeletionStore(t, *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, scenario := range []struct {
		name          string
		age           time.Duration
		user, pending bool
		want          bool
	}{
		{"complete", 29*24*time.Hour + time.Hour, false, false, true},
		{"recent", time.Hour, false, false, false},
		{"account remains", 30 * 24 * time.Hour, true, false, false},
		{"revocation pending", 30 * 24 * time.Hour, false, true, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			owner := newTestID()
			cleanupDeletionOutbox(t, seed, owner)
			at := now.Add(-scenario.age)
			hash := bytes.Repeat([]byte{1}, 32)
			if err := seed.DB.Create(&accountDeletionModel{UserID: owner, DeletedAt: at, AuditEventID: newTestID(), ReceiptHash: hash}).Error; err != nil {
				t.Fatal(err)
			}
			if scenario.user {
				if err := seed.DB.Create(&userModel{ID: owner, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
					t.Fatal(err)
				}
			}
			if scenario.pending {
				if err := seed.DB.Exec("INSERT INTO authorization_outbox_models(id,resource_type,resource_id,relation,subject_type,subject_id,owner_user_id,actor_user_id,operation,created_at) VALUES (?,'resource',?,'owner','user',?,?,?,'delete',?)", newTestID(), newTestID(), owner, owner, owner, now).Error; err != nil {
					t.Fatal(err)
				}
			}
			record := application.DeletionRecord{UserID: owner, DeletedAt: at, ReceiptHash: strings.Repeat("01", 32)}
			wrong := record
			wrong.ReceiptHash = strings.Repeat("02", 32)
			if ready, err := runtime.PrepareDeletionRetirement(context.Background(), wrong); err != nil || ready {
				t.Fatal("wrong capability accepted", ready, err)
			}
			ready, err := runtime.PrepareDeletionRetirement(context.Background(), record)
			if err != nil || ready != scenario.want {
				t.Fatalf("ready=%v want=%v error=%v", ready, scenario.want, err)
			}
			if scenario.want {
				if found, err := runtime.DeletionReceipt(context.Background(), owner, hash, now); err != nil || found {
					t.Fatal("retired receipt remained usable", found, err)
				}
				event := audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceViewed, TargetType: "account_deletion", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
				if found, err := runtime.ConfirmDeletionReceipt(context.Background(), owner, hash, now, event); err != nil || found {
					t.Fatal("retired receipt audited", found, err)
				}
			}
		})
	}
	if err := runtime.DB.Exec("UPDATE account_deletion_models SET journal_retired_at=clock_timestamp()").Error; err == nil {
		t.Fatal("runtime gained direct marker update")
	}
}

func openAccountDeletionStore(t *testing.T, dsn string) (*Store, error) {
	t.Helper()
	store, err := Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db, err := store.DB.DB()
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return store, nil
}
func cleanupDeletionOutbox(t *testing.T, store *Store, owner string) {
	t.Helper()
	t.Cleanup(func() {
		if err := store.DB.Where("owner_user_id = ? OR actor_user_id = ?", owner, owner).Delete(&authorizationOutboxModel{}).Error; err != nil {
			t.Error(err)
		}
	})
}
