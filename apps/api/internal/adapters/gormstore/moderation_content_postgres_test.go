package gormstore

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresReportContentVisibilityAndConcurrentReceipt(t *testing.T) {
	f := newNudgeFixture(t, "reportcontent", false)
	ctx := context.Background()
	activityID := "report-activity-" + newTestID()
	if err := f.migration.DB.Table("recorded_activity_models").Create(map[string]any{"id": activityID, "path_id": f.pathID, "participant_id": f.sender.ID, "occurrence_time_zone": "Etc/UTC", "started_at": f.now.Add(-time.Hour), "ended_at": f.now.Add(-time.Minute), "created_at": f.now, "updated_at": f.now}).Error; err != nil {
		t.Fatal(err)
	}
	eventID := "practice:" + activityID
	target, err := f.repository.ResolvePracticeCommentTarget(ctx, f.recipient.ID, eventID)
	if err != nil {
		t.Fatal(err)
	}
	comment, err := f.repository.CreatePracticeComment(ctx, socialCommentTestCommand(f.sender.ID, target, "Original public text", "", 0, socialapp.CreatePracticeCommentOperation, "report-content-comment-01", f.now))
	if err != nil {
		t.Fatal(err)
	}
	r := NewModerationRepository(f.runtime.DB)
	for _, target := range []domain.Target{{Kind: domain.Path, ID: f.pathID}, {Kind: domain.FeedEvent, ID: eventID}, {Kind: domain.Comment, ID: comment.Comment.ID}} {
		access, err := r.ResolveReportTarget(ctx, f.recipient.ID, target, f.now)
		if err != nil || access.PathID != f.pathID || access.SubjectUserID != f.sender.ID {
			t.Fatalf("%s: %+v %v", target.Kind, access, err)
		}
	}
	access, err := r.ResolveReportTarget(ctx, f.recipient.ID, domain.Target{Kind: domain.Comment, ID: comment.Comment.ID}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	command := app.ReportCommand{ID: newTestID(), ReporterID: f.recipient.ID, Access: access, Reason: domain.SpamOrScam, At: f.now, Idempotency: ports.Idempotency{PrincipalID: f.recipient.ID, Operation: app.SubmitOperation, Key: "report-content-concurrent", RequestHash: bytesOf(5)}}
	command.Audit = audit.Event{ID: newTestID(), OwnerUserID: f.recipient.ID, ActorUserID: f.recipient.ID, Action: audit.ResourceCreated, TargetType: "moderation_report", TargetID: command.ID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: f.now}
	var wait sync.WaitGroup
	results := make([]app.Receipt, 2)
	failures := make([]error, 2)
	for i := range 2 {
		wait.Add(1)
		go func() { defer wait.Done(); results[i], failures[i] = r.SubmitReport(ctx, command) }()
	}
	wait.Wait()
	for i := range 2 {
		if failures[i] != nil || results[i].ID != command.ID {
			t.Fatalf("retry %d: %+v %v", i, results[i], failures[i])
		}
	}
	var count int64
	if err := f.migration.DB.Table("moderation_case_models").Where("reporter_id=?", f.recipient.ID).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("concurrent cases=%d %v", count, err)
	}
	if err := f.migration.DB.Table("path_membership_models").Where("path_id=? AND user_id=?", f.pathID, f.recipient.ID).Delete(&struct{}{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.migration.DB.Table("path_models").Where("id=?", f.pathID).Update("visibility", "private").Error; err != nil {
		t.Fatal(err)
	}
	for _, target := range []domain.Target{{Kind: domain.Path, ID: f.pathID}, {Kind: domain.FeedEvent, ID: eventID}, {Kind: domain.Comment, ID: comment.Comment.ID}} {
		if _, err := r.ResolveReportTarget(ctx, f.recipient.ID, target, f.now); !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("revoked %s error=%v", target.Kind, err)
		}
	}
}
