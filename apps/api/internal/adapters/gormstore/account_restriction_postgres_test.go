package gormstore

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"testing"
	"time"
)

func TestPostgresAccountRestrictionOwnerExpiryAndReversal(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL integration DSNs required")
	}
	runtime, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeAccountRestrictionStore(t, runtime)
	admin, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeAccountRestrictionStore(t, admin)
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, other := newTestID(), newTestID()
	for _, id := range []string{owner, other} {
		if err = admin.DB.Create(&userModel{ID: id, Status: identity.StatusActive, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	until := now.Add(time.Hour)
	suspension := enforcementModel{ID: newTestID(), SubjectUserID: owner, Action: "suspension", PolicyReason: "Reason", IssuedAt: now, UntilAt: &until}
	ban := enforcementModel{ID: newTestID(), SubjectUserID: owner, Action: "ban", PolicyReason: "Reason", IssuedAt: now}
	peerBan := enforcementModel{ID: newTestID(), SubjectUserID: other, Action: "ban", PolicyReason: "Reason", IssuedAt: now}
	for _, row := range []enforcementModel{suspension, ban, peerBan} {
		if err = admin.DB.Omit("CaseID", "AffectedCommentID", "AffectedCommentCreatedAt").Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	read := func(at time.Time, want string) {
		t.Helper()
		decision, e := runtime.ActiveAccountRestriction(context.Background(), owner, at)
		if e != nil {
			t.Fatal(e)
		}
		if want == "" {
			if decision != nil {
				t.Fatal("restriction unexpectedly active")
			}
		} else if decision == nil || decision.ID != want || decision.SubjectUserID != owner {
			t.Fatalf("decision=%+v want=%s", decision, want)
		}
	}
	read(now.Add(-time.Nanosecond), "")
	read(now, ban.ID)
	if err = admin.DB.Create(&appealModel{EnforcementID: ban.ID, ID: newTestID(), Explanation: "Appeal", SubmittedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err = admin.DB.Create(&appealDecisionModel{EnforcementID: ban.ID, Outcome: string(moderation.AppealReversed), Reviewer: "reviewer", Reason: "Reversed", DecidedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	read(now, suspension.ID)
	read(until, "")
	peer, e := runtime.ActiveAccountRestriction(context.Background(), other, until)
	if e != nil || peer == nil || peer.ID != peerBan.ID {
		t.Fatal("peer restriction changed", e)
	}
}

func closeAccountRestrictionStore(t *testing.T, store *Store) {
	t.Helper()
	db, err := store.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
}
