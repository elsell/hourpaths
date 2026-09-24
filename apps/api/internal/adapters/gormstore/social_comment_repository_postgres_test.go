package gormstore

import (
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

func TestPracticeCommentForeignKeyRaceIsOpaque(t *testing.T) {
	if err := classifySocialCommentError(gorm.ErrForeignKeyViolated); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("foreign-key race error=%v, want opaque not found", err)
	}
}

func TestPostgresPracticeCommentsCreatePageEditHistoryReplayCASAndOwnerDelete(t *testing.T) {
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
	assertRuntimeSocialCommentPrivileges(t, runtimeStore)

	now := time.Now().UTC().Truncate(time.Microsecond)
	actor := socialRelationshipTestUser(t, migrationStore.DB, "commenter", identity.ProfileVisibilityPublic, now)
	owner := socialRelationshipTestUser(t, migrationStore.DB, "commentowner", identity.ProfileVisibilityPublic, now)
	pathID, activityID := "comment-path-"+newTestID(), "comment-activity-"+newTestID()
	eventID := "practice:" + activityID
	t.Cleanup(func() {
		migrationStore.DB.Table("path_models").Where("id = ?", pathID).Delete(&struct{ ID string }{})
		migrationStore.DB.Table("user_models").Where("id IN ?", []string{actor.ID, owner.ID}).Delete(&struct{ ID string }{})
	})
	type pathRow struct {
		ID, OwnerUserID, Name, Visibility string
		CreatedAt, UpdatedAt              time.Time
	}
	type activityRow struct {
		ID, PathID, ParticipantID, OccurrenceTimeZone string
		StartedAt, EndedAt, CreatedAt, UpdatedAt      time.Time
	}
	if err := migrationStore.DB.Table("path_models").Create(&pathRow{ID: pathID, OwnerUserID: owner.ID, Name: "Piano", Visibility: "public", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("recorded_activity_models").Create(&activityRow{ID: activityID, PathID: pathID, ParticipantID: owner.ID, OccurrenceTimeZone: "Etc/UTC", StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-30 * time.Minute), CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := migrationStore.DB.Table("follow_models").Create(&socialFollowTestModel{FollowerUserID: actor.ID, FollowingUserID: owner.ID, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	repository := NewSocialFeedRepository(runtimeStore.DB)
	target, err := repository.ResolvePracticeCommentTarget(context.Background(), actor.ID, eventID)
	if err != nil || target != (socialapp.PracticeCommentTarget{EventID: eventID, PathID: pathID, OwnerUserID: owner.ID}) {
		t.Fatalf("target=%+v err=%v", target, err)
	}
	create := socialCommentTestCommand(actor.ID, target, "Caf\u00e9!\n\U0001f389", "", 0, socialapp.CreatePracticeCommentOperation, "comment-create-001", now)
	created, err := repository.CreatePracticeComment(context.Background(), create)
	if err != nil || created.Replayed || !created.Comment.Valid() || created.Comment.AuthorID != actor.ID || created.Comment.EventID != eventID || created.Comment.Version != 1 || created.Comment.Text != create.Text {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	assertCommentNotification(t, migrationStore.DB, created.Comment.ID, eventID, actor.ID, owner.ID, 1, create.NotificationEligibleAt)

	replay, found, err := repository.FindPracticeCommentReplay(context.Background(), create.Idempotency)
	if err != nil || !found || replay.ActorUserID != actor.ID || replay.EventID != eventID || !commentsEqual(replay.Comment, created.Comment) || replay.Deleted {
		t.Fatalf("replay=%+v found=%v err=%v", replay, found, err)
	}
	recreated, err := repository.CreatePracticeComment(context.Background(), create)
	if err != nil || !recreated.Replayed || !commentsEqual(recreated.Comment, created.Comment) {
		t.Fatalf("recreated=%+v err=%v", recreated, err)
	}
	conflictingIdempotency := create.Idempotency
	conflictingIdempotency.RequestHash = append([]byte(nil), create.Idempotency.RequestHash...)
	conflictingIdempotency.RequestHash[0] ^= 0xff
	if _, _, err := repository.FindPracticeCommentReplay(context.Background(), conflictingIdempotency); !errors.Is(err, ports.ErrIdempotencyConflict) {
		t.Fatalf("conflicting replay error=%v", err)
	}

	selfCreate := socialCommentTestCommand(owner.ID, target, "Owner response", "", 0, socialapp.CreatePracticeCommentOperation, "comment-create-002", now.Add(time.Second))
	selfCreate.NotifyEventOwner = false
	second, err := repository.CreatePracticeComment(context.Background(), selfCreate)
	if err != nil || second.Comment.AuthorID != owner.ID {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	assertCommentNotification(t, migrationStore.DB, second.Comment.ID, eventID, owner.ID, owner.ID, 0, time.Time{})

	heart := socialCommentHeartTestCommand(owner.ID, target, created.Comment, true, "comment-heart-set01", now.Add(1500*time.Millisecond))
	hearted, err := repository.SetPracticeCommentHeart(context.Background(), heart)
	if err != nil || hearted.CommentID != created.Comment.ID || hearted.HeartCount != 1 || !hearted.HeartedByViewer {
		t.Fatalf("hearted=%+v err=%v", hearted, err)
	}
	assertCommentHeartNotification(t, migrationStore.DB, created.Comment.ID, owner.ID, actor.ID, 1, heart.NotificationEligibleAt)
	heartReplay, found, err := repository.FindPracticeCommentHeartReplay(context.Background(), heart.Idempotency)
	if err != nil || !found || !heartReplay.Hearted || heartReplay.Summary != hearted {
		t.Fatalf("heart replay=%+v found=%v err=%v", heartReplay, found, err)
	}
	replayedHeart, err := repository.SetPracticeCommentHeart(context.Background(), heart)
	if err != nil || replayedHeart != hearted {
		t.Fatalf("replayed heart=%+v err=%v", replayedHeart, err)
	}
	roster, err := repository.ListPracticeCommentHearts(context.Background(), actor.ID, eventID, created.Comment.ID, socialapp.CommentHeartRosterPageRequest{Snapshot: now.Add(2 * time.Second), Limit: 1})
	if err != nil || len(roster.Items) != 1 || roster.HasMore || roster.Items[0].Profile.ID != owner.ID || !roster.Items[0].HeartedAt.Equal(heart.OccurredAt) {
		t.Fatalf("heart roster=%+v err=%v", roster, err)
	}
	unheart := socialCommentHeartTestCommand(owner.ID, target, created.Comment, false, "comment-heart-del01", now.Add(1750*time.Millisecond))
	unhearted, err := repository.RemovePracticeCommentHeart(context.Background(), unheart)
	if err != nil || unhearted.CommentID != created.Comment.ID || unhearted.HeartCount != 0 || unhearted.HeartedByViewer {
		t.Fatalf("unhearted=%+v err=%v", unhearted, err)
	}
	assertCommentHeartNotification(t, migrationStore.DB, created.Comment.ID, owner.ID, actor.ID, 0, time.Time{})
	replayedHeart, err = repository.SetPracticeCommentHeart(context.Background(), heart)
	if err != nil || replayedHeart != hearted {
		t.Fatalf("set(K1), remove(K2), set(K1) replay=%+v err=%v, want original result=%+v", replayedHeart, err, hearted)
	}
	currentHeart, err := commentHeartSummary(runtimeStore.DB, created.Comment.ID, owner.ID)
	if err != nil || currentHeart != unhearted {
		t.Fatalf("cross-operation replay changed current state=%+v err=%v, want=%+v", currentHeart, err, unhearted)
	}
	assertCommentHeartNotification(t, migrationStore.DB, created.Comment.ID, owner.ID, actor.ID, 0, time.Time{})

	page, err := repository.ListPracticeComments(context.Background(), actor.ID, eventID, socialapp.CommentPageRequest{Snapshot: now.Add(2 * time.Second), Limit: 1})
	if err != nil || len(page.Items) != 1 || !page.HasMore || !commentsEqual(page.Items[0].Comment, created.Comment) || page.Items[0].Author.ID != actor.ID || page.Items[0].Author.Username != *actor.Username {
		t.Fatalf("first page=%+v err=%v", page, err)
	}
	next, err := repository.ListPracticeComments(context.Background(), actor.ID, eventID, socialapp.CommentPageRequest{AfterID: created.Comment.ID, AfterCreated: created.Comment.CreatedAt, Snapshot: now.Add(2 * time.Second), Limit: 1})
	if err != nil || len(next.Items) != 1 || next.HasMore || !commentsEqual(next.Items[0].Comment, second.Comment) {
		t.Fatalf("next page=%+v err=%v", next, err)
	}

	resolved, err := repository.ResolvePracticeComment(context.Background(), actor.ID, eventID, created.Comment.ID)
	if err != nil || resolved.Target != target || !commentsEqual(resolved.Comment, created.Comment) {
		t.Fatalf("resolved=%+v err=%v", resolved, err)
	}
	edit := socialCommentTestCommand(actor.ID, target, "Edited text", created.Comment.ID, 1, socialapp.EditPracticeCommentOperation, "comment-edit-0001", now.Add(2*time.Second))
	edited, err := repository.EditPracticeComment(context.Background(), edit)
	if err != nil || edited.Replayed || edited.Comment.Version != 2 || edited.Comment.Text != edit.Text || !edited.Comment.CreatedAt.Equal(created.Comment.CreatedAt) || !edited.Comment.UpdatedAt.Equal(edit.OccurredAt) {
		t.Fatalf("edited=%+v err=%v", edited, err)
	}
	editedReplay, err := repository.EditPracticeComment(context.Background(), edit)
	if err != nil || !editedReplay.Replayed || !commentsEqual(editedReplay.Comment, edited.Comment) {
		t.Fatalf("edited replay=%+v err=%v", editedReplay, err)
	}
	stale := socialCommentTestCommand(actor.ID, target, "Stale overwrite", created.Comment.ID, 1, socialapp.EditPracticeCommentOperation, "comment-edit-stale", now.Add(3*time.Second))
	if _, err := repository.EditPracticeComment(context.Background(), stale); !errors.Is(err, ports.ErrConflict) {
		t.Fatalf("stale edit error=%v", err)
	}

	history, err := repository.ListCommentHistory(context.Background(), actor.ID, eventID, created.Comment.ID, socialapp.CommentHistoryPageRequest{Snapshot: now.Add(3 * time.Second), Limit: 1})
	if err != nil || len(history.Versions) != 1 || !history.HasMore || history.Versions[0].Version != 1 || history.Versions[0].Text != created.Comment.Text || !history.Versions[0].CreatedAt.Equal(created.Comment.CreatedAt) {
		t.Fatalf("history=%+v err=%v", history, err)
	}
	historyNext, err := repository.ListCommentHistory(context.Background(), actor.ID, eventID, created.Comment.ID, socialapp.CommentHistoryPageRequest{AfterVersion: 1, AfterCreated: created.Comment.CreatedAt, Snapshot: now.Add(3 * time.Second), Limit: 1})
	if err != nil || len(historyNext.Versions) != 1 || historyNext.HasMore || historyNext.Versions[0].Version != 2 || historyNext.Versions[0].Text != edited.Comment.Text || !historyNext.Versions[0].CreatedAt.Equal(edited.Comment.UpdatedAt) {
		t.Fatalf("history next=%+v err=%v", historyNext, err)
	}
	reheart := socialCommentHeartTestCommand(owner.ID, target, edited.Comment, true, "comment-heart-set02", now.Add(3500*time.Millisecond))
	if _, err := repository.SetPracticeCommentHeart(context.Background(), reheart); err != nil {
		t.Fatalf("reheart before source deletion: %v", err)
	}
	assertCommentHeartNotification(t, migrationStore.DB, created.Comment.ID, owner.ID, actor.ID, 1, reheart.NotificationEligibleAt)

	deleteCommand := socialCommentTestCommand(owner.ID, target, "", created.Comment.ID, 0, socialapp.DeletePracticeCommentOperation, "comment-delete-001", now.Add(4*time.Second))
	deleted, err := repository.DeletePracticeComment(context.Background(), deleteCommand)
	if err != nil || deleted.Replayed || !deleted.Deleted || deleted.CommentID != created.Comment.ID {
		t.Fatalf("deleted=%+v err=%v", deleted, err)
	}
	assertCommentRemoved(t, migrationStore.DB, created.Comment.ID)
	redeleted, err := repository.DeletePracticeComment(context.Background(), deleteCommand)
	if err != nil || !redeleted.Replayed || !redeleted.Deleted || redeleted.CommentID != created.Comment.ID {
		t.Fatalf("redeleted=%+v err=%v", redeleted, err)
	}

	down, err := dbmigrations.Files.ReadFile("000048_practice_event_comments.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	downSQL := strings.TrimSpace(string(down))
	downSQL = strings.TrimSpace(strings.TrimPrefix(downSQL, "BEGIN;"))
	downSQL = strings.TrimSpace(strings.TrimSuffix(downSQL, "COMMIT;"))
	tx := migrationStore.DB.Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback().Error })
	if err := tx.Exec(downSQL).Error; err == nil || !strings.Contains(err.Error(), "cannot remove practice comments while comment data exists") { // hourpaths-direct-sql: allow rollback-only execution of embedded migration
		t.Fatalf("down migration with replay-only tombstone error=%v, want data-preservation refusal", err)
	}
}

func socialCommentTestCommand(actor string, target socialapp.PracticeCommentTarget, text, commentID string, expectedVersion int64, operation, key string, at time.Time) socialapp.CommentCommand {
	payload := operation + "\x00" + target.EventID
	if commentID != "" {
		payload += "\x00" + commentID
	}
	if expectedVersion > 0 {
		payload += "\x00" + strconv.FormatInt(expectedVersion, 10)
	}
	if text != "" {
		payload += "\x00" + text
	}
	digest := sha256.Sum256([]byte(payload))
	action, targetID := audit.ResourceCreated, target.EventID
	if operation == socialapp.EditPracticeCommentOperation {
		action, targetID = audit.ResourceUpdated, commentID
	} else if operation == socialapp.DeletePracticeCommentOperation {
		action, targetID = audit.ResourceDeleted, commentID
	}
	command := socialapp.CommentCommand{
		ActorUserID: actor, Target: target, CommentID: commentID, Text: text, ExpectedVersion: expectedVersion, OccurredAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]},
		Audit:       audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: action, TargetType: "practice_comment", TargetID: targetID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at},
	}
	if operation == socialapp.CreatePracticeCommentOperation {
		command.NotificationEligibleAt = at.Add(5 * time.Second)
		command.NotifyEventOwner = actor != target.OwnerUserID
	}
	return command
}

func socialCommentHeartTestCommand(actor string, target socialapp.PracticeCommentTarget, comment socialdomain.Comment, hearted bool, key string, at time.Time) socialapp.CommentHeartCommand {
	operation, action := socialapp.RemovePracticeCommentHeartOperation, audit.ResourceDeleted
	if hearted {
		operation, action = socialapp.SetPracticeCommentHeartOperation, audit.ResourceCreated
	}
	digest := sha256.Sum256([]byte(operation + "\x00" + target.EventID + "\x00" + comment.ID))
	command := socialapp.CommentHeartCommand{ActorUserID: actor, Target: target, Comment: comment, OccurredAt: at,
		Idempotency: ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]},
		Audit:       audit.Event{ID: newTestID(), OwnerUserID: actor, ActorUserID: actor, Action: action, TargetType: "practice_comment_heart", TargetID: comment.ID, Outcome: audit.Succeeded, CorrelationID: newTestID(), OccurredAt: at}}
	if hearted {
		command.NotificationEligibleAt, command.NotifyCommentAuthor = at.Add(5*time.Second), actor != comment.AuthorID
	}
	return command
}

func assertCommentHeartNotification(t *testing.T, db *gorm.DB, commentID, actor, recipient string, want int64, createdAt time.Time) {
	t.Helper()
	var count int64
	query := db.Table("notification_models").Where("kind = 'comment_heart' AND comment_id = ? AND actor_user_id = ? AND recipient_user_id = ?", commentID, actor, recipient)
	if err := query.Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("comment heart notification count=%d want=%d", count, want)
	}
	if want == 1 {
		var row struct{ CreatedAt time.Time }
		if err := query.Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		if !row.CreatedAt.Equal(createdAt) {
			t.Fatalf("comment heart notification created_at=%v want=%v", row.CreatedAt, createdAt)
		}
	}
}

func assertCommentNotification(t *testing.T, db *gorm.DB, commentID, eventID, actor, owner string, want int64, createdAt time.Time) {
	t.Helper()
	var count int64
	query := db.Table("notification_models").Where("kind = 'practice_comment' AND comment_id = ? AND social_feed_event_id = ? AND actor_user_id = ? AND recipient_user_id = ?", commentID, eventID, actor, owner)
	if err := query.Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("notification count=%d want=%d", count, want)
	}
	if want == 1 {
		var row struct{ CreatedAt time.Time }
		if err := query.Take(&row).Error; err != nil {
			t.Fatal(err)
		}
		if !row.CreatedAt.Equal(createdAt) {
			t.Fatalf("notification created_at=%v want=%v", row.CreatedAt, createdAt)
		}
	}
}

func commentsEqual(left, right socialdomain.Comment) bool {
	return left.ID == right.ID && left.EventID == right.EventID && left.AuthorID == right.AuthorID &&
		left.Text == right.Text && left.Version == right.Version &&
		left.CreatedAt.Equal(right.CreatedAt) && left.UpdatedAt.Equal(right.UpdatedAt)
}

func assertCommentRemoved(t *testing.T, db *gorm.DB, commentID string) {
	t.Helper()
	for _, table := range []string{"social_practice_comment_models", "social_practice_comment_revision_models", "social_practice_comment_heart_models", "notification_models"} {
		var count int64
		query := db.Table(table)
		if table == "notification_models" {
			query = query.Where("comment_id = ?", commentID)
		} else if table == "social_practice_comment_heart_models" {
			query = query.Where("comment_id = ?", commentID)
		} else {
			column := "id"
			if table == "social_practice_comment_revision_models" {
				column = "comment_id"
			}
			query = query.Where(column+" = ?", commentID)
		}
		if err := query.Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s rows=%d want 0", table, count)
		}
	}
}

func assertRuntimeSocialCommentPrivileges(t *testing.T, store *Store) {
	t.Helper()
	type privileges struct{ CanSelect, CanInsert, CanUpdate, CanDelete, CanTruncate bool }
	checks := map[string]privileges{
		"social_practice_comment_models":              {CanSelect: true, CanInsert: true, CanUpdate: true, CanDelete: true},
		"social_practice_comment_revision_models":     {CanSelect: true, CanInsert: true},
		"social_practice_comment_replay_models":       {CanSelect: true, CanInsert: true},
		"social_practice_comment_heart_models":        {CanSelect: true, CanInsert: true, CanDelete: true},
		"social_practice_comment_heart_replay_models": {CanSelect: true, CanInsert: true},
	}
	for table, want := range checks {
		var got privileges
		if err := store.DB.Raw(`SELECT
has_table_privilege(current_user, ?, 'SELECT') AS can_select,
has_table_privilege(current_user, ?, 'INSERT') AS can_insert,
has_table_privilege(current_user, ?, 'UPDATE') AS can_update,
has_table_privilege(current_user, ?, 'DELETE') AS can_delete,
has_table_privilege(current_user, ?, 'TRUNCATE') AS can_truncate`, "public."+table, "public."+table, "public."+table, "public."+table, "public."+table).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("runtime %s privileges=%+v want=%+v", table, got, want)
		}
	}
}
