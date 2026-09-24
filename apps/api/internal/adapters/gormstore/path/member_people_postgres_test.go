package pathstore

import (
	"context"
	"fmt"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestPostgresPeopleProjectionIsAvailableToCurrentMemberAndMarksOnlyTheirOwnBlock(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, administrator, viewer, blocked, reverseBlocked := "people-owner-"+suffix, "people-admin-"+suffix, "people-viewer-"+suffix, "people-blocked-"+suffix, "people-reverse-"+suffix
	pathID := "people-path-" + suffix
	now := time.Date(2026, 8, 3, 4, 15, 0, 0, time.UTC)
	seedGoalUpdateUsers(t, migrationDB, now, owner, administrator, viewer, blocked, reverseBlocked)
	for userID, username := range map[string]string{owner: "po" + suffix, administrator: "pa" + suffix, viewer: "pv" + suffix, blocked: "pb" + suffix, reverseBlocked: "pr" + suffix} {
		if err := migrationDB.Table("user_models").Where("id = ?", userID).Update("username", username).Error; err != nil {
			t.Fatal(err)
		}
	}
	seedGoalUpdatePath(t, migrationDB, domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{
		Name: "People", Visibility: "private",
		IntervalGoal:  domain.IntervalGoal{Present: true, TargetSeconds: 1_200, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 0}},
		OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 7_200},
	}, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now}, owner)
	if err := migrationDB.Model(&membershipModel{}).Where("path_id = ? AND user_id = ?", pathID, owner).Update("joined_at", now.Add(-110*time.Minute)).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Create(&[]membershipModel{
		{PathID: pathID, UserID: administrator, Role: "administrator", JoinedAt: now.Add(-100 * time.Minute)},
		{PathID: pathID, UserID: viewer, Role: "participant", JoinedAt: now.Add(-90 * time.Minute)},
		{PathID: pathID, UserID: blocked, Role: "participant", JoinedAt: now.Add(-80 * time.Minute)},
		{PathID: pathID, UserID: reverseBlocked, Role: "supporter", JoinedAt: now.Add(-70 * time.Minute)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{owner, administrator, viewer, blocked} {
		timeZone := "Etc/UTC"
		if userID == blocked {
			timeZone = "America/New_York"
		}
		if err := migrationDB.Table("user_preference_models").Create(map[string]any{"user_id": userID, "first_day_of_week": 1, "current_time_zone": timeZone, "created_at": now, "updated_at": now}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrationDB.Table("block_models").Create([]map[string]any{
		{"blocker_user_id": viewer, "blocked_user_id": blocked, "created_at": now.Add(-time.Hour)},
		{"blocker_user_id": reverseBlocked, "blocked_user_id": viewer, "created_at": now.Add(-time.Hour)},
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("recorded_activity_models").Create([]map[string]any{
		{"id": "people-activity-1-" + suffix, "path_id": pathID, "participant_id": blocked, "started_at": now.Add(-20 * time.Minute), "ended_at": now.Add(-10 * time.Minute), "occurrence_time_zone": "Etc/UTC", "created_at": now.Add(-10 * time.Minute), "updated_at": now.Add(-10 * time.Minute)},
		{"id": "people-activity-2-" + suffix, "path_id": pathID, "participant_id": blocked, "started_at": now.Add(-5 * time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now},
		{"id": "people-retained-supporter-" + suffix, "path_id": pathID, "participant_id": reverseBlocked, "started_at": now.Add(-5 * time.Minute), "ended_at": now, "occurrence_time_zone": "Etc/UTC", "created_at": now, "updated_at": now},
	}).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupDeletionFixture(migrationDB, []string{owner, administrator, viewer, blocked, reverseBlocked}, []string{pathID})
	})

	page, err := New(runtimeDB).ListMembers(context.Background(), application.MemberListQuery{ActorUserID: viewer, PathID: domain.ID(pathID)}, application.MemberPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
	if err != nil || len(page.Items) != 5 {
		t.Fatalf("ordinary-member People page=%+v err=%v", page, err)
	}
	if len(page.ProjectionFingerprint) != 64 {
		t.Fatalf("projection fingerprint=%q", page.ProjectionFingerprint)
	}
	byID := make(map[string]application.Member, len(page.Items))
	for _, member := range page.Items {
		byID[member.UserID] = member
	}
	if got := byID[blocked]; !got.BlockedByViewer || got.SessionCount != 2 || got.TotalTrackedSeconds != 900 || got.IntervalProgress == nil || *got.IntervalProgress != (application.GoalProgress{AccumulatedSeconds: 600, TargetSeconds: 1_200}) || got.OverallProgress == nil || *got.OverallProgress != (application.GoalProgress{AccumulatedSeconds: 900, TargetSeconds: 7_200}) || got.CanRemove || got.CanLeave {
		t.Fatalf("blocked Path member=%+v", got)
	}
	if got := byID[reverseBlocked]; got.BlockedByViewer || got.SessionCount != 0 || got.TotalTrackedSeconds != 0 || got.IntervalProgress != nil || got.OverallProgress != nil {
		t.Fatalf("reverse block direction leaked=%+v", got)
	}
	if got := byID[viewer]; !got.CanLeave || got.CanRemove {
		t.Fatalf("viewer capabilities=%+v", got)
	}
	adminPage, err := New(runtimeDB).ListMembers(context.Background(), application.MemberListQuery{ActorUserID: administrator, PathID: domain.ID(pathID)}, application.MemberPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
	if err != nil || len(adminPage.Items) != 5 {
		t.Fatalf("administrator People page=%+v err=%v", adminPage, err)
	}
	adminByID := make(map[string]application.Member, len(adminPage.Items))
	for _, member := range adminPage.Items {
		adminByID[member.UserID] = member
	}
	if adminByID[owner].CanRemove || !adminByID[blocked].CanRemove {
		t.Fatalf("administrator hierarchy owner=%+v ordinary=%+v", adminByID[owner], adminByID[blocked])
	}
	if !adminByID[administrator].CanStepDownAdministrator || adminByID[administrator].CanGrantAdministrator || adminByID[administrator].CanRevokeAdministrator || adminByID[blocked].CanGrantAdministrator {
		t.Fatalf("administrator lifecycle capabilities self=%+v ordinary=%+v", adminByID[administrator], adminByID[blocked])
	}
	ownerPage, err := New(runtimeDB).ListMembers(context.Background(), application.MemberListQuery{ActorUserID: owner, PathID: domain.ID(pathID)}, application.MemberPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
	if err != nil || len(ownerPage.Items) != 5 {
		t.Fatalf("creator People page=%+v err=%v", ownerPage, err)
	}
	ownerByID := make(map[string]application.Member, len(ownerPage.Items))
	for _, member := range ownerPage.Items {
		ownerByID[member.UserID] = member
	}
	if !ownerByID[blocked].CanGrantAdministrator || !ownerByID[administrator].CanRevokeAdministrator || ownerByID[administrator].CanStepDownAdministrator || ownerByID[reverseBlocked].CanGrantAdministrator || ownerByID[owner].CanGrantAdministrator {
		t.Fatalf("creator administrator lifecycle flags creator=%+v admin=%+v participant=%+v supporter=%+v", ownerByID[owner], ownerByID[administrator], ownerByID[blocked], ownerByID[reverseBlocked])
	}
	if err := migrationDB.Table("user_preference_models").Where("user_id = ?", blocked).Update("current_time_zone", "America/Chicago").Error; err != nil {
		t.Fatal(err)
	}
	changedPage, err := New(runtimeDB).ListMembers(context.Background(), application.MemberListQuery{ActorUserID: viewer, PathID: domain.ID(pathID)}, application.MemberPageRequest{Limit: 25, Snapshot: now.Add(time.Second)})
	if err != nil {
		t.Fatalf("changed participant time zone projection: %v", err)
	}
	if changedPage.ProjectionFingerprint == page.ProjectionFingerprint {
		t.Fatalf("projection fingerprint did not change after participant time zone changed: %q", page.ProjectionFingerprint)
	}
	if err := migrationDB.Table("user_preference_models").Where("user_id = ?", blocked).Update("current_time_zone", "Local").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := New(runtimeDB).ListMembers(context.Background(), application.MemberListQuery{ActorUserID: viewer, PathID: domain.ID(pathID)}, application.MemberPageRequest{Limit: 25, Snapshot: now.Add(time.Second)}); err == nil {
		t.Fatal("malformed participant time zone must fail the People projection closed")
	}
}
