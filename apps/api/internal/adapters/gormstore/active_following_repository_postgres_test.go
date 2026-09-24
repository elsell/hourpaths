package gormstore

import (
	"context"
	"sort"
	"testing"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

func TestPostgresActiveFollowingReturnsOnlyCurrentDirectEligibleFollowTimersAtParticipantBoundaries(t *testing.T) {
	if *postgresTestDSN == "" || *migrationPostgresTestDSN == "" {
		t.Skip("-database-dsn and -migration-database-dsn are required for PostgreSQL integration")
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

	now := time.Now().UTC().Truncate(time.Microsecond)
	visibility := identity.ProfileVisibilityPublic
	viewer := socialRelationshipTestUser(t, migrationStore.DB, "activeviewer", visibility, now)
	first := socialRelationshipTestUser(t, migrationStore.DB, "activefirst", visibility, now)
	second := socialRelationshipTestUser(t, migrationStore.DB, "activesecond", visibility, now)
	sharedOnly := socialRelationshipTestUser(t, migrationStore.DB, "activeshared", visibility, now)
	pending := socialRelationshipTestUser(t, migrationStore.DB, "activepending", visibility, now)
	arbitrary := socialRelationshipTestUser(t, migrationStore.DB, "activearbitrary", visibility, now)
	inactive := socialRelationshipTestUser(t, migrationStore.DB, "activeinactive", visibility, now)
	blocked := socialRelationshipTestUser(t, migrationStore.DB, "activeblocked", visibility, now)
	viewerBlocked := socialRelationshipTestUser(t, migrationStore.DB, "activeviewerblocked", visibility, now)
	archived := socialRelationshipTestUser(t, migrationStore.DB, "activearchived", visibility, now)
	users := []userModel{viewer, first, second, sharedOnly, pending, arbitrary, inactive, blocked, viewerBlocked, archived}
	if err := migrationStore.DB.Model(&userModel{}).Where("id = ?", inactive.ID).Update("status", identity.StatusDisabled).Error; err != nil {
		t.Fatal(err)
	}

	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type membershipRow struct{ PathID, UserID, Role string }
	type timerRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt                                     time.Time
	}
	type requestRow struct {
		ID, RequesterUserID, TargetUserID string
		CreatedAt                         time.Time
	}
	pathIDs := make([]string, 0, len(users))
	timerIDs := make([]string, 0, len(users)+1)
	seedTimer := func(prefix string, participant userModel, started time.Time) (string, string) {
		t.Helper()
		pathID, timerID := prefix+"-path-"+newTestID(), prefix+"-timer-"+newTestID()
		pathIDs = append(pathIDs, pathID)
		timerIDs = append(timerIDs, timerID)
		if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: participant.ID, Name: prefix + " Path", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
			t.Fatal(err)
		}
		if err := migrationStore.DB.Table("running_timer_models").Create(&timerRow{ID: timerID, PathID: pathID, ParticipantID: participant.ID, StartedAt: started, OccurrenceTimeZone: "Etc/UTC"}).Error; err != nil {
			t.Fatal(err)
		}
		return pathID, timerID
	}
	firstPath, firstTimer := seedTimer("First", first, now.Add(-time.Hour))
	secondPath, secondTimer := seedTimer("Second", second, now.Add(-30*time.Minute))
	_, secondOtherTimer := seedTimer("SecondOther", second, now.Add(-15*time.Minute))
	sharedPath, _ := seedTimer("Shared", sharedOnly, now.Add(-time.Minute))
	seedTimer("Pending", pending, now.Add(-time.Minute))
	seedTimer("Arbitrary", arbitrary, now.Add(-time.Minute))
	seedTimer("Inactive", inactive, now.Add(-time.Minute))
	seedTimer("Blocked", blocked, now.Add(-time.Minute))
	seedTimer("ViewerBlocked", viewerBlocked, now.Add(-time.Minute))
	archivedPath, _ := seedTimer("Archived", archived, now.Add(-time.Minute))
	if err := migrationStore.DB.Table("path_models").Where("id = ?", archivedPath).Updates(map[string]any{"archived_at": now, "updated_at": now}).Error; err != nil {
		t.Fatal(err)
	}

	if err := migrationStore.DB.Table("follow_models").Create([]socialFollowTestModel{
		{FollowerUserID: viewer.ID, FollowingUserID: first.ID, CreatedAt: now},
		{FollowerUserID: viewer.ID, FollowingUserID: second.ID, CreatedAt: now},
		{FollowerUserID: viewer.ID, FollowingUserID: inactive.ID, CreatedAt: now},
		{FollowerUserID: viewer.ID, FollowingUserID: blocked.ID, CreatedAt: now},
		{FollowerUserID: viewer.ID, FollowingUserID: viewerBlocked.ID, CreatedAt: now},
		{FollowerUserID: viewer.ID, FollowingUserID: archived.ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("follow_request_models").Create(&requestRow{ID: newTestID(), RequesterUserID: viewer.ID, TargetUserID: pending.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("path_membership_models").Create(&membershipRow{PathID: sharedPath, UserID: viewer.ID, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("block_models").Create([]socialBlockTestModel{
		{BlockerUserID: blocked.ID, BlockedUserID: viewer.ID, CreatedAt: now},
		{BlockerUserID: viewer.ID, BlockedUserID: viewerBlocked.ID, CreatedAt: now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		migrationStore.DB.Table("running_timer_models").Where("id IN ?", timerIDs).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("path_models").Where("id IN ?", pathIDs).Delete(&struct{ ID string }{})
		ids := make([]string, 0, len(users))
		for _, user := range users {
			ids = append(ids, user.ID)
		}
		migrationStore.DB.Table("user_models").Where("id IN ?", ids).Delete(&struct{ ID string }{})
	})

	repository := NewSocialFeedRepository(runtimeStore.DB)
	eligible := []string{first.ID, second.ID}
	sort.Strings(eligible)
	pageOne, err := repository.ListActiveTimerCandidates(context.Background(), viewer.ID, socialapp.ActiveFollowingPageRequest{Limit: 1})
	if err != nil || len(pageOne.Items) != 1 || !pageOne.HasMore || pageOne.Items[0].ParticipantID != eligible[0] {
		t.Fatalf("page one=%+v err=%v", pageOne, err)
	}
	pageTwo, err := repository.ListActiveTimerCandidates(context.Background(), viewer.ID, socialapp.ActiveFollowingPageRequest{AfterParticipantID: eligible[0], Limit: 1})
	if err != nil || len(pageTwo.Items) != 1 || pageTwo.HasMore || pageTwo.Items[0].ParticipantID != eligible[1] {
		t.Fatalf("page two=%+v err=%v", pageTwo, err)
	}
	byParticipant := map[string]socialapp.ActiveFollowingCandidate{pageOne.Items[0].ParticipantID: pageOne.Items[0], pageTwo.Items[0].ParticipantID: pageTwo.Items[0]}
	if got := byParticipant[first.ID]; len(got.Timers) != 1 || got.Timers[0].ID != firstTimer || got.Timers[0].PathID != firstPath {
		t.Fatalf("first candidate=%+v", got)
	}
	if got := byParticipant[second.ID]; len(got.Timers) != 2 || got.Timers[0].ID != secondTimer || got.Timers[1].ID != secondOtherTimer || got.Timers[0].PathID != secondPath {
		t.Fatalf("second candidate=%+v", got)
	}
	if err := migrationStore.DB.Table("path_models").Where("id = ?", firstPath).Update("visibility", "private").Error; err != nil {
		t.Fatal(err)
	}
	narrowed, err := repository.ListActiveTimerCandidates(context.Background(), viewer.ID, socialapp.ActiveFollowingPageRequest{Limit: 10})
	if err != nil || len(narrowed.Items) != 1 || narrowed.Items[0].ParticipantID != second.ID {
		t.Fatalf("after visibility narrowing=%+v err=%v", narrowed, err)
	}

	if err := migrationStore.DB.Table("running_timer_models").Where("id = ?", firstTimer).Delete(&struct{ ID string }{}).Error; err != nil {
		t.Fatal(err)
	}
	remaining, err := repository.ListActiveTimerCandidates(context.Background(), viewer.ID, socialapp.ActiveFollowingPageRequest{Limit: 10})
	if err != nil || len(remaining.Items) != 1 || remaining.Items[0].ParticipantID != second.ID {
		t.Fatalf("after stop=%+v err=%v", remaining, err)
	}
}
