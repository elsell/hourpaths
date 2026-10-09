package moderation

import (
	"errors"
	"testing"
	"time"
)

func TestAppealWindowAndFinality(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	notice := Enforcement{ID: "notice", SubjectUserID: "subject", Action: Warning, PolicyReason: "Community guidelines", IssuedAt: at}
	for _, tc := range []struct {
		name string
		now  time.Time
		want error
	}{
		{"before notice", at.Add(-time.Nanosecond), ErrAppealUnavailable},
		{"last instant", at.Add(30*24*time.Hour - time.Nanosecond), nil},
		{"deadline", at.Add(30 * 24 * time.Hour), ErrAppealUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			appeal, err := notice.NewAppeal("appeal", "  Please review the context.  ", tc.now)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if err == nil && (appeal.Explanation != "Please review the context." || appeal.EnforcementID != notice.ID) {
				t.Fatal("appeal lost its explanation or decision binding")
			}
		})
	}
	appeal, err := notice.NewAppeal("appeal", "Context", at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := appeal.Decide("reviewer2", AppealReversed, "Context changes the decision", at.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if appeal.Outcome != "" || resolved.Explanation != appeal.Explanation || resolved.SubmittedAt != appeal.SubmittedAt {
		t.Fatal("review mutated original appeal evidence")
	}
	if _, err = resolved.Decide("reviewer3", AppealUpheld, "Changed my mind", at.Add(3*time.Hour)); !errors.Is(err, ErrAppealFinal) {
		t.Fatal("final appeal was reopened")
	}
}

func TestTemporaryRestrictionsEndWithoutChangingHistoricalDecision(t *testing.T) {
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	until := at.Add(time.Hour)
	suspension := Enforcement{ID: "notice", SubjectUserID: "subject", Action: Suspension, PolicyReason: "Policy", IssuedAt: at, Until: until}
	if err := suspension.Validate(); err != nil {
		t.Fatal(err)
	}
	if !suspension.RestrictsAccount(at) || suspension.RestrictsAccount(until) || suspension.RestrictsAccount(at.Add(-time.Second)) {
		t.Fatal("restriction applied outside its interval")
	}
	suspension.Until = at
	if err := suspension.Validate(); err == nil {
		t.Fatal("invalid suspension accepted")
	}
	ban := Enforcement{ID: "ban", SubjectUserID: "subject", Action: Ban, PolicyReason: "Policy", IssuedAt: at}
	if !ban.RestrictsAccount(at.Add(1000 * 24 * time.Hour)) {
		t.Fatal("permanent restriction expired")
	}
	ban.Until = until
	if err := ban.Validate(); err == nil {
		t.Fatal("permanent ban accepted with temporary expiry")
	}
}
