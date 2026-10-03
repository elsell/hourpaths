package deletionjournal

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app"
)

func TestRetirementPreservesPendingAndRecentRecordsAndPersistsRestoreBoundary(t *testing.T) {
	ctx := context.Background()
	directory := filepath.Join(t.TempDir(), "journal")
	key := bytes.Repeat([]byte{9}, 32)
	journal, err := New(directory, key)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	for _, fixture := range []struct {
		owner string
		age   time.Duration
	}{{"completed", 31 * 24 * time.Hour}, {"pending", 31 * 24 * time.Hour}, {"recent", time.Hour}} {
		_, err = journal.Admit(ctx, application.DeletionRecord{UserID: fixture.owner, DeletedAt: now.Add(-fixture.age), AuditEventID: fixture.owner + "-audit", ReceiptHash: strings.Repeat("01", 32)})
		if err != nil {
			t.Fatal(err)
		}
	}
	removed, err := journal.Retire(ctx, now, 10, func(_ context.Context, record application.DeletionRecord) (bool, error) {
		return record.UserID != "pending", nil
	})
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d error=%v", removed, err)
	}
	journal, err = New(directory, key)
	if err != nil {
		t.Fatal(err)
	}
	records, boundary, err := journal.Snapshot(ctx)
	if err != nil || !boundary.Equal(now) || len(records) != 2 {
		t.Fatalf("records=%v boundary=%v error=%v", records, boundary, err)
	}
	for _, record := range records {
		if record.UserID == "completed" {
			t.Fatal("completed record retained")
		}
	}
	if _, err = journal.Retire(ctx, now.Add(-time.Second), 10, func(context.Context, application.DeletionRecord) (bool, error) { return true, nil }); err == nil {
		t.Fatal("backward clock accepted")
	}
	if err = os.Remove(filepath.Join(directory, boundaryFile)); err != nil {
		t.Fatal(err)
	}
	if _, _, err = journal.Snapshot(ctx); err == nil {
		t.Fatal("missing boundary accepted")
	}
	if _, err = New(directory, key); err == nil {
		t.Fatal("missing boundary silently reset on reopen")
	}
}

func TestFailedRetirementEligibilityDoesNotRemoveEvidence(t *testing.T) {
	journal, err := New(filepath.Join(t.TempDir(), "journal"), bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	record := application.DeletionRecord{UserID: "owner", DeletedAt: now.Add(-31 * 24 * time.Hour), AuditEventID: "audit", ReceiptHash: strings.Repeat("02", 32)}
	if _, err = journal.Admit(context.Background(), record); err != nil {
		t.Fatal(err)
	}
	unavailable := errors.New("unavailable")
	count, err := journal.Retire(context.Background(), now, 1, func(context.Context, application.DeletionRecord) (bool, error) { return false, unavailable })
	if !errors.Is(err, unavailable) || count != 0 {
		t.Fatal(count, err)
	}
	records, boundary, err := journal.Snapshot(context.Background())
	if err != nil || len(records) != 1 || !boundary.IsZero() {
		t.Fatal("failed eligibility changed journal", err)
	}
}

func TestSnapshotCannotObserveRetirementWithOldBoundary(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "journal")
	key := bytes.Repeat([]byte{4}, 32)
	journal, err := New(directory, key)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := New(directory, key)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	if _, err = journal.Admit(context.Background(), application.DeletionRecord{UserID: "owner", DeletedAt: now.Add(-31 * 24 * time.Hour), AuditEventID: "audit", ReceiptHash: strings.Repeat("03", 32)}); err != nil {
		t.Fatal(err)
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		_, err := journal.Retire(context.Background(), now, 1, func(context.Context, application.DeletionRecord) (bool, error) {
			close(entered)
			<-release
			return true, nil
		})
		done <- err
	}()
	<-entered
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	_, _, err = reader.Snapshot(ctx)
	cancel()
	close(release)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("snapshot bypassed in-progress retirement", err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	records, boundary, err := reader.Snapshot(context.Background())
	if err != nil || len(records) != 0 || !boundary.Equal(now) {
		t.Fatal("inconsistent snapshot", boundary, err)
	}
	if err = os.Remove(filepath.Join(directory, boundaryFile)); err != nil {
		t.Fatal(err)
	}
	if _, err = New(directory, key); err == nil {
		t.Fatal("empty retired journal lost boundary on reopen")
	}
}
