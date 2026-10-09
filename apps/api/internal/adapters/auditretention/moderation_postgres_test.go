package auditretention

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestModerationRetentionPreservesOpenAndRecentCases(t *testing.T) {
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
	now := time.Now().UTC()
	expired, recent, active, appealed := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		id, state string
		closed    any
	}{{expired, "dismissed", now.Add(-89*24*time.Hour - time.Hour)}, {recent, "dismissed", now.Add(-88 * 24 * time.Hour)}, {active, "open", nil}, {appealed, "actioned", now.Add(-90 * 24 * time.Hour)}} {
		if err = admin.Exec(`INSERT INTO moderation_case_models(id,target_kind,target_id,reason,explanation,evidence,state,created_at,closed_at) VALUES (?,'profile','test-subject','something_else','','{}',?,?,?)`, tc.id, tc.state, now.Add(-100*24*time.Hour), tc.closed).Error; err != nil {
			t.Fatal(err)
		}
	}
	subject, notice := uuid.NewString(), uuid.NewString()
	if err = admin.Exec(`INSERT INTO user_models(id,status,display_name,created_at,updated_at) VALUES(?,'active','Retention test',?,?)`, subject, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.Exec(`INSERT INTO moderation_enforcement_models(id,case_id,subject_user_id,action,policy_reason,issued_at) VALUES(?,?,?,'warning','Policy',?)`, notice, appealed, subject, now.Add(-90*24*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.Exec(`INSERT INTO moderation_appeal_models(enforcement_id,id,explanation,submitted_at) VALUES(?,'retention-appeal-001','Please review',?)`, notice, now.Add(-89*24*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.Exec("SELECT moderation_list_cases(NULL)").Error; err == nil {
		t.Fatal("NULL bypassed bounded case listing")
	}
	var snapshot string
	if err = retention.Raw("SELECT moderation_read_case(?)", active).Scan(&snapshot).Error; err == nil {
		t.Fatal("retention role gained moderation review")
	}
	if err = admin.Raw("SELECT moderation_read_case(?)", active).Scan(&snapshot).Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.Exec("SELECT moderation_set_case_state(?, 'open','reviewing','Investigating reported content')", active).Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.Exec("SELECT moderation_set_case_state(?, 'open','dismissed','Stale decision')", active).Error; err == nil {
		t.Fatal("stale decision overwrote current review")
	}
	var events int64
	admin.Table("moderation_review_event_models").Where("case_id=?", active).Count(&events)
	if events != 2 {
		t.Fatalf("review events=%d", events)
	}
	// Other fixtures may be eligible. Each run still respects one combined bound.
	for i := 0; i < 100; i++ {
		n, err := runner.DeletedAccountBatch(context.Background(), 1)
		if err != nil || n > 1 {
			t.Fatalf("batch=%d err=%v", n, err)
		}
		if n == 0 {
			break
		}
		if i == 99 {
			t.Fatal("retention did not converge")
		}
	}
	for id, want := range map[string]int64{expired: 0, recent: 1, active: 1, appealed: 1} {
		var n int64
		if err = admin.Table("moderation_case_models").Where("id=?", id).Count(&n).Error; err != nil || n != want {
			t.Fatalf("case %s count=%d want=%d err=%v", id, n, want, err)
		}
	}
	if err = retention.Exec("DELETE FROM moderation_case_models WHERE id=?", active).Error; err == nil {
		t.Fatal("retention can bypass eligibility")
	}
}
