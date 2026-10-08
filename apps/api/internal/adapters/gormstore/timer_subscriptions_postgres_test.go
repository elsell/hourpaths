package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresTimerSubscriptionsFollowDefaultReplayStaleAndUnfollow(t *testing.T) {
	relationships, db := socialRelationshipTestRepository(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	actor, target := socialRelationshipTestUsers(t, db, identity.ProfileVisibilityPublic, now)
	other := socialRelationshipTestUser(t, db, "other", identity.ProfileVisibilityPublic, now)
	if err := db.Create(&socialFollowModel{FollowerUserID: actor.ID, FollowingUserID: target.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewTimerSubscriptionRepository(relationships.db)
	ctx := context.Background()
	initial, err := repository.GetTimerSubscription(ctx, actor.ID, socialapp.TimerSubscriptionPerson, target.ID)
	if err != nil || initial.Enabled || initial.Revision != 0 {
		t.Fatalf("default: %+v %v", initial, err)
	}
	if _, err := repository.GetTimerSubscription(ctx, other.ID, socialapp.TimerSubscriptionPerson, target.ID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("unrelated account: %v", err)
	}
	command := timerSubscriptionTestCommand(actor.ID, target.ID, socialapp.TimerSubscriptionPerson, "timer-subscription-key-1", 0, true, now)
	first, err := repository.UpdateTimerSubscription(ctx, command)
	if err != nil || !first.Preference.Enabled || first.Preference.Revision != 1 || first.Replayed {
		t.Fatalf("update: %+v %v", first, err)
	}
	replay, err := repository.UpdateTimerSubscription(ctx, command)
	if err != nil || !replay.Replayed || replay.Preference != first.Preference {
		t.Fatalf("retry: %+v %v", replay, err)
	}
	stale := timerSubscriptionTestCommand(actor.ID, target.ID, socialapp.TimerSubscriptionPerson, "timer-subscription-key-2", 0, false, now)
	if _, err := repository.UpdateTimerSubscription(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale second client: %v", err)
	}
	failed := timerSubscriptionTestCommand(actor.ID, target.ID, socialapp.TimerSubscriptionPerson, "timer-subscription-key-3", 1, false, now)
	failed.Audit.ID = command.Audit.ID
	if _, err := repository.UpdateTimerSubscription(ctx, failed); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("duplicate audit must rollback: %v", err)
	}
	current, err := repository.GetTimerSubscription(ctx, actor.ID, socialapp.TimerSubscriptionPerson, target.ID)
	if err != nil || current != first.Preference {
		t.Fatalf("rollback persistence: %+v %v", current, err)
	}
	if err := db.Where("follower_user_id = ? AND following_user_id = ?", actor.ID, target.ID).Delete(&socialFollowModel{}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpdateTimerSubscription(ctx, command); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("retry after unfollow: %v", err)
	}
}

func timerSubscriptionTestCommand(actor, subject string, scope socialapp.TimerSubscriptionScope, key string, revision int64, enabled bool, at time.Time) socialapp.TimerSubscriptionCommand {
	digest := sha256.Sum256([]byte(key))
	return socialapp.TimerSubscriptionCommand{
		ActorUserID: actor, SubjectID: subject, Scope: scope, Enabled: enabled, ExpectedRevision: revision, OccurredAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: socialapp.UpdateTimerSubscriptionOperation, Key: key, RequestHash: digest[:]},
		Audit:       audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: audit.ResourceUpdated, Outcome: audit.Succeeded, CorrelationID: newTestID(), TargetType: "timer_subscription", TargetID: subject, OccurredAt: at},
	}
}

func TestPostgresTimerSubscriptionsParticipantDefaultAndPrivatePreference(t *testing.T) {
	f := newNudgeFixture(t, "timersub", false)
	r := NewTimerSubscriptionRepository(f.runtime.DB)
	ctx := context.Background()
	for _, actor := range []string{f.sender.ID, f.recipient.ID} {
		initial, err := r.GetTimerSubscription(ctx, actor, socialapp.TimerSubscriptionPath, f.pathID)
		if err != nil || !initial.Enabled || initial.Revision != 0 {
			t.Fatalf("participant default: %+v %v", initial, err)
		}
	}
	command := timerSubscriptionTestCommand(f.recipient.ID, f.pathID, socialapp.TimerSubscriptionPath, "timer-path-subscription-key-1", 0, false, f.now)
	changed, err := r.UpdateTimerSubscription(ctx, command)
	if err != nil || changed.Preference.Enabled || changed.Preference.Revision != 1 {
		t.Fatalf("participant update: %+v %v", changed, err)
	}
	owner, err := r.GetTimerSubscription(ctx, f.sender.ID, socialapp.TimerSubscriptionPath, f.pathID)
	if err != nil || !owner.Enabled || owner.Revision != 0 {
		t.Fatalf("changed another participant: %+v %v", owner, err)
	}
	if err := f.migration.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", f.pathID, f.recipient.ID).Update("role", "supporter").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetTimerSubscription(ctx, f.recipient.ID, socialapp.TimerSubscriptionPath, f.pathID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("supporter preference exposed: %v", err)
	}
	if _, err := r.UpdateTimerSubscription(ctx, command); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("supporter retry: %v", err)
	}
}
