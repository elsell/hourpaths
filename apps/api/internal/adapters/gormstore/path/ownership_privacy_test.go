package pathstore

import (
	"bytes"
	"context"
	"fmt"
	rootstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"reflect"
	"testing"
	"time"
)

func TestPostgresOwnershipPrivateRecipientNarrowsAndReplaysAudience(t *testing.T) {
	runtimeDB, migrationDB := goalUpdateDatabases(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	prefix := fmt.Sprintf("pt%d", now.UnixNano())
	creator, recipient, pathID := prefix+"-owner", prefix+"-recipient", prefix+"-path"
	seedInvitationUser(t, migrationDB, creator, prefix+".owner", "public", now)
	seedInvitationUser(t, migrationDB, recipient, prefix+".recipient", "private", now)
	path := domain.Entity{ID: domain.ID(pathID), OwnerUserID: creator, Attributes: domain.Attributes{Name: "Transfer privacy", Visibility: "public"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	seedGoalUpdatePath(t, migrationDB, path, creator)
	if err := migrationDB.Create(&membershipModel{PathID: pathID, UserID: recipient, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := NewOwnershipTransferRepository(runtimeDB)
	transfer := mustOwnershipTransfer(t, prefix+"-request", path.ID, creator, recipient, now, now.Add(time.Hour))
	if _, err := repository.Initiate(ctx, initiateTransferCommand(transfer, prefix+"-init", bytes.Repeat([]byte{1}, 32), prefix+"-created")); err != nil {
		t.Fatal(err)
	}
	accepted, _ := transfer.Accept(recipient, now.Add(time.Minute))
	command := acceptTransferCommand(accepted, prefix+"-accept", 2, prefix+"-accepted")
	store := &rootstore.Store{DB: runtimeDB}
	reconciler := app.App{AuthorizationBatchOutbox: store, AuthorizationSerializer: store, Audits: store, RelationshipWriter: privacyRelationshipWriter{}, Clock: privacyTransferClock{}}
	service := pathapp.NewOwnershipTransferService(pathapp.OwnershipTransferDependencies{Auth: privacyTransferAuth{recipient}, Profiles: privacyTransferProfile{}, Transfers: repository, Audits: store, AuditRateLimiter: privacyTransferLimiter{}, Clock: privacyFixedClock{now.Add(time.Minute)}, NewID: func() string { return fmt.Sprintf("event-%d", time.Now().UnixNano()) }, AuthorizationReconciler: privacyCheckedReconciler{reconciler, store}, AuthorizationWorker: "privacy-test-worker"})
	result, err := service.Accept(ctx, "Bearer test", transfer.ID, command.Idempotency.Key)

	if err != nil {
		t.Fatal(err)
	}
	if result.Path.Visibility != "followers" || result.Path.OwnerUserID != recipient {
		t.Fatalf("private owner received %+v", result.Path)
	}
	updates := result.AuthorizationBatch.Updates
	if len(updates) != 6 || updates[4] != (ports.RelationshipUpdate{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: pathID, Relation: "public_viewer", SubjectType: "user", SubjectID: "*"}) || updates[5] != (ports.RelationshipUpdate{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: pathID, Relation: "followers_owner", SubjectType: "user", SubjectID: recipient}) {
		t.Fatalf("audience batch: %+v", updates)
	}
	replay, err := service.Accept(ctx, "Bearer test", transfer.ID, command.Idempotency.Key)
	if err != nil || !replay.Replayed || !reflect.DeepEqual(replay.AuthorizationBatch, result.AuthorizationBatch) || !reflect.DeepEqual(replay.RelationshipUpdates, updates) {
		t.Fatalf("replay %+v, %v", replay, err)
	}
	assertOwnershipRoles(t, migrationDB, pathID, recipient, map[string]string{creator: "administrator", recipient: "participant"})
}

func TestPostgresOwnershipPrivacyConcurrentAcceptance(t *testing.T) {
	runtimeDB, db := goalUpdateDatabases(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Now().UTC().Truncate(time.Microsecond)
	prefix := fmt.Sprintf("pc%d", now.UnixNano())
	owner, recipient, pathID := prefix+"o", prefix+"r", prefix+"p"
	seedInvitationUser(t, db, owner, prefix+"o", "public", now)
	seedInvitationUser(t, db, recipient, prefix+"r", "public", now)
	path := domain.Entity{ID: domain.ID(pathID), OwnerUserID: owner, Attributes: domain.Attributes{Name: "Concurrent privacy", Visibility: "public"}, CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)}
	seedGoalUpdatePath(t, db, path, owner)
	if err := db.Create(&membershipModel{PathID: pathID, UserID: recipient, Role: "participant"}).Error; err != nil {
		t.Fatal(err)
	}
	transfers := NewOwnershipTransferRepository(runtimeDB)
	transfer := mustOwnershipTransfer(t, prefix+"t", path.ID, owner, recipient, now, now.Add(time.Hour))
	if _, err := transfers.Initiate(ctx, initiateTransferCommand(transfer, prefix+"init", bytes.Repeat([]byte{1}, 32), prefix+"created")); err != nil {
		t.Fatal(err)
	}
	accepted, _ := transfer.Accept(recipient, now.Add(time.Second))
	n := 0
	nextID := func() string { n++; return fmt.Sprintf("%s-event-%d", prefix, n) }
	privacy := app.ProfilePrivacyCommand{ActorUserID: recipient, Update: app.ProfilePrivacyUpdate{Visibility: "private", ExpectedRevision: 1, Confirmed: true}, ChangedAt: now, NewID: nextID, AuthorizationWorker: "privacy-worker", AuthorizationLease: time.Minute, Idempotency: ports.Idempotency{PrincipalID: recipient, Operation: "account.profile.privacy", Key: prefix + "privacy", RequestHash: bytes.Repeat([]byte{2}, 32)}, Audit: audit.Event{ID: nextID(), OwnerUserID: recipient, ActorUserID: recipient, Action: audit.ResourceUpdated, TargetType: "user", TargetID: recipient, Outcome: audit.Succeeded, CorrelationID: nextID(), OccurredAt: now}}
	start := make(chan struct{})
	done := make(chan error, 2)
	go func() { <-start; _, err := New(runtimeDB).UpdateOwnProfilePrivacy(ctx, privacy); done <- err }()
	go func() {
		<-start
		_, err := transfers.Accept(ctx, acceptTransferCommand(accepted, prefix+"accept", 3, prefix+"accepted"))
		done <- err
	}()
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	var row model
	if err := db.Where("id = ?", pathID).Take(&row).Error; err != nil || row.OwnerUserID != recipient || row.Visibility != "followers" {
		t.Fatalf("race result %+v %v", row, err)
	}
}

type privacyTransferClock struct{}

func (privacyTransferClock) Now() time.Time { return time.Now().UTC() }

type privacyRelationshipWriter struct{}

func (privacyRelationshipWriter) WriteRelationships(_ context.Context, changes []ports.RelationshipUpdate) error {
	if len(changes) != 6 {
		return fmt.Errorf("expected atomic audience batch, got %d", len(changes))
	}
	return nil
}

type privacyTransferAuth struct{ user string }

func (a privacyTransferAuth) Authenticate(context.Context, string) (ports.Principal, error) {
	return ports.Principal{UserID: a.user, Scopes: []string{"api:user"}}, nil
}

type privacyTransferProfile struct{}

func (privacyTransferProfile) TimeZone(context.Context, string) (string, error) {
	return "Etc/UTC", nil
}
func (privacyTransferProfile) PathCreationProfile(context.Context, string) (pathapp.PathCreationProfile, error) {
	return pathapp.PathCreationProfile{}, nil
}

type privacyTransferLimiter struct{}

func (privacyTransferLimiter) Allow(string, time.Time) bool { return true }

type privacyFixedClock struct{ now time.Time }

func (c privacyFixedClock) Now() time.Time { return c.now }

type privacyCheckedReconciler struct {
	app   app.App
	store *rootstore.Store
}

func (r privacyCheckedReconciler) ReconcileAuthorizationBatch(ctx context.Context, id, worker string) error {
	if _, err := r.store.AuthorizationBatchCompleted(ctx, id); err != nil {
		return fmt.Errorf("completion read: %w", err)
	}
	return r.app.ReconcileAuthorizationBatch(ctx, id, worker)
}
