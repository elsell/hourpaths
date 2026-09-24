package social

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestSetAchievementCommentHeartAuthorizesAndSchedulesOneNotification(t *testing.T) {
	now := time.Date(2026, 7, 28, 18, 0, 0, 123456789, time.UTC)
	canonical := now.Truncate(time.Microsecond)
	comment := validComment("comment-1", "achievement:goal", "author", 1, canonical.Add(-time.Hour), canonical.Add(-time.Hour), "Nice")
	repository := &controlledComments{
		resolved:     ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: comment.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: comment},
		heartSummary: CommentHeartSummary{CommentID: comment.ID, HeartCount: 3, HeartedByViewer: true},
	}
	service, authorizer, _ := commentService(repository)
	service.Clock = fixedClock{now: now}

	got, err := service.SetPracticeCommentHeart(context.Background(), "Bearer session", comment.EventID, comment.ID, "heart-comment-0001")
	if err != nil || got != repository.heartSummary {
		t.Fatalf("summary=%+v err=%v", got, err)
	}
	command := repository.heartCommand
	if command == nil || command.ActorUserID != "viewer" || command.Comment != comment || command.Target != repository.resolved.Target ||
		command.OccurredAt != canonical || command.NotificationEligibleAt != canonical.Add(5*time.Second) || !command.NotifyCommentAuthor || !command.Audit.Valid() {
		t.Fatalf("command=%+v", command)
	}
	if command.Idempotency.Operation != SetPracticeCommentHeartOperation || command.Idempotency.Key != "heart-comment-0001" || len(authorizer.checks) != 1 {
		t.Fatalf("idempotency=%+v checks=%v", command.Idempotency, authorizer.checks)
	}

	repository.resolved.Comment.AuthorID = "viewer"
	repository.heartSummary = CommentHeartSummary{CommentID: comment.ID, HeartCount: 1, HeartedByViewer: true}
	_, err = service.SetPracticeCommentHeart(context.Background(), "Bearer session", comment.EventID, comment.ID, "heart-comment-0002")
	if err != nil || repository.heartCommand.NotifyCommentAuthor {
		t.Fatalf("self-heart command=%+v err=%v", repository.heartCommand, err)
	}
}

func TestRemovePracticeCommentHeartIsOpaqueAndCancelsSilently(t *testing.T) {
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	comment := validComment("comment-1", "practice:activity", "author", 1, now.Add(-time.Hour), now.Add(-time.Hour), "Nice")
	repository := &controlledComments{
		resolved:     ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: comment.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: comment},
		heartSummary: CommentHeartSummary{CommentID: comment.ID},
	}
	service, _, _ := commentService(repository)
	service.Clock = fixedClock{now: now}
	got, err := service.RemovePracticeCommentHeart(context.Background(), "Bearer session", comment.EventID, comment.ID, "unheart-comment-01")
	if err != nil || got != repository.heartSummary || repository.heartRemoved == nil || !repository.heartRemoved.NotificationEligibleAt.IsZero() || repository.heartRemoved.NotifyCommentAuthor {
		t.Fatalf("summary=%+v command=%+v err=%v", got, repository.heartRemoved, err)
	}

	repository.commentErr = ports.ErrNotFound
	repository.heartFound = true
	repository.heartReplay = PracticeCommentHeartReplay{ActorUserID: "viewer", EventID: comment.EventID, CommentID: comment.ID, Hearted: false, Summary: CommentHeartSummary{CommentID: comment.ID}}
	repository.heartRemoved = nil
	_, err = service.RemovePracticeCommentHeart(context.Background(), "Bearer session", comment.EventID, comment.ID, "unheart-comment-02")
	if !errors.Is(err, ports.ErrNotFound) || repository.heartRemoved != nil {
		t.Fatalf("opaque remove command=%+v err=%v", repository.heartRemoved, err)
	}
}

func TestListPracticeCommentHeartsPagesSafeProfilesAndBindsCursor(t *testing.T) {
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	comment := validComment("comment-1", "practice:activity", "author", 1, now.Add(-time.Hour), now.Add(-time.Hour), "Nice")
	profiles := []domain.PublicProfile{
		{ID: "user-1", Username: "alex", DisplayName: "Alex", Relationship: domain.RelationshipFollowing},
		{ID: "user-2", Username: "bea", DisplayName: "Bea", Relationship: domain.RelationshipNone},
	}
	repository := &controlledComments{
		resolved:  ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: comment.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: comment},
		heartPage: CommentHeartRosterPage{Items: []CommentHeartRosterItem{{Profile: profiles[0], HeartedAt: now.Add(-time.Minute)}, {Profile: profiles[1], HeartedAt: now}}, HasMore: true},
	}
	service, _, audits := commentService(repository)
	service.Clock = fixedClock{now: now}
	items, cursor, err := service.ListPracticeCommentHearts(context.Background(), "Bearer session", comment.EventID, comment.ID, "", 2)
	if err != nil || len(items) != 2 || items[0] != profiles[0] || cursor == "" {
		t.Fatalf("items=%+v cursor=%q err=%v", items, cursor, err)
	}
	if repository.heartRequest.Snapshot != now || len(audits.events) != 1 || audits.events[0].Action != audit.ResourceListed {
		t.Fatalf("request=%+v audits=%+v", repository.heartRequest, audits.events)
	}
	repository.heartPage = CommentHeartRosterPage{}
	_, _, err = service.ListPracticeCommentHearts(context.Background(), "Bearer session", comment.EventID, "comment-other", cursor, 2)
	if !errors.Is(err, ports.ErrInvalidArgument) || repository.resolvedID != comment.ID {
		t.Fatalf("cross-comment cursor reached repository id=%q err=%v", repository.resolvedID, err)
	}
}

func TestVisibleCommentHeartProjectionMustBeAuthoritative(t *testing.T) {
	now := time.Date(2026, 7, 28, 18, 0, 0, 0, time.UTC)
	comment := validComment("comment-1", "practice:activity", "author", 1, now.Add(-time.Minute), now.Add(-time.Minute), "Nice")
	item := commentItem(comment)
	item.HeartCount, item.HeartedByViewer = 2, true
	repository := &controlledComments{target: PracticeCommentTarget{EventID: comment.EventID, PathID: "path", OwnerUserID: "owner"}, commentPage: CommentPage{Items: []PracticeCommentItem{item}}}
	service, _, _ := commentService(repository)
	service.Clock = fixedClock{now: now}
	items, _, err := service.ListPracticeComments(context.Background(), "Bearer session", comment.EventID, "", 20)
	if err != nil || items[0].HeartCount != 2 || !items[0].HeartedByViewer {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	repository.commentPage.Items[0].HeartCount = -1
	_, _, err = service.ListPracticeComments(context.Background(), "Bearer session", comment.EventID, "", 20)
	if !errors.Is(err, errInvalidDependencies) {
		t.Fatalf("negative count err=%v", err)
	}
}
