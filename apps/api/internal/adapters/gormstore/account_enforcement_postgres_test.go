package gormstore

import (
	"bytes"
	"context"
	"errors"
	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/operationalmoderation"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	activitydomain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/domain/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type enforcementClock struct{ at time.Time }

func (c enforcementClock) Now() time.Time { return c.at }

func TestPostgresAccountEnforcementStopsOnlySubjectTimersAtomically(t *testing.T) {
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
	owner, peer, caseID := newTestID(), newTestID(), newTestID()
	for _, id := range []string{owner, peer} {
		if err = admin.DB.Create(&userModel{ID: id, Status: identity.StatusActive, DisplayName: "Enforcement fixture", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	var subjectPath string
	for _, subject := range []string{owner, owner, peer} {
		path := newTestID()
		if subject == owner {
			subjectPath = path
		}
		if err = admin.DB.Exec("INSERT INTO path_models(id,owner_user_id,name,visibility,created_at,updated_at) VALUES(?,?,'Test','private',?,?)", path, subject, now.Add(-time.Hour), now).Error; err != nil {
			t.Fatal(err)
		}
		if err = admin.DB.Exec("INSERT INTO path_membership_models(path_id,user_id,role,joined_at) VALUES(?,?,'administrator',?)", path, subject, now.Add(-time.Hour)).Error; err != nil {
			t.Fatal(err)
		}
		if err = admin.DB.Exec("INSERT INTO running_timer_models(id,path_id,participant_id,started_at,occurrence_time_zone) VALUES(?,?,?,?,'America/New_York')", newTestID(), path, subject, now.Add(-time.Minute)).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = admin.DB.Exec("INSERT INTO moderation_case_models(id,reporter_id,subject_user_id,target_kind,target_id,reason,explanation,evidence,created_at) VALUES(?,?,?,'profile',?,'spam_or_scam','','{}',?)", caseID, peer, owner, owner, now).Error; err != nil {
		t.Fatal(err)
	}
	command := app.AccountEnforcementCommand{CaseID: caseID, NoticeID: newTestID(), Action: moderation.Suspension, PolicyReason: "Reviewed case", Until: now.Add(time.Hour)}
	denied := operationalmoderation.AccountEnforcer{DB: runtime.DB, Clock: enforcementClock{now}, NewID: newTestID}
	if _, err = denied.RestrictAccount(context.Background(), command); err == nil {
		t.Fatal("runtime credential issued enforcement")
	}
	broken := operationalmoderation.AccountEnforcer{DB: admin.DB, Clock: enforcementClock{now}, NewID: func() string { return "" }}
	if _, err = broken.RestrictAccount(context.Background(), command); err == nil {
		t.Fatal("broken audit succeeded")
	}
	var count int64
	if err = admin.DB.Table("moderation_enforcement_models").Where("id=?", command.NoticeID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("partial restriction committed")
	}
	if err = admin.DB.Table("running_timer_models").Where("participant_id=?", owner).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("failed enforcement changed timers")
	}
	clock := &pausedEnforcementClock{at: now, entered: make(chan struct{}), release: make(chan struct{})}
	service := operationalmoderation.AccountEnforcer{DB: admin.DB, Clock: clock, NewID: newTestID}
	defer func() {
		select {
		case <-clock.release:
		default:
			close(clock.release)
		}
	}()
	type outcome struct {
		decision moderation.Enforcement
		err      error
	}
	done := make(chan outcome, 1)
	go func() {
		decision, e := service.RestrictAccount(context.Background(), command)
		done <- outcome{decision, e}
	}()
	select {
	case <-clock.entered:
	case early := <-done:
		t.Fatalf("enforcement failed before lock: %v", early.err)
	case <-time.After(5 * time.Second):
		t.Fatal("enforcement did not acquire account lock")
	}
	// A separate runtime connection cannot obtain the shared account lock while
	// enforcement is paused. This verifies the interleaving without sleeping.
	var lockedID string
	lockErr := runtime.DB.Raw("SELECT id FROM user_models WHERE id=? FOR SHARE NOWAIT", owner).Row().Scan(&lockedID)
	var sqlState interface{ SQLState() string }
	if !errors.As(lockErr, &sqlState) || sqlState.SQLState() != "55P03" {
		close(clock.release)
		t.Fatalf("account lock absent: %v", lockErr)
	}
	timer, _ := activitydomain.StartTimer(newTestID(), subjectPath, owner, now, "Etc/UTC", now)
	timerAudit := audit.Event{ID: newTestID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ActivityTimerStarted, TargetType: "timer", TargetID: timer.ID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: now}
	started := make(chan struct{})
	raced := make(chan error, 1)
	go func() {
		close(started)
		_, e := activitystore.New(runtime.DB).StartTimer(context.Background(), activityapp.StartTimerCommand{Timer: timer, Idempotency: ports.Idempotency{PrincipalID: owner, Operation: activityapp.StartTimerOperation, Key: newTestID(), RequestHash: bytes.Repeat([]byte{3}, 32)}, Audit: timerAudit})
		raced <- e
	}()
	<-started
	close(clock.release)
	result := <-done
	decision, err := result.decision, result.err
	if err != nil {
		t.Fatal(err)
	}
	if raceErr := <-raced; !errors.Is(raceErr, ports.ErrAccountRestricted) {
		t.Fatalf("racing start bypassed enforcement: %v", raceErr)
	}
	service.Clock = enforcementClock{now}
	_, err = activitystore.New(runtime.DB).SynchronizeTimer(context.Background(), activityapp.OfflineTimerCommand{Timer: timer, Kind: "start", RecordedAt: now, IdentityHash: activityapp.OfflineTimerIdentityHash(timer), Idempotency: ports.Idempotency{PrincipalID: owner, Operation: activityapp.OfflineTimerOperation, Key: newTestID(), RequestHash: bytes.Repeat([]byte{4}, 32)}, Audit: timerAudit})
	if !errors.Is(err, ports.ErrAccountRestricted) {
		t.Fatalf("offline replay bypassed enforcement: %v", err)
	}
	if decision.SubjectUserID != owner || !decision.IssuedAt.Equal(now) {
		t.Fatal("wrong decision", decision)
	}
	if replay, e := service.RestrictAccount(context.Background(), command); e != nil || replay != decision {
		t.Fatal("retry diverged", e)
	}
	if err = admin.DB.Table("running_timer_models").Where("participant_id=?", owner).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("subject timer survived")
	}
	if err = admin.DB.Table("running_timer_models").Where("participant_id=?", peer).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("peer timer changed")
	}
	if err = admin.DB.Table("recorded_activity_models").Where("participant_id=? AND started_at=? AND ended_at=? AND occurrence_time_zone=?", owner, now.Add(-time.Minute), now, "America/New_York").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("time lost or duplicated", count)
	}
	if err = admin.DB.Table("audit_event_models").Where("owner_user_id=? AND action='activity.timer_stopped'", owner).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatal("missing timer audits", count)
	}
	// Reversal is an operational capability, preserves the saved activities, and
	// does not restart timers. An overlapping ban must continue denying admission.
	if err = admin.DB.Table("moderation_appeal_models").Create(map[string]any{"enforcement_id": command.NoticeID, "id": newTestID(), "explanation": "Please review", "submitted_at": now}).Error; err != nil {
		t.Fatal(err)
	}
	if err = runtime.DB.Exec("SELECT moderation_decide_account_appeal(?,?,?,?)", command.NoticeID, "reversed", "Reviewed appeal", "Only available reviewer").Error; err == nil {
		t.Fatal("runtime decided appeal")
	}
	if err = admin.DB.Exec("SELECT moderation_decide_account_appeal(?,?,?,?)", command.NoticeID, "reversed", "Reviewed appeal", "").Error; err == nil {
		t.Fatal("same reviewer escaped justification")
	}
	if err = admin.DB.Exec("SELECT moderation_decide_account_appeal(?,?,?,?)", command.NoticeID, "reversed", "Reviewed appeal", "Only available reviewer").Error; err != nil {
		t.Fatal(err)
	}
	if active, e := runtime.ActiveAccountRestriction(context.Background(), owner, now); e != nil || active != nil {
		t.Fatalf("reversal not effective: %v %v", active, e)
	}
	if err = admin.DB.Exec("SELECT moderation_decide_account_appeal(?,?,?,?)", command.NoticeID, "upheld", "Changed decision", "Only available reviewer").Error; err == nil {
		t.Fatal("final appeal replaced")
	}
	if err = admin.DB.Table("running_timer_models").Where("participant_id=?", owner).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("reversal restarted timer: %d %v", count, err)
	}

}

type pausedEnforcementClock struct {
	at               time.Time
	entered, release chan struct{}
}

func (c *pausedEnforcementClock) Now() time.Time { close(c.entered); <-c.release; return c.at }
