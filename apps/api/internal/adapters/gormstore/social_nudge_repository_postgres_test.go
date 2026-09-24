package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestNudgeSendLocksProgressBeforeSocialAndPathRows(t *testing.T) {
	source, err := os.ReadFile("social_nudge_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	progress := strings.Index(text, "progresslock.Lock(tx, nudge.RecipientID, nudge.PathID)")
	socialPair := strings.Index(text, "lockNudgePair(tx, nudge.SenderID, nudge.RecipientID, nudge.PathID)")
	pathRows := strings.Index(text, "lockNudgePathParticipants(tx, nudge.SenderID, nudge.RecipientID, nudge.PathID)")
	if progress < 0 || socialPair < 0 || pathRows < 0 || progress > socialPair || socialPair > pathRows {
		t.Fatal("nudge send must lock progress before social, Path, and membership rows")
	}
}

func TestNudgeDuplicatePersistenceFailureIsUnavailable(t *testing.T) {
	err := classifyNudgePersistenceError(gorm.ErrDuplicatedKey)
	if !errors.Is(err, ports.ErrUnavailable) || errors.Is(err, socialapp.ErrNudgeAlreadySent) {
		t.Fatalf("duplicate persistence failure=%v", err)
	}
}

type nudgeFixture struct {
	runtime, migration *Store
	repository         *SocialFeedRepository
	sender, recipient  userModel
	pathID             string
	now                time.Time
}

func newNudgeFixture(t *testing.T, prefix string, withGoal bool) nudgeFixture {
	t.Helper()
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("PostgreSQL DSNs required")
	}
	runtimeStore, err := Open("postgres", *postgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	migrationStore, err := Open("postgres", *migrationPostgresTestDSN)
	if err != nil {
		t.Fatal(err)
	}
	closeSocialProfileTestStore(t, runtimeStore)
	closeSocialProfileTestStore(t, migrationStore)
	now := time.Date(2026, 8, 5, 17, 0, 0, 0, time.UTC)
	sender := socialRelationshipTestUser(t, migrationStore.DB, prefix+"sender", identity.ProfileVisibilityPublic, now)
	recipient := socialRelationshipTestUser(t, migrationStore.DB, prefix+"recipient", identity.ProfileVisibilityPublic, now)
	pathID := prefix + "-path-" + newTestID()
	path := map[string]any{"id": pathID, "owner_user_id": sender.ID, "name": "Piano", "visibility": "public", "created_at": now.Add(-time.Hour), "updated_at": now.Add(-time.Hour)}
	if withGoal {
		path["interval_goal_target_seconds"] = int64(60)
		path["interval_goal_recurrence"] = "daily"
		path["interval_goal_start_hour"] = int16(0)
	}
	if err := migrationStore.DB.Table("path_models").Create(path).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("path_membership_models").Create(map[string]any{"path_id": pathID, "user_id": recipient.ID, "role": "participant", "joined_at": now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("user_preference_models").Create(map[string]any{"user_id": recipient.ID, "first_day_of_week": 1, "current_time_zone": "America/New_York", "created_at": now.Add(-time.Hour), "updated_at": now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{sender.ID, recipient.ID}).Delete(&struct{ ID string }{})
	})
	return nudgeFixture{runtime: runtimeStore, migration: migrationStore, repository: NewSocialFeedRepository(runtimeStore.DB), sender: sender, recipient: recipient, pathID: pathID, now: now}
}

func nudgePreferenceTestCommand(actor, path, key string, expected int64, audience socialdomain.NudgeAudience, at time.Time) socialapp.NudgePreferenceCommand {
	digest := sha256.Sum256([]byte(string(audience)))
	return socialapp.NudgePreferenceCommand{ActorUserID: actor, PathID: path, Audience: audience, ExpectedRevision: expected, ChangedAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: socialapp.UpdateNudgeAudienceOperation, Key: key, RequestHash: digest[:]},
		Audit:       audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: audit.ResourceUpdated, TargetType: "nudge_preference", TargetID: path, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}}
}

func nudgeSendTestCommand(id, sender, recipient, path, key string, preset socialdomain.NudgePreset, at time.Time) socialapp.NudgeCommand {
	digest := sha256.Sum256([]byte(path + "\x00" + recipient + "\x00" + string(preset)))
	nudge := socialdomain.Nudge{ID: id, SenderID: sender, RecipientID: recipient, PathID: path, Content: socialdomain.NudgeContent{Kind: socialdomain.NudgeContentPreset, Preset: preset}, SentAt: at}
	return socialapp.NudgeCommand{Nudge: nudge, Idempotency: ports.Idempotency{PrincipalID: sender, Operation: socialapp.SendNudgeOperation, Key: key, RequestHash: digest[:]}, Audit: audit.Event{ID: newTestID(), OwnerUserID: sender, ActorUserID: sender, Action: audit.ResourceCreated, TargetType: "nudge", TargetID: id, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}}
}

func TestPostgresNudgePreferenceDefaultsRevisionReplayConflictAndOwnTrackingScope(t *testing.T) {
	fixture := newNudgeFixture(t, "nudgepref", false)
	ctx := context.Background()
	initial, err := fixture.repository.GetNudgeAudience(ctx, fixture.recipient.ID, fixture.pathID)
	if err != nil || initial.PathID != fixture.pathID || initial.UserID != fixture.recipient.ID || initial.Audience != socialdomain.DefaultNudgeAudience || initial.Revision != 0 || !initial.UpdatedAt.IsZero() {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	command := nudgePreferenceTestCommand(fixture.recipient.ID, fixture.pathID, "nudge-pref-key-0001", 0, socialdomain.NudgeAudienceEveryone, fixture.now)
	updated, err := fixture.repository.UpdateNudgeAudience(ctx, command)
	if err != nil || updated.Audience != socialdomain.NudgeAudienceEveryone || updated.Revision != 1 || updated.UpdatedAt != fixture.now {
		t.Fatalf("updated=%+v err=%v", updated, err)
	}
	replayed, err := fixture.repository.UpdateNudgeAudience(ctx, command)
	if err != nil || replayed != updated {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	conflict := command
	conflict.Idempotency.RequestHash = bytesOf(9)
	if _, err := fixture.repository.UpdateNudgeAudience(ctx, conflict); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflict=%v", err)
	}
	stale := nudgePreferenceTestCommand(fixture.recipient.ID, fixture.pathID, "nudge-pref-key-0002", 0, socialdomain.NudgeAudienceNobody, fixture.now.Add(time.Second))
	if _, err := fixture.repository.UpdateNudgeAudience(ctx, stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale=%v", err)
	}
	var audits int64
	if err := fixture.migration.DB.Table("audit_event_models").Where("target_type = 'nudge_preference' AND target_id = ?", fixture.pathID).Count(&audits).Error; err != nil || audits != 1 {
		t.Fatalf("audits=%d err=%v", audits, err)
	}
	if err := fixture.migration.DB.Table("path_membership_models").Where("path_id = ? AND user_id = ?", fixture.pathID, fixture.recipient.ID).Update("role", "supporter").Error; err != nil {
		t.Fatal(err)
	}
	var preferences int64
	if err := fixture.migration.DB.Table("path_nudge_preference_models").Where("path_id = ? AND user_id = ?", fixture.pathID, fixture.recipient.ID).Count(&preferences).Error; err != nil || preferences != 0 {
		t.Fatalf("ineligible preference count=%d err=%v", preferences, err)
	}
	if _, err := fixture.repository.GetNudgeAudience(ctx, fixture.recipient.ID, fixture.pathID); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("supporter preference visibility=%v", err)
	}
}

func TestPostgresNudgeAudienceMatrixSupporterSenderBlocksAndArchive(t *testing.T) {
	fixture := newNudgeFixture(t, "nudgematrix", false)
	ctx := context.Background()
	outsider := socialRelationshipTestUser(t, fixture.migration.DB, "nmout", identity.ProfileVisibilityPublic, fixture.now)
	supporter := socialRelationshipTestUser(t, fixture.migration.DB, "nmsup", identity.ProfileVisibilityPublic, fixture.now)
	t.Cleanup(func() {
		fixture.migration.DB.Table("user_models").Where("id IN ?", []string{outsider.ID, supporter.ID}).Delete(&struct{ ID string }{})
	})
	if err := fixture.migration.DB.Table("path_membership_models").Create(map[string]any{"path_id": fixture.pathID, "user_id": supporter.ID, "role": "supporter", "joined_at": fixture.now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}

	assertEligible := func(sender string) {
		t.Helper()
		got, err := fixture.repository.GetNudgeEligibility(ctx, sender, fixture.recipient.ID, fixture.pathID, fixture.now)
		if err != nil || !got.Eligible {
			t.Fatalf("sender=%s eligibility=%+v err=%v", sender, got, err)
		}
	}
	assertDenied := func(sender string, want error) {
		t.Helper()
		if _, err := fixture.repository.GetNudgeEligibility(ctx, sender, fixture.recipient.ID, fixture.pathID, fixture.now); !errors.Is(err, want) {
			t.Fatalf("sender=%s denial=%v want=%v", sender, err, want)
		}
	}

	assertEligible(fixture.sender.ID)
	assertEligible(supporter.ID)
	assertDenied(outsider.ID, socialapp.ErrNudgeAudienceDenied)

	followers := nudgePreferenceTestCommand(fixture.recipient.ID, fixture.pathID, "nudge-matrix-pref-01", 0, socialdomain.NudgeAudienceFollowers, fixture.now)
	if _, err := fixture.repository.UpdateNudgeAudience(ctx, followers); err != nil {
		t.Fatal(err)
	}
	assertDenied(outsider.ID, socialapp.ErrNudgeAudienceDenied)
	if err := fixture.migration.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: outsider.ID, FollowingUserID: fixture.recipient.ID, CreatedAt: fixture.now}).Error; err != nil {
		t.Fatal(err)
	}
	assertEligible(outsider.ID)

	everyone := nudgePreferenceTestCommand(fixture.recipient.ID, fixture.pathID, "nudge-matrix-pref-02", 1, socialdomain.NudgeAudienceEveryone, fixture.now.Add(time.Second))
	if _, err := fixture.repository.UpdateNudgeAudience(ctx, everyone); err != nil {
		t.Fatal(err)
	}
	if err := fixture.migration.DB.Table("follow_models").Where("follower_user_id = ? AND following_user_id = ?", outsider.ID, fixture.recipient.ID).Delete(&socialFollowTestModel{}).Error; err != nil {
		t.Fatal(err)
	}
	assertEligible(outsider.ID)
	if err := fixture.migration.DB.Table("path_models").Where("id = ?", fixture.pathID).Update("visibility", "private").Error; err != nil {
		t.Fatal(err)
	}
	assertDenied(outsider.ID, ports.ErrNotFound)
	assertEligible(fixture.sender.ID)
	assertEligible(supporter.ID)
	if err := fixture.migration.DB.Table("path_models").Where("id = ?", fixture.pathID).Update("visibility", "followers").Error; err != nil {
		t.Fatal(err)
	}
	assertDenied(outsider.ID, ports.ErrNotFound)
	if err := fixture.migration.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: outsider.ID, FollowingUserID: fixture.sender.ID, CreatedAt: fixture.now}).Error; err != nil {
		t.Fatal(err)
	}
	assertEligible(outsider.ID)
	if err := fixture.migration.DB.Table("follow_models").Where("follower_user_id = ? AND following_user_id = ?", outsider.ID, fixture.sender.ID).Delete(&socialFollowTestModel{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.migration.DB.Table("path_models").Where("id = ?", fixture.pathID).Update("visibility", "public").Error; err != nil {
		t.Fatal(err)
	}

	nobody := nudgePreferenceTestCommand(fixture.recipient.ID, fixture.pathID, "nudge-matrix-pref-03", 2, socialdomain.NudgeAudienceNobody, fixture.now.Add(2*time.Second))
	if _, err := fixture.repository.UpdateNudgeAudience(ctx, nobody); err != nil {
		t.Fatal(err)
	}
	assertDenied(fixture.sender.ID, socialapp.ErrNudgeAudienceDenied)
	rejected := nudgeSendTestCommand("nudge-"+newTestID(), fixture.sender.ID, fixture.recipient.ID, fixture.pathID, "nudge-matrix-send-01", socialdomain.NudgeLetsGo, fixture.now.Add(2*time.Second))
	if _, err := fixture.repository.SendNudge(ctx, rejected); !errors.Is(err, socialapp.ErrNudgeAudienceDenied) {
		t.Fatalf("audience-denied send=%v", err)
	}
	assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 0, 0, 0, 0)

	everyone = nudgePreferenceTestCommand(fixture.recipient.ID, fixture.pathID, "nudge-matrix-pref-04", 3, socialdomain.NudgeAudienceEveryone, fixture.now.Add(3*time.Second))
	if _, err := fixture.repository.UpdateNudgeAudience(ctx, everyone); err != nil {
		t.Fatal(err)
	}
	if err := fixture.migration.DB.Table("block_models").Create(&socialBlockTestModel{BlockerUserID: fixture.recipient.ID, BlockedUserID: outsider.ID, CreatedAt: fixture.now}).Error; err != nil {
		t.Fatal(err)
	}
	assertDenied(outsider.ID, ports.ErrNotFound)
	if err := fixture.migration.DB.Table("block_models").Where("blocker_user_id = ? AND blocked_user_id = ?", fixture.recipient.ID, outsider.ID).Delete(&socialBlockTestModel{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := fixture.migration.DB.Table("path_models").Where("id = ?", fixture.pathID).Update("archived_at", fixture.now).Error; err != nil {
		t.Fatal(err)
	}
	assertDenied(outsider.ID, ports.ErrNotFound)
}

func TestPostgresNudgeAudienceEligibilityGoalAndRollingLimit(t *testing.T) {
	fixture := newNudgeFixture(t, "nudgeelig", true)
	ctx := context.Background()
	eligible, err := fixture.repository.GetNudgeEligibility(ctx, fixture.sender.ID, fixture.recipient.ID, fixture.pathID, fixture.now)
	if err != nil || !eligible.Eligible {
		t.Fatalf("eligible=%+v err=%v", eligible, err)
	}
	start := fixture.now.Add(-30 * time.Minute)
	if err := fixture.migration.DB.Table("recorded_activity_models").Create(map[string]any{"id": "nudge-activity-" + newTestID(), "path_id": fixture.pathID, "participant_id": fixture.recipient.ID, "started_at": start, "ended_at": start.Add(time.Minute), "occurrence_time_zone": "America/New_York", "created_at": start.Add(time.Minute), "updated_at": start.Add(time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	complete, err := fixture.repository.GetNudgeEligibility(ctx, fixture.sender.ID, fixture.recipient.ID, fixture.pathID, fixture.now)
	if err != nil || complete.Eligible || complete.Reason != socialapp.NudgeGoalCompleteReason {
		t.Fatalf("complete=%+v err=%v", complete, err)
	}

	rolling := newNudgeFixture(t, "nudgeroll", false)
	command := nudgeSendTestCommand("nudge-"+newTestID(), rolling.sender.ID, rolling.recipient.ID, rolling.pathID, "nudge-send-key-0001", socialdomain.NudgeLetsGo, rolling.now)
	if _, err := rolling.repository.SendNudge(ctx, command); err != nil {
		t.Fatal(err)
	}
	limited, err := rolling.repository.GetNudgeEligibility(ctx, rolling.sender.ID, rolling.recipient.ID, rolling.pathID, rolling.now.Add(time.Hour))
	if err != nil || limited.Eligible || limited.Reason != socialapp.NudgeRateLimitedReason {
		t.Fatalf("limited=%+v err=%v", limited, err)
	}
	boundary, err := rolling.repository.GetNudgeEligibility(ctx, rolling.sender.ID, rolling.recipient.ID, rolling.pathID, rolling.now.Add(24*time.Hour))
	if err != nil || !boundary.Eligible {
		t.Fatalf("boundary=%+v err=%v", boundary, err)
	}
}

func TestPostgresNudgeIntervalLimitUsesRecipientOccurrenceAndIndependentSender(t *testing.T) {
	fixture := newNudgeFixture(t, "nudgeinterval", true)
	ctx := context.Background()
	first := nudgeSendTestCommand("nudge-"+newTestID(), fixture.sender.ID, fixture.recipient.ID, fixture.pathID, "nudge-interval-key-01", socialdomain.NudgeLetsGo, fixture.now)
	if _, err := fixture.repository.SendNudge(ctx, first); err != nil {
		t.Fatal(err)
	}
	limited, err := fixture.repository.GetNudgeEligibility(ctx, fixture.sender.ID, fixture.recipient.ID, fixture.pathID, fixture.now.Add(time.Hour))
	if err != nil || limited.Eligible || limited.Reason != socialapp.NudgeRateLimitedReason {
		t.Fatalf("same occurrence eligibility=%+v err=%v", limited, err)
	}

	other := socialRelationshipTestUser(t, fixture.migration.DB, "niother", identity.ProfileVisibilityPublic, fixture.now)
	t.Cleanup(func() {
		fixture.migration.DB.Table("user_models").Where("id = ?", other.ID).Delete(&struct{ ID string }{})
	})
	if err := fixture.migration.DB.Table("path_membership_models").Create(map[string]any{"path_id": fixture.pathID, "user_id": other.ID, "role": "supporter", "joined_at": fixture.now.Add(-time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	independent := nudgeSendTestCommand("nudge-"+newTestID(), other.ID, fixture.recipient.ID, fixture.pathID, "nudge-interval-key-02", socialdomain.NudgeKeepItGoing, fixture.now.Add(time.Hour))
	if _, err := fixture.repository.SendNudge(ctx, independent); err != nil {
		t.Fatal(err)
	}
	next, err := fixture.repository.GetNudgeEligibility(ctx, fixture.sender.ID, fixture.recipient.ID, fixture.pathID, fixture.now.Add(24*time.Hour))
	if err != nil || !next.Eligible {
		t.Fatalf("next recipient occurrence eligibility=%+v err=%v", next, err)
	}
	assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 2, 2, 2, 2)
}

func TestPostgresNudgeSendChannelSuppressionDeliveryReplayRollbackAndConcurrency(t *testing.T) {
	fixture := newNudgeFixture(t, "nudgesend", false)
	ctx := context.Background()
	if err := fixture.migration.DB.Table("notification_channel_preference_models").Create(map[string]any{"user_id": fixture.recipient.ID, "channel": "nudges", "enabled": false, "revision": 1, "created_at": fixture.now, "updated_at": fixture.now}).Error; err != nil {
		t.Fatal(err)
	}
	command := nudgeSendTestCommand("nudge-"+newTestID(), fixture.sender.ID, fixture.recipient.ID, fixture.pathID, "nudge-send-key-0001", socialdomain.NudgeKeepItGoing, fixture.now)
	result, err := fixture.repository.SendNudge(ctx, command)
	if err != nil || result.Nudge != command.Nudge || result.Replayed {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	replay, err := fixture.repository.SendNudge(ctx, command)
	if err != nil || replay.Nudge != command.Nudge || !replay.Replayed {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	conflict := command
	conflict.Idempotency.RequestHash = bytesOf(7)
	if _, err := fixture.repository.SendNudge(ctx, conflict); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("idempotency conflict=%v", err)
	}
	assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 1, 1, 0, 0)

	if err := fixture.migration.DB.Table("notification_channel_preference_models").Where("user_id = ? AND channel = 'nudges'", fixture.recipient.ID).Updates(map[string]any{"enabled": true, "revision": 2, "updated_at": fixture.now.Add(24 * time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	delivered := nudgeSendTestCommand("nudge-"+newTestID(), fixture.sender.ID, fixture.recipient.ID, fixture.pathID, "nudge-send-key-0002", socialdomain.NudgeTimeToWork, fixture.now.Add(24*time.Hour))
	if _, err := fixture.repository.SendNudge(ctx, delivered); err != nil {
		t.Fatal(err)
	}
	assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 2, 2, 1, 1)

	rollback := nudgeSendTestCommand("nudge-"+newTestID(), fixture.sender.ID, fixture.recipient.ID, fixture.pathID, "nudge-send-key-0003", socialdomain.NudgeLetsGo, fixture.now.Add(48*time.Hour+time.Second))
	rollback.Audit.ID = command.Audit.ID
	if _, err := fixture.repository.SendNudge(ctx, rollback); !errors.Is(err, ports.ErrUnavailable) {
		t.Fatalf("rollback error=%v", err)
	}
	assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 2, 2, 1, 1)

	concurrent := newNudgeFixture(t, "nudgeconcurrent", false)
	commands := []socialapp.NudgeCommand{
		nudgeSendTestCommand("nudge-"+newTestID(), concurrent.sender.ID, concurrent.recipient.ID, concurrent.pathID, "nudge-send-key-0004", socialdomain.NudgeLetsGo, concurrent.now),
		nudgeSendTestCommand("nudge-"+newTestID(), concurrent.sender.ID, concurrent.recipient.ID, concurrent.pathID, "nudge-send-key-0005", socialdomain.NudgeTimeToWork, concurrent.now),
	}
	var wait sync.WaitGroup
	errs := make([]error, 2)
	for index := range commands {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			_, errs[index] = concurrent.repository.SendNudge(ctx, commands[index])
		}(index)
	}
	wait.Wait()
	successes, limited := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, socialapp.ErrNudgeAlreadySent) {
			limited++
		} else {
			t.Fatalf("concurrent error=%v", err)
		}
	}
	if successes != 1 || limited != 1 {
		t.Fatalf("concurrent errors=%v", errs)
	}
	assertNudgeCounts(t, concurrent.migration.DB, concurrent.pathID, 1, 1, 1, 1)
}

func TestPostgresNudgeSendWaitsForRecipientPathProgressAndRevalidates(t *testing.T) {
	fixture := newNudgeFixture(t, "nudgerace", true)
	ctx := context.Background()
	tx := fixture.migration.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })
	if err := progresslock.Lock(tx, fixture.recipient.ID, fixture.pathID); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	command := nudgeSendTestCommand("nudge-"+newTestID(), fixture.sender.ID, fixture.recipient.ID, fixture.pathID, "nudge-race-key-0001", socialdomain.NudgeLetsGo, fixture.now)
	go func() {
		_, err := fixture.repository.SendNudge(ctx, command)
		result <- err
	}()
	select {
	case err := <-result:
		t.Fatalf("send escaped recipient progress lock: %v", err)
	case <-time.After(250 * time.Millisecond):
	}

	start := fixture.now.Add(-time.Minute)
	committedAfterSendStarted := fixture.now.Add(time.Second)
	if err := tx.Table("recorded_activity_models").Create(map[string]any{"id": "nudge-race-activity-" + newTestID(), "path_id": fixture.pathID, "participant_id": fixture.recipient.ID, "started_at": start, "ended_at": fixture.now, "occurrence_time_zone": "America/New_York", "created_at": committedAfterSendStarted, "updated_at": committedAfterSendStarted}).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, socialapp.ErrNudgeGoalComplete) {
			t.Fatalf("send revalidation error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("send did not resume after recipient progress commit")
	}
	assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 0, 0, 0, 0)
}

func TestPostgresNudgeSendSerializesBlockAndPathAuthorizationRevocation(t *testing.T) {
	t.Run("block", func(t *testing.T) {
		fixture := newNudgeFixture(t, "nudgeblockrace", false)
		tx := fixture.migration.DB.Begin()
		if tx.Error != nil {
			t.Fatal(tx.Error)
		}
		t.Cleanup(func() { tx.Rollback() })
		if err := lockSocialPair(tx, fixture.sender.ID, fixture.recipient.ID); err != nil {
			t.Fatal(err)
		}
		result := beginNudgeSend(t, fixture, "nudge-block-race-key")
		assertNudgeSendStillWaiting(t, result)
		if err := tx.Table("block_models").Create(&socialBlockTestModel{BlockerUserID: fixture.recipient.ID, BlockedUserID: fixture.sender.ID, CreatedAt: fixture.now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit().Error; err != nil {
			t.Fatal(err)
		}
		assertNudgeSendDenied(t, result)
		assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 0, 0, 0, 0)
	})

	t.Run("archive", func(t *testing.T) {
		fixture := newNudgeFixture(t, "nudgearchiverace", false)
		tx := fixture.migration.DB.Begin()
		if tx.Error != nil {
			t.Fatal(tx.Error)
		}
		t.Cleanup(func() { tx.Rollback() })
		var path struct{ ID string }
		if err := tx.Table("path_models").Select("id").Where("id = ?", fixture.pathID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&path).Error; err != nil {
			t.Fatal(err)
		}
		result := beginNudgeSend(t, fixture, "nudge-archive-race-key")
		assertNudgeSendStillWaiting(t, result)
		if err := tx.Table("path_models").Where("id = ?", fixture.pathID).Updates(map[string]any{"archived_at": fixture.now, "updated_at": fixture.now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit().Error; err != nil {
			t.Fatal(err)
		}
		assertNudgeSendDenied(t, result)
		assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 0, 0, 0, 0)
	})

	t.Run("recipient role downgrade", func(t *testing.T) {
		fixture := newNudgeFixture(t, "nudgerolerace", false)
		tx := fixture.migration.DB.Begin()
		if tx.Error != nil {
			t.Fatal(tx.Error)
		}
		t.Cleanup(func() { tx.Rollback() })
		var membership struct{ UserID string }
		if err := tx.Table("path_membership_models").Select("user_id").Where("path_id = ? AND user_id = ?", fixture.pathID, fixture.recipient.ID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&membership).Error; err != nil {
			t.Fatal(err)
		}
		result := beginNudgeSend(t, fixture, "nudge-role-race-key")
		assertNudgeSendStillWaiting(t, result)
		if err := tx.Table("path_membership_models").Where("path_id = ? AND user_id = ?", fixture.pathID, fixture.recipient.ID).Update("role", "supporter").Error; err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit().Error; err != nil {
			t.Fatal(err)
		}
		assertNudgeSendDenied(t, result)
		assertNudgeCounts(t, fixture.migration.DB, fixture.pathID, 0, 0, 0, 0)
	})

}

func beginNudgeSend(t *testing.T, fixture nudgeFixture, key string) <-chan error {
	t.Helper()
	result := make(chan error, 1)
	command := nudgeSendTestCommand("nudge-"+newTestID(), fixture.sender.ID, fixture.recipient.ID, fixture.pathID, key, socialdomain.NudgeLetsGo, fixture.now)
	go func() {
		_, err := fixture.repository.SendNudge(context.Background(), command)
		result <- err
	}()
	return result
}

func assertNudgeSendStillWaiting(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		t.Fatalf("nudge escaped authorization lock: %v", err)
	case <-time.After(250 * time.Millisecond):
	}
}

func assertNudgeSendDenied(t *testing.T, result <-chan error) {
	t.Helper()
	select {
	case err := <-result:
		if !errors.Is(err, ports.ErrNotFound) {
			t.Fatalf("nudge authorization revalidation error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nudge did not resume after authorization change")
	}
}

func assertNudgeCounts(t *testing.T, db *gorm.DB, path string, nudges, replays, notifications, outboxes int64) {
	t.Helper()
	checks := []struct {
		table, where string
		want         int64
	}{
		{"social_nudge_models", "path_id = ?", nudges},
		{"social_nudge_replay_models", "path_id = ?", replays},
		{"notification_models", "path_id = ? AND kind = 'nudge_received'", notifications},
		{"notification_push_outbox_models", "notification_id IN (SELECT id FROM notification_models WHERE path_id = ? AND kind = 'nudge_received')", outboxes},
		{"audit_event_models", "target_type = 'nudge' AND target_id IN (SELECT id FROM social_nudge_models WHERE path_id = ?)", nudges},
	}
	for _, check := range checks {
		var count int64
		if err := db.Table(check.table).Where(check.where, path).Count(&count).Error; err != nil || count != check.want {
			t.Fatalf("%s count=%d want=%d err=%v", check.table, count, check.want, err)
		}
	}
}

func bytesOf(value byte) []byte {
	result := make([]byte, sha256.Size)
	for index := range result {
		result[index] = value
	}
	return result
}
