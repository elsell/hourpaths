package activity

import (
	"testing"
	"time"
)

func TestMergeEditRetainsLateLosingVersion(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	original, err := RecordManualActivity(ManualActivity{ID: "entry", PathID: "path", ParticipantID: "owner", StartedAt: now.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "UTC", Note: "original"}, now)
	if err != nil {
		t.Fatal(err)
	}
	initial, _ := NewActivityEditOrder(now, 0, "initial")
	newer, _ := NewActivityEditOrder(now.Add(time.Minute), 0, "newer")
	older, _ := NewActivityEditOrder(now.Add(time.Second), 0, "older")
	edit := ActivityEdit{StartedAt: original.StartedAt, DurationSeconds: 120, OccurrenceTimeZone: "UTC", Note: "newer"}
	first, err := original.MergeEditByOwner("owner", edit, initial, newer, now.Add(2*time.Minute))
	if err != nil || !first.Applied || first.Activity.Note != "newer" || first.Revision.Activity != original {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	edit.Note = "older"
	edit.DurationSeconds = 90
	late, err := first.Activity.MergeEditByOwner("owner", edit, first.Order, older, now.Add(3*time.Minute))
	if err != nil || late.Applied || late.Activity != first.Activity || late.Order != newer || late.Revision.Activity.Note != "older" || late.Revision.Activity.DurationSeconds() != 90 || !late.Revision.ReplacedAt.Equal(now.Add(3*time.Minute)) {
		t.Fatalf("late=%+v err=%v", late, err)
	}
	early, err := original.MergeEditByOwner("owner", edit, initial, older, now.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	edit.Note = "newer"
	edit.DurationSeconds = 120
	last, err := early.Activity.MergeEditByOwner("owner", edit, early.Order, newer, now.Add(3*time.Minute))
	if err != nil || !last.Applied || last.Activity.Note != first.Activity.Note || last.Activity.DurationSeconds() != first.Activity.DurationSeconds() {
		t.Fatalf("last=%+v err=%v", last, err)
	}
}

func TestMergeEditValidatesLosingEditsAndOwnerBeforeOrdering(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	entry, _ := RecordManualActivity(ManualActivity{ID: "entry", PathID: "path", ParticipantID: "owner", StartedAt: now.Add(-time.Hour), DurationSeconds: 60, OccurrenceTimeZone: "UTC"}, now)
	current, _ := NewActivityEditOrder(now, 1, "current")
	incoming, _ := NewActivityEditOrder(now, 0, "old")
	edit := ActivityEdit{StartedAt: entry.StartedAt, DurationSeconds: 90, OccurrenceTimeZone: "UTC"}
	if _, err := entry.MergeEditByOwner("other", edit, current, incoming, now); err == nil {
		t.Fatal("cross-owner losing edit accepted")
	}
	edit.DurationSeconds = 0
	if _, err := entry.MergeEditByOwner("owner", edit, current, incoming, now); err == nil {
		t.Fatal("invalid losing edit accepted")
	}
	edit.DurationSeconds = 90
	if _, err := entry.MergeEditByOwner("owner", edit, current, ActivityEditOrder{}, now); err == nil {
		t.Fatal("invalid order accepted")
	}
	if _, err := entry.MergeEditByOwner("owner", edit, current, current, now); err == nil {
		t.Fatal("same operation must be settled through idempotency boundary")
	}
}
