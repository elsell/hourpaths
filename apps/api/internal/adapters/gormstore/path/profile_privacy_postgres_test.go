package pathstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPostgresProfilePrivacyAtomicNarrowingAndRetry(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	owner, other := "privacy-owner-"+suffix, "privacy-other-"+suffix
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedInvitationUser(t, migrationDB, owner, "owner."+suffix, identity.ProfileVisibilityPublic, now)
	seedInvitationUser(t, migrationDB, other, "other."+suffix, identity.ProfileVisibilityPublic, now)
	ids := []string{}
	for i, visibility := range []string{"public", "public", "followers", "private", "public"} {
		id := fmt.Sprintf("privacy-%s-%d", suffix, i)
		ids = append(ids, id)
		user := owner
		if i == 4 {
			user = other
		}
		path := domain.Entity{ID: domain.ID(id), OwnerUserID: user, Attributes: domain.Attributes{Name: "Practice", Visibility: visibility}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}
		seedGoalUpdatePath(t, migrationDB, path, user)
		if i == 1 {
			if err := migrationDB.Model(&model{}).Where("id = ?", id).Update("archived_at", now.Add(-time.Second)).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Cleanup(func() { cleanupInvitationFixture(t, migrationDB, []string{owner, other}, ids) })
	n := 0
	nextID := func() string { n++; return fmt.Sprintf("privacy-%s-event-%d", suffix, n) }
	command := app.ProfilePrivacyCommand{ActorUserID: owner, Update: app.ProfilePrivacyUpdate{Visibility: identity.ProfileVisibilityPrivate, ExpectedRevision: 1, Confirmed: true}, ChangedAt: now, NewID: nextID, AuthorizationWorker: "privacy-worker", AuthorizationLease: time.Minute, Idempotency: ports.Idempotency{PrincipalID: owner, Operation: "account.profile.privacy", Key: "privacy-change-key-0001", RequestHash: bytes.Repeat([]byte{1}, 32)}, Audit: audit.Event{ID: nextID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceUpdated, TargetType: "user", TargetID: owner, Outcome: audit.Succeeded, CorrelationID: nextID(), OccurredAt: now}}
	repository := New(runtimeDB)
	if err := migrationDB.Create(fromAudit(command.Audit)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repository.UpdateOwnProfilePrivacy(context.Background(), command); err == nil {
		t.Fatal("narrowing accepted duplicate audit")
	}
	initial, err := repository.GetOwnProfilePrivacy(context.Background(), owner)
	if err != nil || initial.Visibility != identity.ProfileVisibilityPublic || initial.Revision != 1 {
		t.Fatalf("failed narrowing changed profile=%+v err=%v", initial, err)
	}
	var retained model
	if err := migrationDB.Where("id = ?", ids[0]).Take(&retained).Error; err != nil || retained.Visibility != "public" {
		t.Fatalf("failed narrowing changed path=%+v err=%v", retained, err)
	}
	command.Audit.ID = nextID()
	saved, err := repository.UpdateOwnProfilePrivacy(context.Background(), command)
	if err != nil || saved.Profile.Revision != 2 || saved.Profile.Visibility != identity.ProfileVisibilityPrivate || len(saved.AffectedPathIDs) != 2 {
		t.Fatalf("save=%+v err=%v", saved, err)
	}
	for i, want := range []string{"followers", "followers", "followers", "private", "public"} {
		var row model
		if err := migrationDB.Where("id = ?", ids[i]).Take(&row).Error; err != nil || row.Visibility != want {
			t.Fatalf("path=%+v want=%s err=%v", row, want, err)
		}
	}
	if err := runtimeDB.Model(&privacyMutationModel{}).Where("user_id = ?", owner).Update("visibility", "public").Error; err == nil {
		t.Fatal("runtime can alter privacy replay")
	}
	if err := runtimeDB.Where("user_id = ?", owner).Delete(&privacyMutationModel{}).Error; err == nil {
		t.Fatal("runtime can delete privacy replay")
	}
	replay, err := repository.UpdateOwnProfilePrivacy(context.Background(), command)
	if err != nil || replay.Profile != saved.Profile || len(replay.AffectedPathIDs) != 2 {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	mismatch := command
	mismatch.Idempotency.RequestHash = bytes.Repeat([]byte{2}, 32)
	if _, err := repository.UpdateOwnProfilePrivacy(context.Background(), mismatch); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("key conflict=%v", err)
	}
	stale := command
	stale.Idempotency.Key = "privacy-change-key-0002"
	if _, err := repository.UpdateOwnProfilePrivacy(context.Background(), stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale=%v", err)
	}
	// A failing atomic audit must roll back even the profile-only expansion.
	expand := stale
	expand.Update.ExpectedRevision = 2
	expand.Update.Visibility = identity.ProfileVisibilityPublic
	if _, err := repository.UpdateOwnProfilePrivacy(context.Background(), expand); err == nil {
		t.Fatal("duplicate audit did not roll back")
	}
	current, err := repository.GetOwnProfilePrivacy(context.Background(), owner)
	if err != nil || current != saved.Profile {
		t.Fatalf("rollback=%+v err=%v", current, err)
	}
	expand.Audit.ID = nextID()
	if _, err := repository.UpdateOwnProfilePrivacy(context.Background(), expand); err != nil {
		t.Fatal(err)
	}
	var publicCount, noticeCount int64
	if err := migrationDB.Model(&model{}).Where("owner_user_id = ? AND visibility = 'public'", owner).Count(&publicCount).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationDB.Table("notification_models").Where("path_id IN ?", ids).Count(&noticeCount).Error; err != nil {
		t.Fatal(err)
	}
	if publicCount != 0 || noticeCount != 0 {
		t.Fatalf("public paths=%d notices=%d", publicCount, noticeCount)
	}
	// Creation racing a privacy transition may commit before narrowing or be
	// rejected afterward; it must never leave a public Path under a private owner.
	concurrent := command
	concurrent.Update.ExpectedRevision = 3
	concurrent.Idempotency.Key = "privacy-change-key-0003"
	concurrent.Audit.ID = nextID()
	createAt := now.Add(time.Minute)
	createdPath := domain.Entity{ID: domain.ID("privacy-racing-" + suffix), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Concurrent", Visibility: "public"}, CreatedAt: createAt, UpdatedAt: createAt}
	ids = append(ids, string(createdPath.ID))
	createAudit := audit.Event{ID: nextID(), OwnerUserID: owner, ActorUserID: owner, Action: audit.ResourceCreated, TargetType: "path", TargetID: string(createdPath.ID), Outcome: audit.Succeeded, CorrelationID: nextID(), OccurredAt: createAt}
	creator := ports.AuthorizationChange{ID: nextID(), ResourceType: "path", ResourceID: string(createdPath.ID), Relation: "creator", SubjectType: "user", SubjectID: owner, OwnerUserID: owner, ActorUserID: owner, Operation: ports.AuthorizationTouch, LockedBy: "privacy-create-worker", Lease: time.Minute}
	createKey := ports.Idempotency{PrincipalID: owner, Operation: "path.create", Key: "privacy-racing-create-0001", RequestHash: bytes.Repeat([]byte{3}, 32)}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	start := make(chan struct{})
	privacyDone := make(chan error, 1)
	createDone := make(chan error, 1)
	go func() { <-start; _, err := repository.UpdateOwnProfilePrivacy(ctx, concurrent); privacyDone <- err }()
	go func() {
		<-start
		_, _, err := repository.Create(ctx, createdPath, creator, createKey, createAudit)
		createDone <- err
	}()
	close(start)
	if err := <-privacyDone; err != nil {
		t.Fatalf("concurrent privacy: %v", err)
	}
	if err := <-createDone; err != nil && !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("concurrent creation: %v", err)
	}
	if err := migrationDB.Model(&model{}).Where("owner_user_id = ? AND visibility = 'public'", owner).Count(&publicCount).Error; err != nil || publicCount != 0 {
		t.Fatalf("race left public paths=%d err=%v", publicCount, err)
	}

}
