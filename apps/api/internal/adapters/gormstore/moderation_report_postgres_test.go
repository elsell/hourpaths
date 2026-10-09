package gormstore

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresReportSnapshotReplayAuditAndPrivateCases(t *testing.T) {
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
	reporter, subject := "reporter-"+newTestID(), "subject-"+newTestID()
	for _, id := range []string{reporter, subject} {
		name := "report_" + strings.ReplaceAll(newTestID(), "-", "")
		description := "Original description"
		if err = admin.DB.Create(&userModel{ID: id, Username: &name, DisplayName: "Report test", Description: &description, Status: identity.StatusActive, CreatedAt: at, UpdatedAt: at}).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := NewModerationRepository(runtime.DB)
	ctx := context.Background()
	target := domain.Target{Kind: domain.Profile, ID: subject}
	access, err := r.ResolveReportTarget(ctx, reporter, target, at)
	if err != nil {
		t.Fatal(err)
	}
	id := newTestID()
	command := app.ReportCommand{ID: id, ReporterID: reporter, Access: access, Reason: domain.SpamOrScam, Explanation: "Context", At: at, Idempotency: ports.Idempotency{PrincipalID: reporter, Operation: app.SubmitOperation, Key: "report-operation-0001", RequestHash: bytes.Repeat([]byte{1}, 32)}, Audit: audit.Event{ID: newTestID(), OwnerUserID: reporter, ActorUserID: reporter, Action: audit.ResourceCreated, TargetType: "moderation_report", TargetID: id, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}}
	first, err := r.SubmitReport(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if first.BlockTarget == nil || first.BlockTarget.UserID != subject {
		t.Fatal("missing current block identity")
	}
	if err = admin.DB.Model(&userModel{}).Where("id=?", subject).Update("description", "Changed description").Error; err != nil {
		t.Fatal(err)
	}
	again, err := r.SubmitReport(ctx, command)
	if err != nil || again.ID != first.ID {
		t.Fatalf("replay=%+v err=%v", again, err)
	}
	var row struct{ Evidence []byte }
	if err = admin.DB.Table("moderation_case_models").Select("evidence").Where("id=?", id).Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(row.Evidence, []byte("Original description")) || bytes.Contains(row.Evidence, []byte("Changed description")) {
		t.Fatal("evidence was rewritten")
	}
	if err = runtime.DB.Table("moderation_case_models").Select("id").Limit(1).Scan(&[]string{}).Error; err == nil {
		t.Fatal("runtime can read private cases")
	}
	command.Idempotency.RequestHash = bytes.Repeat([]byte{2}, 32)
	if _, err = r.SubmitReport(ctx, command); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("changed retry: %v", err)
	}
	command.ID = newTestID()
	command.Idempotency.Key = "report-operation-0002"
	// Reuse the first audit ID to force a database error after the case INSERT.
	command.Audit.TargetID = command.ID
	command.Audit.Action = audit.ResourceCreated
	if _, err = r.SubmitReport(ctx, command); err == nil {
		t.Fatal("accepted invalid audit")
	}
	var count int64
	admin.DB.Table("moderation_case_models").Where("reporter_id=?", reporter).Count(&count)
	if count != 1 {
		t.Fatalf("cases=%d", count)
	}
	if err = admin.DB.Create(&socialBlockTestModel{BlockerUserID: subject, BlockedUserID: reporter, CreatedAt: at}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = r.ResolveReportTarget(ctx, reporter, target, at); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("blocked target exposed: %v", err)
	}
	replayKey := command.Idempotency
	replayKey.Key = "report-operation-0001"
	replayKey.RequestHash = bytes.Repeat([]byte{1}, 32)
	blocked, found, err := r.FindReportReplay(ctx, replayKey)
	if err != nil || !found || blocked.ID != first.ID || blocked.BlockTarget != nil {
		t.Fatalf("blocked identity exposed by retry: %+v %v", blocked, err)
	}
	deleteAccount := func(id string) error {
		cleanupDeletionOutbox(t, admin, id)
		return runtime.DeleteAccount(ctx, application.AccountDeletionCommand{UserID: id, DeletedAt: at, NewID: newTestID, ReceiptHash: bytes.Repeat([]byte{3}, 32), Audit: audit.Event{ID: newTestID(), OwnerUserID: id, ActorUserID: id, Action: audit.ResourceDeleted, TargetType: "account", TargetID: id, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}})
	}
	if err = deleteAccount(subject); err != nil {
		t.Fatal(err)
	}
	deleted, found, err := r.FindReportReplay(ctx, replayKey)
	if err != nil || !found || deleted.BlockTarget != nil {
		t.Fatal("deleted identity exposed by retry")
	}
	if err = deleteAccount(reporter); err != nil {
		t.Fatal(err)
	}
	var receipts int64
	if err = admin.DB.Table("moderation_report_receipt_models").Where("reporter_id=?", reporter).Count(&receipts).Error; err != nil || receipts != 0 {
		t.Fatal("reporter deletion retained receipts")
	}
	var retained struct {
		ReporterID, SubjectUserID *string
		Evidence                  []byte
	}
	if err = admin.DB.Table("moderation_case_models").Where("id=?", id).Take(&retained).Error; err != nil || retained.ReporterID != nil || retained.SubjectUserID != nil || !bytes.Contains(retained.Evidence, []byte("Original description")) {
		t.Fatalf("minimal investigation evidence: %+v %v", retained, err)
	}

}
