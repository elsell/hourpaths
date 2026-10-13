package app

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type restrictionFixture struct {
	value *moderation.Enforcement
	err   error
	owner string
	at    time.Time
}

func (f *restrictionFixture) ActiveAccountRestriction(_ context.Context, owner string, at time.Time) (*moderation.Enforcement, error) {
	f.owner, f.at = owner, at
	return f.value, f.err
}

func TestAccountRestrictionAdmissionPreservesSessionAndPrivateNotice(t *testing.T) {
	a := timeZoneTestApp(nil)
	now := a.Clock.Now()
	var events []audit.Event
	a.Audits = fakeAudits{events: &events}
	f := &restrictionFixture{value: &moderation.Enforcement{ID: "notice", SubjectUserID: "owner", Action: moderation.Suspension, PolicyReason: "Reviewed policy reason", IssuedAt: now.Add(-time.Hour), Until: now.Add(time.Hour)}}
	a.AccountRestrictions = f
	if err := a.AdmitAccountUse(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrAccountRestricted) {
		t.Fatalf("restriction admission: %v", err)
	}
	review, err := a.CurrentAccountRestriction(context.Background(), "Bearer session")
	if err != nil || review.UserID != "owner" || review.Decision == nil || review.Decision.ID != "notice" || f.owner != "owner" || !f.at.Equal(now) {
		t.Fatalf("private review=%+v err=%v", review, err)
	}
	// A storage result must never substitute another account's private decision.
	f.value.SubjectUserID = "other"
	if _, err = a.CurrentAccountRestriction(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatal("foreign decision exposed", err)
	}
	f.value = nil
	if err = a.AdmitAccountUse(context.Background(), "Bearer session"); err != nil {
		t.Fatal(err)
	}
	f.err = errors.New("storage unavailable")
	if err = a.AdmitAccountUse(context.Background(), "Bearer session"); err == nil {
		t.Fatal("failed storage granted access")
	}
	a.AccountRestrictions = nil
	if err = a.AdmitAccountUse(context.Background(), "Bearer session"); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatal("missing dependency granted access")
	}
	if len(events) != 2 || events[0].Action != audit.ResourceAccessDenied || events[1].Action != audit.ResourceViewed {
		t.Fatalf("missing private admission/review audit: %+v", events)
	}
}
