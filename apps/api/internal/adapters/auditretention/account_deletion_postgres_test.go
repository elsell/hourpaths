package auditretention

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestDeletedAccountRetentionIsBoundedAndPreservesUnrelatedAudit(t *testing.T) {
	if *databaseDSN == "" || *retentionDSN == "" {
		t.Skip("database and retention DSNs required")
	}
	admin, err := gorm.Open(postgres.Open(*databaseDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	retention, err := gorm.Open(postgres.Open(*retentionDSN), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{DB: retention}
	// The suite uses a dedicated database; drain eligible rows from a prior run.
	for i := 0; ; i++ {
		n, err := runner.DeletedAccountBatch(context.Background(), 10000)
		if err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			break
		}
		if i >= 10 {
			t.Fatal("unexpected retention fixture volume")
		}
	}
	now := time.Now().UTC()
	expired, recent, other := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for owner, age := range map[string]time.Duration{expired: 29*24*time.Hour + time.Hour, recent: 28 * 24 * time.Hour} {
		if err := admin.Exec("INSERT INTO account_deletion_models(user_id,deleted_at,audit_event_id,receipt_hash) VALUES (?,?,?,?)", owner, now.Add(-age), uuid.NewString(), make([]byte, 32)).Error; err != nil {
			t.Fatal(err)
		}
	}
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for i, owner := range []string{expired, expired, recent, other} {
		if err := admin.Exec("INSERT INTO audit_event_models(id,owner_user_id,actor_user_id,action,target_type,target_id,outcome,correlation_id,occurred_at) VALUES (?,?,?,?,?,?,?,?,?)", ids[i], owner, owner, "resource.viewed", "account", owner, "succeeded", uuid.NewString(), now).Error; err != nil {
			t.Fatal(err)
		}
	}
	n, err := runner.DeletedAccountBatch(context.Background(), 1)
	if err != nil || n != 1 {
		t.Fatalf("first bounded batch: %d %v", n, err)
	}
	n, err = runner.DeletedAccountBatch(context.Background(), 1)
	if err != nil || n != 1 {
		t.Fatalf("second bounded batch: %d %v", n, err)
	}
	for i, id := range ids {
		var count int64
		if err := admin.Table("audit_event_models").Where("id = ?", id).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		want := int64(1)
		if i < 2 {
			want = 0
		}
		if count != want {
			t.Fatalf("audit %d: got %d want %d", i, count, want)
		}
	}
	if err := retention.Exec("DELETE FROM audit_event_models WHERE id = ?", ids[2]).Error; err == nil {
		t.Fatal("retention gained direct audit delete")
	}
	if err := retention.Exec("UPDATE account_deletion_retention_run_models SET deleted_count = 99").Error; err == nil {
		t.Fatal("retention summary is mutable")
	}
	if _, err := runner.DeletedAccountBatch(context.Background(), 10001); err == nil {
		t.Fatal("unbounded batch accepted")
	}
}
