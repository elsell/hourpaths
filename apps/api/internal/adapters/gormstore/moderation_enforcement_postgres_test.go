package gormstore

import (
	"context"
	"errors"
	"testing"
	"time"

	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresAppealOwnershipReplayAndAtomicAudit(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL integration DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Truncate(time.Microsecond)
	owner, other := newTestID(), newTestID()
	for _, id := range []string{owner, other} {
		if err = admin.DB.Create(&userModel{ID: id, Status: identity.StatusActive, DisplayName: "Moderation test", CreatedAt: at, UpdatedAt: at}).Error; err != nil {
			t.Fatal(err)
		}
	}
	caseID := newTestID()
	if err = admin.DB.Exec(`INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'profile',?,'spam_or_scam','','{}',?)`, caseID, other, owner, owner, at).Error; err != nil {
		t.Fatal(err)
	}
	issuedID := newTestID()
	if err = runtime.DB.Exec(`SELECT moderation_warn_user(?,?,?)`, caseID, issuedID, "Policy reason").Error; err == nil {
		t.Fatal("runtime can invoke operational warning")
	}
	if err = admin.DB.Exec(`SELECT moderation_warn_user(?,?,?)`, caseID, issuedID, "Policy reason").Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.DB.Exec(`SELECT moderation_warn_user(?,?,?)`, caseID, newTestID(), "Another reason").Error; err == nil {
		t.Fatal("closed case action repeated")
	}
	var issued enforcementModel
	if err = admin.DB.Where("id=?", issuedID).Take(&issued).Error; err != nil || issued.SubjectUserID != owner || issued.PolicyReason != "Policy reason" {
		t.Fatal("notice missing", err)
	}
	noticeID := issuedID
	appealAt := time.Now().UTC().Truncate(time.Microsecond)
	r := NewModerationRepository(runtime.DB)
	event := func(actor string, action audit.Action, targetType, targetID string) audit.Event {
		return audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: action, TargetType: targetType, TargetID: targetID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: appealAt}
	}
	n, err := r.ReadNotice(context.Background(), owner, noticeID, event(owner, audit.ResourceViewed, "enforcement_notice", noticeID))
	if err != nil || n.Decision.SubjectUserID != owner {
		t.Fatalf("notice=%+v err=%v", n, err)
	}
	if _, err = r.ReadNotice(context.Background(), other, noticeID, event(other, audit.ResourceViewed, "enforcement_notice", noticeID)); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("cross-account read", err)
	}
	proposed, err := n.Decision.NewAppeal("appeal-operation-001", "Context", appealAt)
	if err != nil {
		t.Fatal(err)
	}
	c := app.AppealCommand{OwnerID: owner, Appeal: proposed, Audit: event(owner, audit.ResourceCreated, "moderation_appeal", noticeID)}
	first, err := r.SubmitAppeal(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	again, err := r.SubmitAppeal(context.Background(), c)
	if err != nil || again.ID != first.ID || again.EnforcementID != first.EnforcementID || again.Explanation != first.Explanation || !again.SubmittedAt.Equal(first.SubmittedAt) {
		t.Fatal("lost-response replay", err)
	}
	var pending int
	if err = runtime.DB.Exec(`SELECT moderation_list_appeals(20)`).Error; err == nil {
		t.Fatal("runtime can list private appeals")
	}
	if err = runtime.DB.Exec(`SELECT moderation_read_appeal(?)`, noticeID).Error; err == nil {
		t.Fatal("runtime can read operational appeal")
	}
	if err = admin.DB.Raw(`SELECT count(*) FROM jsonb_array_elements(moderation_list_appeals(100)) p WHERE p->>'notice_id'=?`, noticeID).Scan(&pending).Error; err != nil || pending != 1 {
		t.Fatal("pending appeal missing from operator queue", err, pending)
	}
	var explanation string
	if err = admin.DB.Raw(`SELECT moderation_read_appeal(?)->'appeal'->>'explanation'`, noticeID).Scan(&explanation).Error; err != nil || explanation != "Context" {
		t.Fatal("operator cannot inspect original appeal", err)
	}
	if err = runtime.DB.Exec(`SELECT moderation_decide_warning_appeal(?,?,?,?)`, noticeID, "reversed", "Context considered", "Only available reviewer").Error; err == nil {
		t.Fatal("runtime can decide appeal")
	}
	if err = admin.DB.Exec(`SELECT moderation_decide_warning_appeal(?,?,?,?)`, noticeID, "reversed", "Context considered", "").Error; err == nil {
		t.Fatal("same reviewer without recorded exception")
	}
	if err = admin.DB.Exec(`SELECT moderation_decide_warning_appeal(?,?,?,?)`, noticeID, "reversed", "Context considered", "Only available reviewer").Error; err != nil {
		t.Fatal(err)
	}
	resolved, err := r.ReadNotice(context.Background(), owner, noticeID, event(owner, audit.ResourceViewed, "enforcement_notice", noticeID))
	if err != nil || resolved.Appeal == nil || resolved.Appeal.Outcome != "reversed" || resolved.Appeal.Explanation != "Context" || resolved.Decision.PolicyReason != "Policy reason" {
		t.Fatal("final decision lost original evidence", err)
	}
	if err = admin.DB.Exec(`SELECT moderation_decide_warning_appeal(?,?,?,?)`, noticeID, "upheld", "Changed mind", "Only available reviewer").Error; err == nil {
		t.Fatal("final decision changed")
	}
	c.Appeal.Explanation = "Changed"
	if _, err = r.SubmitAppeal(context.Background(), c); !errors.Is(err, ports.ErrConflict) {
		t.Fatal("appeal overwritten", err)
	}
	c.OwnerID = other
	c.Audit = event(other, audit.ResourceCreated, "moderation_appeal", noticeID)
	if _, err = r.SubmitAppeal(context.Background(), c); !errors.Is(err, ports.ErrNotFound) {
		t.Fatal("cross-account write", err)
	}
	secondID := newTestID()
	if err = admin.DB.Exec(`INSERT INTO moderation_enforcement_models(id,subject_user_id,action,policy_reason,issued_at) VALUES(?,?,'warning','Policy',?)`, secondID, owner, at).Error; err != nil {
		t.Fatal(err)
	}
	c.OwnerID = owner
	c.Appeal.EnforcementID = secondID
	c.Audit = event(owner, audit.ResourceCreated, "moderation_appeal", secondID)
	if err = runtime.AppendAuditEvent(context.Background(), c.Audit); err != nil {
		t.Fatal(err)
	}
	if _, err = r.SubmitAppeal(context.Background(), c); err == nil {
		t.Fatal("mutation accepted duplicate audit")
	}
	var count int64
	if err = admin.DB.Table("moderation_appeal_models").Where("enforcement_id=?", secondID).Count(&count).Error; err != nil || count != 0 {
		t.Fatal("appeal survived failed audit", err)
	}
	if err = runtime.DB.Exec(`UPDATE moderation_appeal_models SET explanation='rewrite' WHERE enforcement_id=?`, noticeID).Error; err == nil {
		t.Fatal("runtime can rewrite appeal evidence")
	}
	if err = runtime.DB.Exec(`INSERT INTO moderation_enforcement_models(id,subject_user_id,action,policy_reason,issued_at) VALUES(?,?,'ban','Fake decision',?)`, newTestID(), owner, at).Error; err == nil {
		t.Fatal("runtime can issue enforcement")
	}
}
