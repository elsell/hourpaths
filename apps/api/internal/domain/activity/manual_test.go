package activity

import (
	"strings"
	"testing"
	"time"
)

func TestRecordManualActivityBuildsCanonicalCompletedActivity(t *testing.T) {
	now := time.Date(2026, 7, 22, 16, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	startedAt := time.Date(2026, 7, 22, 10, 30, 0, 250_000_000, time.FixedZone("EDT", -4*60*60))

	activity, err := RecordManualActivity(ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "participant-1",
		StartedAt: startedAt, DurationSeconds: 90, OccurrenceTimeZone: "America/New_York",
		Note: "Cafe\u0301 practice\nsecond line",
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	wantStart := startedAt.UTC()
	wantEnd := wantStart.Add(90 * time.Second)
	if activity.ID != "activity-1" || activity.PathID != "path-1" || activity.ParticipantID != "participant-1" {
		t.Fatalf("manual identity = %+v", activity)
	}
	if activity.StartedAt != wantStart || activity.EndedAt != wantEnd || activity.StartedAt.Location() != time.UTC || activity.EndedAt.Location() != time.UTC {
		t.Fatalf("manual instants = (%v, %v), want (%v, %v)", activity.StartedAt, activity.EndedAt, wantStart, wantEnd)
	}
	if activity.OccurrenceTimeZone != "America/New_York" || activity.Note != "Café practice\nsecond line" {
		t.Fatalf("manual occurrence data = %+v", activity)
	}
	if activity.CreatedAt != now.UTC() || activity.UpdatedAt != now.UTC() || activity.DurationSeconds() != 90 {
		t.Fatalf("manual lifecycle = %+v", activity)
	}
}

func TestRecordManualActivityTreatsWhitespaceOnlyNoteAsAbsent(t *testing.T) {
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)
	activity, err := RecordManualActivity(ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "participant-1",
		StartedAt: now.Add(-time.Second), DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC", Note: " \r\n\u2003 ",
	}, now)
	if err != nil || activity.Note != "" {
		t.Fatalf("whitespace-only note = %q, %v", activity.Note, err)
	}
}

func TestRecordManualActivityValidatesCanonicalCompletedState(t *testing.T) {
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)
	valid := ManualActivity{ID: "activity-1", PathID: "path-1", ParticipantID: "participant-1", StartedAt: now.Add(-time.Minute), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC"}
	tests := map[string]func(*ManualActivity, *time.Time){
		"missing identity":        func(input *ManualActivity, _ *time.Time) { input.ID = " " },
		"missing Path":            func(input *ManualActivity, _ *time.Time) { input.PathID = "" },
		"missing participant":     func(input *ManualActivity, _ *time.Time) { input.ParticipantID = "\t" },
		"missing start":           func(input *ManualActivity, _ *time.Time) { input.StartedAt = time.Time{} },
		"zero duration":           func(input *ManualActivity, _ *time.Time) { input.DurationSeconds = 0 },
		"negative duration":       func(input *ManualActivity, _ *time.Time) { input.DurationSeconds = -1 },
		"future end":              func(input *ManualActivity, _ *time.Time) { input.DurationSeconds = 61 },
		"invalid occurrence zone": func(input *ManualActivity, _ *time.Time) { input.OccurrenceTimeZone = "UTC-04:00" },
		"blank occurrence zone":   func(input *ManualActivity, _ *time.Time) { input.OccurrenceTimeZone = "" },
		"missing current instant": func(_ *ManualActivity, current *time.Time) { *current = time.Time{} },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			input, current := valid, now
			mutate(&input, &current)
			if got, err := RecordManualActivity(input, current); err == nil || got != (RecordedActivity{}) {
				t.Fatalf("RecordManualActivity() = %+v, %v", got, err)
			}
		})
	}
}

func TestActivityNoteValidationUsesNormalizedUnicodeCharacters(t *testing.T) {
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)
	base := ManualActivity{ID: "activity-1", PathID: "path-1", ParticipantID: "participant-1", StartedAt: now.Add(-time.Second), DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC"}
	for name, note := range map[string]string{
		"tab control":         "practice\tlog",
		"nul control":         "practice\x00log",
		"too many characters": strings.Repeat("é", 2001),
		"invalid UTF-8":       string([]byte{0xff}),
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			input.Note = note
			if _, err := RecordManualActivity(input, now); err == nil {
				t.Fatal("invalid note was accepted")
			}
		})
	}
	base.Note = strings.Repeat("e\u0301", 2000) + "\r\n🙂"
	if _, err := RecordManualActivity(base, now); err == nil {
		t.Fatal("normalized note beyond 2,000 characters was accepted")
	}
	base.Note = strings.Repeat("e\u0301", 1997) + "\r\n🙂"
	if got, err := RecordManualActivity(base, now); err != nil || got.Note != strings.Repeat("é", 1997)+"\r\n🙂" {
		t.Fatalf("valid normalized multiline note = %q, %v", got.Note, err)
	}
}

func TestOwnerEditReturnsUpdatedActivityAndImmutablePriorRevision(t *testing.T) {
	createdAt := time.Date(2026, 7, 20, 15, 0, 0, 0, time.UTC)
	original, err := RecordManualActivity(ManualActivity{
		ID: "activity-1", PathID: "path-1", ParticipantID: "participant-1",
		StartedAt: createdAt.Add(-time.Hour), DurationSeconds: 600, OccurrenceTimeZone: "America/New_York", Note: "original",
	}, createdAt)
	if err != nil {
		t.Fatal(err)
	}
	originalCopy := original
	editedAt := createdAt.Add(48 * time.Hour)
	edited, revision, err := original.EditByOwner("participant-1", ActivityEdit{
		StartedAt: createdAt.Add(-2 * time.Hour), DurationSeconds: 3600,
		OccurrenceTimeZone: "Europe/Paris", Note: "revise\u0301d",
	}, editedAt)
	if err != nil {
		t.Fatal(err)
	}
	if original != originalCopy || revision.Activity != original || revision.ReplacedAt != editedAt.UTC() {
		t.Fatalf("prior state mutated: original=%+v revision=%+v", original, revision)
	}
	if edited.ID != original.ID || edited.PathID != original.PathID || edited.ParticipantID != original.ParticipantID || edited.CreatedAt != original.CreatedAt {
		t.Fatalf("edit changed stable state: %+v", edited)
	}
	if edited.StartedAt != createdAt.Add(-2*time.Hour).UTC() || edited.DurationSeconds() != 3600 || edited.OccurrenceTimeZone != "Europe/Paris" || edited.Note != "reviséd" || edited.UpdatedAt != editedAt.UTC() {
		t.Fatalf("edited activity = %+v", edited)
	}
}

func TestActivityEditRejectsNonOwnerAndInvalidReplacementWithoutMutation(t *testing.T) {
	now := time.Date(2026, 7, 22, 20, 0, 0, 0, time.UTC)
	original, err := RecordManualActivity(ManualActivity{ID: "activity-1", PathID: "path-1", ParticipantID: "participant-1", StartedAt: now.Add(-time.Minute), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC"}, now)
	if err != nil {
		t.Fatal(err)
	}
	validEdit := ActivityEdit{StartedAt: now.Add(-time.Minute), DurationSeconds: 30, OccurrenceTimeZone: "Etc/UTC"}
	for name, test := range map[string]struct {
		owner string
		edit  ActivityEdit
		at    time.Time
	}{
		"different participant":  {owner: "participant-2", edit: validEdit, at: now},
		"future end":             {owner: "participant-1", edit: ActivityEdit{StartedAt: now, DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC"}, at: now},
		"subsecond unavailable":  {owner: "participant-1", edit: ActivityEdit{StartedAt: now.Add(-time.Second), DurationSeconds: 0, OccurrenceTimeZone: "Etc/UTC"}, at: now},
		"invalid note":           {owner: "participant-1", edit: ActivityEdit{StartedAt: now.Add(-time.Second), DurationSeconds: 1, OccurrenceTimeZone: "Etc/UTC", Note: "bad\t"}, at: now},
		"missing edit instant":   {owner: "participant-1", edit: validEdit},
		"edit predates activity": {owner: "participant-1", edit: validEdit, at: now.Add(-time.Second)},
	} {
		t.Run(name, func(t *testing.T) {
			edited, revision, editErr := original.EditByOwner(test.owner, test.edit, test.at)
			if editErr == nil || edited != (RecordedActivity{}) || revision != (ActivityRevision{}) || original.Note != "" || original.DurationSeconds() != 60 {
				t.Fatalf("EditByOwner() = %+v, %+v, %v; original=%+v", edited, revision, editErr, original)
			}
		})
	}
}
