package social

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledComments struct {
	target         PracticeCommentTarget
	resolved       ResolvedPracticeComment
	commentPage    CommentPage
	historyPage    CommentHistoryPage
	mutation       CommentMutationResult
	deleteResult   CommentDeleteResult
	replay         PracticeCommentReplay
	replayFound    bool
	replayErr      error
	replayRequest  ports.Idempotency
	err            error
	targetErr      error
	commentErr     error
	listErr        error
	historyErr     error
	mutationErr    error
	resolvedViewer string
	resolvedEvent  string
	resolvedID     string
	listViewer     string
	listEvent      string
	listRequest    CommentPageRequest
	historyViewer  string
	historyEvent   string
	historyID      string
	historyRequest CommentHistoryPageRequest
	created        *CommentCommand
	edited         *CommentCommand
	deleted        *CommentCommand
	heartReplay    PracticeCommentHeartReplay
	heartFound     bool
	heartSummary   CommentHeartSummary
	heartPage      CommentHeartRosterPage
	heartCommand   *CommentHeartCommand
	heartRemoved   *CommentHeartCommand
	heartRequest   CommentHeartRosterPageRequest
}

func (repository *controlledComments) FindPracticeCommentReplay(_ context.Context, idempotency ports.Idempotency) (PracticeCommentReplay, bool, error) {
	repository.replayRequest = idempotency
	return repository.replay, repository.replayFound, repository.replayErr
}

func (repository *controlledComments) ResolvePracticeCommentTarget(_ context.Context, viewer, eventID string) (PracticeCommentTarget, error) {
	repository.resolvedViewer, repository.resolvedEvent = viewer, eventID
	return repository.target, firstCommentError(repository.targetErr, repository.err)
}

func (repository *controlledComments) ResolvePracticeComment(_ context.Context, viewer, eventID, commentID string) (ResolvedPracticeComment, error) {
	repository.resolvedViewer, repository.resolvedEvent, repository.resolvedID = viewer, eventID, commentID
	return repository.resolved, firstCommentError(repository.commentErr, repository.err)
}

func (repository *controlledComments) ListPracticeComments(_ context.Context, viewer, eventID string, request CommentPageRequest) (CommentPage, error) {
	repository.listViewer, repository.listEvent, repository.listRequest = viewer, eventID, request
	return repository.commentPage, firstCommentError(repository.listErr, repository.err)
}

func (repository *controlledComments) ListCommentHistory(_ context.Context, viewer, eventID, commentID string, request CommentHistoryPageRequest) (CommentHistoryPage, error) {
	repository.historyViewer, repository.historyEvent, repository.historyID, repository.historyRequest = viewer, eventID, commentID, request
	return repository.historyPage, firstCommentError(repository.historyErr, repository.err)
}

func (repository *controlledComments) CreatePracticeComment(_ context.Context, command CommentCommand) (CommentMutationResult, error) {
	repository.created = &command
	return repository.mutation, firstCommentError(repository.mutationErr, repository.err)
}

func (repository *controlledComments) EditPracticeComment(_ context.Context, command CommentCommand) (CommentMutationResult, error) {
	repository.edited = &command
	return repository.mutation, firstCommentError(repository.mutationErr, repository.err)
}

func (repository *controlledComments) DeletePracticeComment(_ context.Context, command CommentCommand) (CommentDeleteResult, error) {
	repository.deleted = &command
	return repository.deleteResult, firstCommentError(repository.mutationErr, repository.err)
}

func (repository *controlledComments) FindPracticeCommentHeartReplay(_ context.Context, idempotency ports.Idempotency) (PracticeCommentHeartReplay, bool, error) {
	repository.replayRequest = idempotency
	return repository.heartReplay, repository.heartFound, repository.replayErr
}

func (repository *controlledComments) SetPracticeCommentHeart(_ context.Context, command CommentHeartCommand) (CommentHeartSummary, error) {
	repository.heartCommand = &command
	return repository.heartSummary, firstCommentError(repository.mutationErr, repository.err)
}

func (repository *controlledComments) RemovePracticeCommentHeart(_ context.Context, command CommentHeartCommand) (CommentHeartSummary, error) {
	repository.heartRemoved = &command
	return repository.heartSummary, firstCommentError(repository.mutationErr, repository.err)
}

func (repository *controlledComments) ListPracticeCommentHearts(_ context.Context, viewer, eventID, commentID string, request CommentHeartRosterPageRequest) (CommentHeartRosterPage, error) {
	repository.resolvedViewer, repository.resolvedEvent, repository.resolvedID, repository.heartRequest = viewer, eventID, commentID, request
	return repository.heartPage, firstCommentError(repository.listErr, repository.err)
}

func firstCommentError(specific, common error) error {
	if specific != nil {
		return specific
	}
	return common
}

func validComment(id, eventID, author string, version int64, createdAt, updatedAt time.Time, text string) domain.Comment {
	return domain.Comment{ID: id, EventID: eventID, AuthorID: author, Text: text, Version: version, CreatedAt: createdAt, UpdatedAt: updatedAt}
}

func commentItem(comment domain.Comment) PracticeCommentItem {
	return PracticeCommentItem{Comment: comment, Author: domain.PublicProfile{ID: comment.AuthorID, Username: "alex_user", DisplayName: "Alex", Relationship: domain.RelationshipNone}}
}

func commentService(repository *controlledComments) (*Service, *feedAuthorizer, *controlledAudits) {
	audits := &controlledAudits{}
	service := testService(&controlledProfiles{}, audits)
	authorizer := &feedAuthorizer{allowed: map[string]bool{"path": true}}
	service.Comments, service.Authorizer = repository, authorizer
	service.CommentRateLimiter = &controlledRelationshipLimiter{allowed: true}
	return service, authorizer, audits
}

func TestListPracticeCommentsAuthorizesPagesChronologicallyAndBindsCursor(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	one := validComment("comment-1", "practice:activity", "author-1", 1, now.Add(-2*time.Minute), now.Add(-2*time.Minute), "First")
	two := validComment("comment-2", "practice:activity", "author-2", 1, now.Add(-time.Minute), now.Add(-time.Minute), "Second")
	repository := &controlledComments{
		target:      PracticeCommentTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"},
		commentPage: CommentPage{Items: []PracticeCommentItem{commentItem(one), commentItem(two)}, HasMore: true},
	}
	service, authorizer, audits := commentService(repository)
	service.Clock = fixedClock{now: now}
	comments, cursor, err := service.ListPracticeComments(context.Background(), "Bearer session", "practice:activity", "", 2)
	if err != nil || len(comments) != 2 || comments[0].Comment != one || comments[1].Comment != two || comments[0].Author.ID != one.AuthorID || cursor == "" {
		t.Fatalf("comments=%+v cursor=%q err=%v", comments, cursor, err)
	}
	if repository.listViewer != "viewer" || repository.listEvent != "practice:activity" || repository.listRequest.Snapshot != now || repository.listRequest.Limit != 2 {
		t.Fatalf("list request=%+v viewer=%q event=%q", repository.listRequest, repository.listViewer, repository.listEvent)
	}
	if len(authorizer.checks) != 1 || authorizer.checks[0] != "path:path:view:viewer" || len(audits.events) != 1 || audits.events[0].Action != audit.ResourceListed {
		t.Fatalf("checks=%v audits=%+v", authorizer.checks, audits.events)
	}

	repository.commentPage = CommentPage{}
	_, _, err = service.ListPracticeComments(context.Background(), "Bearer session", "practice:other", cursor, 2)
	if !errors.Is(err, ports.ErrInvalidArgument) || repository.listEvent != "practice:activity" {
		t.Fatalf("cross-event cursor reached repository: event=%q err=%v", repository.listEvent, err)
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	_, _, err = service.ListPracticeComments(context.Background(), "Bearer session", "practice:activity", cursor, 2)
	if !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("cross-viewer cursor error=%v", err)
	}
}

func TestListCommentHistoryAuthorizesAndUsesStableVersionCursor(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	comment := validComment("comment-1", "practice:activity", "author", 2, now.Add(-time.Hour), now.Add(-time.Minute), "Current")
	repository := &controlledComments{
		resolved: ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: comment.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: comment},
		historyPage: CommentHistoryPage{Versions: []domain.CommentVersion{
			{CommentID: comment.ID, Text: "Original", Version: 1, CreatedAt: comment.CreatedAt},
			{CommentID: comment.ID, Text: comment.Text, Version: 2, CreatedAt: comment.UpdatedAt},
		}, HasMore: true},
	}
	service, _, _ := commentService(repository)
	service.Clock = fixedClock{now: now}
	versions, cursor, err := service.ListCommentHistory(context.Background(), "Bearer session", comment.EventID, comment.ID, "", 2)
	if err != nil || len(versions) != 2 || cursor == "" || repository.historyRequest.AfterVersion != 0 || repository.historyRequest.Snapshot != now {
		t.Fatalf("versions=%+v cursor=%q request=%+v err=%v", versions, cursor, repository.historyRequest, err)
	}
	repository.historyPage = CommentHistoryPage{}
	_, _, err = service.ListCommentHistory(context.Background(), "Bearer session", comment.EventID, "comment-other", cursor, 2)
	if !errors.Is(err, ports.ErrInvalidArgument) || repository.historyID != comment.ID {
		t.Fatalf("cross-comment cursor reached repository: id=%q err=%v", repository.historyID, err)
	}
}

func TestCreateAchievementCommentNormalizesAuthorizesAndConveysNotificationIntent(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 123456789, time.UTC)
	canonicalNow := now.Truncate(time.Microsecond)
	want := validComment("comment-1", "achievement:goal", "viewer", 1, canonicalNow, canonicalNow, "Caf\u00e9!\n\U0001f389")
	repository := &controlledComments{
		target:   PracticeCommentTarget{EventID: want.EventID, PathID: "path", OwnerUserID: "owner"},
		mutation: CommentMutationResult{Comment: want},
	}
	service, authorizer, _ := commentService(repository)
	service.Clock = fixedClock{now: now}
	got, err := service.CreatePracticeComment(context.Background(), "Bearer session", want.EventID, " Caf\u0065\u0301!\n\U0001f389 ", "create-comment-0001")
	if err != nil || got != want {
		t.Fatalf("comment=%+v err=%v", got, err)
	}
	command := repository.created
	if command == nil || command.ActorUserID != "viewer" || command.Target != repository.target || command.Text != want.Text || command.OccurredAt != canonicalNow || command.NotificationEligibleAt != canonicalNow.Add(5*time.Second) || !command.NotifyEventOwner {
		t.Fatalf("command=%+v", command)
	}
	wantHash := sha256.Sum256([]byte(CreatePracticeCommentOperation + "\x00" + want.EventID + "\x00" + want.Text))
	if command.Idempotency.PrincipalID != "viewer" || command.Idempotency.Operation != CreatePracticeCommentOperation || command.Idempotency.Key != "create-comment-0001" || !bytes.Equal(command.Idempotency.RequestHash, wantHash[:]) || !command.Audit.Valid() {
		t.Fatalf("idempotency=%+v audit=%+v", command.Idempotency, command.Audit)
	}
	if len(authorizer.checks) != 1 || authorizer.checks[0] != "path:path:view:viewer" {
		t.Fatalf("checks=%v", authorizer.checks)
	}

	repository.target.OwnerUserID = "viewer"
	repository.mutation.Comment = validComment("comment-2", want.EventID, "viewer", 1, canonicalNow, canonicalNow, "Self")
	_, err = service.CreatePracticeComment(context.Background(), "Bearer session", want.EventID, "Self", "create-comment-0002")
	if err != nil || repository.created.NotifyEventOwner {
		t.Fatalf("self comment command=%+v err=%v", repository.created, err)
	}
}

func TestEditPracticeCommentIsAuthorOnlyAndNeverNotifiesAgain(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	original := validComment("comment-1", "practice:activity", "viewer", 1, now.Add(-time.Hour), now.Add(-time.Hour), "Original")
	edited := validComment(original.ID, original.EventID, original.AuthorID, 2, original.CreatedAt, now, "Edited")
	repository := &controlledComments{
		resolved: ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: original.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: original},
		mutation: CommentMutationResult{Comment: edited},
	}
	service, _, audits := commentService(repository)
	service.Clock = fixedClock{now: now}
	got, err := service.EditPracticeComment(context.Background(), "Bearer session", original.EventID, original.ID, " Edited ", original.Version, "edit-comment-00001")
	if err != nil || got != edited || repository.edited == nil || repository.edited.NotifyEventOwner || !repository.edited.NotificationEligibleAt.IsZero() {
		t.Fatalf("comment=%+v command=%+v err=%v", got, repository.edited, err)
	}
	wantHash := sha256.Sum256([]byte(EditPracticeCommentOperation + "\x00" + original.EventID + "\x00" + original.ID + "\x001\x00Edited"))
	if repository.edited.ExpectedVersion != 1 || !bytes.Equal(repository.edited.Idempotency.RequestHash, wantHash[:]) {
		t.Fatalf("request hash=%x want=%x", repository.edited.Idempotency.RequestHash, wantHash)
	}
	repository.edited, repository.resolvedID = nil, ""
	_, err = service.EditPracticeComment(context.Background(), "Bearer session", original.EventID, original.ID, "Nope", 0, "edit-comment-00002")
	if !errors.Is(err, ports.ErrInvalidArgument) || repository.resolvedID != "" || repository.edited != nil {
		t.Fatalf("nonpositive expected version reached repository: resolved=%q command=%+v err=%v", repository.resolvedID, repository.edited, err)
	}

	repository.resolved.Comment.AuthorID = "someone-else"
	repository.edited = nil
	_, err = service.EditPracticeComment(context.Background(), "Bearer session", original.EventID, original.ID, "Nope", original.Version, "edit-comment-00002")
	if !errors.Is(err, ports.ErrNotFound) || repository.edited != nil || len(audits.events) != 1 || audits.events[0].TargetID != "hidden" {
		t.Fatalf("unauthorized edit command=%+v audits=%+v err=%v", repository.edited, audits.events, err)
	}
}

func TestDeletePracticeCommentAllowsAuthorOrEventOwnerOnly(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	comment := validComment("comment-1", "practice:activity", "author", 1, now.Add(-time.Hour), now.Add(-time.Hour), "Original")
	for _, actor := range []string{"author", "owner"} {
		t.Run(actor, func(t *testing.T) {
			repository := &controlledComments{
				resolved:     ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: comment.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: comment},
				deleteResult: CommentDeleteResult{CommentID: comment.ID, Deleted: true},
			}
			service, _, _ := commentService(repository)
			service.Auth = controlledAuth{principal: ports.Principal{UserID: actor, Scopes: []string{"api:user"}}}
			service.Clock = fixedClock{now: now}
			result, err := service.DeletePracticeComment(context.Background(), "Bearer session", comment.EventID, comment.ID, "delete-comment-001")
			if err != nil || !result.Deleted || repository.deleted == nil || repository.deleted.ActorUserID != actor || repository.deleted.NotifyEventOwner {
				t.Fatalf("result=%+v command=%+v err=%v", result, repository.deleted, err)
			}
			wantHash := sha256.Sum256([]byte(DeletePracticeCommentOperation + "\x00" + comment.EventID + "\x00" + comment.ID))
			if !bytes.Equal(repository.deleted.Idempotency.RequestHash, wantHash[:]) {
				t.Fatalf("request hash=%x want=%x", repository.deleted.Idempotency.RequestHash, wantHash)
			}
		})
	}

	repository := &controlledComments{resolved: ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: comment.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: comment}}
	service, _, _ := commentService(repository)
	_, err := service.DeletePracticeComment(context.Background(), "Bearer session", comment.EventID, comment.ID, "delete-comment-001")
	if !errors.Is(err, ports.ErrNotFound) || repository.deleted != nil {
		t.Fatalf("unrelated delete command=%+v err=%v", repository.deleted, err)
	}
}

func TestPracticeCommentsDenyOpaquelyAuditAndFailClosed(t *testing.T) {
	dependencyFailure := errors.New("spicedb unavailable")
	auditFailure := errors.New("audit unavailable")
	for _, test := range []struct {
		name      string
		configure func(*controlledComments, *feedAuthorizer, *controlledAudits)
		want      error
		wantAudit bool
	}{
		{name: "missing event", configure: func(repository *controlledComments, _ *feedAuthorizer, _ *controlledAudits) {
			repository.targetErr = ports.ErrNotFound
		}, want: ports.ErrNotFound, wantAudit: true},
		{name: "policy denial", configure: func(_ *controlledComments, authorizer *feedAuthorizer, _ *controlledAudits) {
			authorizer.allowed["path"] = false
		}, want: ports.ErrNotFound, wantAudit: true},
		{name: "policy outage", configure: func(_ *controlledComments, authorizer *feedAuthorizer, _ *controlledAudits) {
			authorizer.err = dependencyFailure
		}, want: dependencyFailure},
		{name: "audit persistence failure", configure: func(repository *controlledComments, _ *feedAuthorizer, audits *controlledAudits) {
			repository.targetErr, audits.err = ports.ErrNotFound, auditFailure
		}, want: auditFailure, wantAudit: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledComments{target: PracticeCommentTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"}}
			service, authorizer, audits := commentService(repository)
			test.configure(repository, authorizer, audits)
			_, _, err := service.ListPracticeComments(context.Background(), "Bearer session", "practice:activity", "", 20)
			if !errors.Is(err, test.want) || len(audits.events) != map[bool]int{true: 1, false: 0}[test.wantAudit] || repository.listEvent != "" {
				t.Fatalf("err=%v audits=%+v repository=%+v", err, audits.events, repository)
			}
			if test.wantAudit && (audits.events[0].Action != audit.ResourceAccessDenied || audits.events[0].TargetType != "practice_comment" || audits.events[0].TargetID != "hidden" || audits.events[0].Outcome != audit.Denied) {
				t.Fatalf("denial audit=%+v", audits.events[0])
			}
		})
	}
}

func TestPracticeCommentsRejectInvalidRateLimitedAndMissingDependenciesBeforeRepository(t *testing.T) {
	for _, test := range []struct {
		name      string
		configure func(*Service)
		eventID   string
		text      string
		key       string
		want      error
	}{
		{name: "invalid event", eventID: " event ", text: "hello", key: "create-comment-0001", want: ports.ErrInvalidArgument},
		{name: "invalid text", eventID: "practice:activity", text: "hello\rworld", key: "create-comment-0001", want: ports.ErrInvalidArgument},
		{name: "invalid idempotency key", eventID: "practice:activity", text: "hello", key: "short", want: ports.ErrInvalidArgument},
		{name: "rate limited", eventID: "practice:activity", text: "hello", key: "create-comment-0001", configure: func(service *Service) { service.CommentRateLimiter = &controlledRelationshipLimiter{} }, want: platformapp.ErrRateLimited},
		{name: "missing repository", eventID: "practice:activity", text: "hello", key: "create-comment-0001", configure: func(service *Service) { service.Comments = nil }, want: errInvalidDependencies},
		{name: "missing authorizer", eventID: "practice:activity", text: "hello", key: "create-comment-0001", configure: func(service *Service) { service.Authorizer = nil }, want: errInvalidDependencies},
		{name: "missing rate limiter", eventID: "practice:activity", text: "hello", key: "create-comment-0001", configure: func(service *Service) { service.CommentRateLimiter = nil }, want: errInvalidDependencies},
		{name: "missing audit", eventID: "practice:activity", text: "hello", key: "create-comment-0001", configure: func(service *Service) { service.Audits = nil }, want: errInvalidDependencies},
		{name: "missing clock", eventID: "practice:activity", text: "hello", key: "create-comment-0001", configure: func(service *Service) { service.Clock = nil }, want: errInvalidDependencies},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledComments{target: PracticeCommentTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"}}
			service, _, _ := commentService(repository)
			if test.configure != nil {
				test.configure(service)
			}
			got, err := service.CreatePracticeComment(context.Background(), "Bearer session", test.eventID, test.text, test.key)
			if !errors.Is(err, test.want) || got != (domain.Comment{}) || repository.resolvedEvent != "" || repository.created != nil {
				t.Fatalf("comment=%+v err=%v repository=%+v", got, err, repository)
			}
		})
	}
}

func TestCommentPageResultsMustRemainBoundedCanonicalAndChronological(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	first := validComment("comment-1", "practice:activity", "author", 1, now.Add(-2*time.Minute), now.Add(-2*time.Minute), "First")
	second := validComment("comment-2", "practice:activity", "author", 1, now.Add(-time.Minute), now.Add(-time.Minute), "Second")
	request := CommentPageRequest{Snapshot: now, Limit: 2}
	if !validCommentPage(CommentPage{Items: []PracticeCommentItem{commentItem(first), commentItem(second)}}, first.EventID, request) {
		t.Fatal("valid chronological comment page rejected")
	}
	for name, page := range map[string]CommentPage{
		"reverse order":         {Items: []PracticeCommentItem{commentItem(second), commentItem(first)}},
		"foreign event":         {Items: []PracticeCommentItem{commentItem(withComment(second, func(value *domain.Comment) { value.EventID = "practice:other" }))}},
		"beyond snapshot":       {Items: []PracticeCommentItem{commentItem(withComment(second, func(value *domain.Comment) { value.CreatedAt = now.Add(time.Second); value.UpdatedAt = value.CreatedAt }))}},
		"mismatched author":     {Items: []PracticeCommentItem{withCommentItem(commentItem(second), func(value *PracticeCommentItem) { value.Author.ID = "other" })}},
		"empty continuation":    {HasMore: true},
		"over repository limit": {Items: []PracticeCommentItem{commentItem(first), commentItem(second), commentItem(validComment("comment-3", first.EventID, "author", 1, now, now, "Third"))}},
	} {
		t.Run(name, func(t *testing.T) {
			if validCommentPage(page, first.EventID, request) {
				t.Fatalf("invalid page accepted: %+v", page)
			}
		})
	}

	versions := []domain.CommentVersion{
		{CommentID: first.ID, Text: "First", Version: 1, CreatedAt: first.CreatedAt},
		{CommentID: first.ID, Text: "Second", Version: 2, CreatedAt: second.CreatedAt},
	}
	historyRequest := CommentHistoryPageRequest{Snapshot: now, Limit: 2}
	if !validCommentHistoryPage(CommentHistoryPage{Versions: versions}, first.ID, historyRequest) {
		t.Fatal("valid chronological history page rejected")
	}
	versions[1].Version = 1
	if validCommentHistoryPage(CommentHistoryPage{Versions: versions}, first.ID, historyRequest) {
		t.Fatal("duplicate history version accepted")
	}
}

func withComment(value domain.Comment, mutate func(*domain.Comment)) domain.Comment {
	mutate(&value)
	return value
}

func withCommentItem(value PracticeCommentItem, mutate func(*PracticeCommentItem)) PracticeCommentItem {
	mutate(&value)
	return value
}

func TestEditPracticeCommentReplayIsValidatedAndVersionIsHashBound(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	prior := validComment("comment", "practice:activity", "viewer", 3, now.Add(-time.Hour), now.Add(-time.Minute), "Prior")
	replay := validComment(prior.ID, prior.EventID, prior.AuthorID, 4, prior.CreatedAt, now.Add(-30*time.Second), "Requested")
	repository := &controlledComments{replay: PracticeCommentReplay{ActorUserID: "viewer", EventID: prior.EventID, Comment: replay}, replayFound: true}
	service, authorizer, audits := commentService(repository)
	service.Clock = fixedClock{now: now}
	got, err := service.EditPracticeComment(context.Background(), "Bearer session", prior.EventID, prior.ID, replay.Text, prior.Version, "edit-comment-00003")
	if err != nil || got != replay || repository.edited != nil || repository.resolvedID != "" || len(authorizer.checks) != 0 || len(audits.events) != 0 {
		t.Fatalf("comment=%+v command=%+v resolved=%q checks=%v audits=%+v err=%v", got, repository.edited, repository.resolvedID, authorizer.checks, audits.events, err)
	}
	want := sha256.Sum256([]byte(EditPracticeCommentOperation + "\x00" + prior.EventID + "\x00" + prior.ID + "\x003\x00" + replay.Text))
	if !bytes.Equal(repository.replayRequest.RequestHash, want[:]) {
		t.Fatalf("request hash=%x want=%x", repository.replayRequest.RequestHash, want)
	}
	otherVersion := commentIdempotency("viewer", EditPracticeCommentOperation, "edit-comment-00003", prior.EventID, prior.ID, prior.Version+1, replay.Text)
	if bytes.Equal(repository.replayRequest.RequestHash, otherVersion.RequestHash) {
		t.Fatal("expected version was omitted from edit request hash")
	}

	repository.replay.Comment.AuthorID = "other"
	if leaked, err := service.EditPracticeComment(context.Background(), "Bearer session", prior.EventID, prior.ID, replay.Text, prior.Version, "edit-comment-00003"); !errors.Is(err, errInvalidDependencies) || leaked != (domain.Comment{}) {
		t.Fatalf("malformed replay leaked comment=%+v err=%v", leaked, err)
	}
}

func TestEditPracticeCommentForwardsCallerVersionForTransactionalCAS(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	current := validComment("comment", "practice:activity", "viewer", 4, now.Add(-time.Hour), now.Add(-time.Minute), "Current")
	repository := &controlledComments{
		resolved:    ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: current.EventID, PathID: "path", OwnerUserID: "owner"}, Comment: current},
		mutationErr: ports.ErrConflict,
	}
	service, _, _ := commentService(repository)
	service.Clock = fixedClock{now: now}
	_, err := service.EditPracticeComment(context.Background(), "Bearer session", current.EventID, current.ID, "Replacement", 3, "edit-comment-00004")
	if !errors.Is(err, ports.ErrConflict) || repository.edited == nil || repository.edited.ExpectedVersion != 3 {
		t.Fatalf("command=%+v err=%v", repository.edited, err)
	}
}

func TestCreateAndDeletePracticeCommentReplaysConvergeWithoutLiveResolution(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	created := validComment("comment", "practice:activity", "viewer", 1, now.Add(-time.Minute), now.Add(-time.Minute), "Created")

	createRepository := &controlledComments{replay: PracticeCommentReplay{ActorUserID: "viewer", EventID: created.EventID, Comment: created}, replayFound: true}
	createService, createAuthorizer, createAudits := commentService(createRepository)
	createService.Clock = fixedClock{now: now}
	got, err := createService.CreatePracticeComment(context.Background(), "Bearer session", created.EventID, created.Text, "create-comment-0005")
	if err != nil || got != created || createRepository.resolvedEvent != "" || createRepository.created != nil || len(createAuthorizer.checks) != 0 || len(createAudits.events) != 0 {
		t.Fatalf("comment=%+v repository=%+v checks=%v audits=%+v err=%v", got, createRepository, createAuthorizer.checks, createAudits.events, err)
	}

	deletedSnapshot := created
	deletedSnapshot.AuthorID = "author"
	deleteRepository := &controlledComments{replay: PracticeCommentReplay{ActorUserID: "viewer", EventID: created.EventID, Comment: deletedSnapshot, Deleted: true}, replayFound: true}
	deleteService, deleteAuthorizer, deleteAudits := commentService(deleteRepository)
	deleteService.Clock = fixedClock{now: now}
	result, err := deleteService.DeletePracticeComment(context.Background(), "Bearer session", created.EventID, created.ID, "delete-comment-005")
	if err != nil || result != (CommentDeleteResult{CommentID: created.ID, Deleted: true, Replayed: true}) || deleteRepository.resolvedID != "" || deleteRepository.deleted != nil || len(deleteAuthorizer.checks) != 0 || len(deleteAudits.events) != 0 {
		t.Fatalf("result=%+v repository=%+v checks=%v audits=%+v err=%v", result, deleteRepository, deleteAuthorizer.checks, deleteAudits.events, err)
	}
}

func TestPracticeCommentRejectsMalformedRepositoryAndReplayResults(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		result   CommentMutationResult
		deleting bool
		deleted  CommentDeleteResult
	}{
		{name: "created wrong author", result: CommentMutationResult{Comment: validComment("comment", "practice:activity", "other", 1, now, now, "hello")}},
		{name: "created wrong version", result: CommentMutationResult{Comment: validComment("comment", "practice:activity", "viewer", 2, now, now, "hello")}},
		{name: "replayed malformed", result: CommentMutationResult{Comment: validComment("comment", "practice:other", "viewer", 1, now, now, "hello"), Replayed: true}},
		{name: "delete wrong identity", deleting: true, deleted: CommentDeleteResult{CommentID: "other", Deleted: true, Replayed: true}},
		{name: "delete not confirmed", deleting: true, deleted: CommentDeleteResult{CommentID: "comment", Replayed: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledComments{
				target:       PracticeCommentTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"},
				mutation:     test.result,
				deleteResult: test.deleted,
				resolved:     ResolvedPracticeComment{Target: PracticeCommentTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "viewer"}, Comment: validComment("comment", "practice:activity", "viewer", 1, now, now, "hello")},
			}
			service, _, _ := commentService(repository)
			service.Clock = fixedClock{now: now}
			if test.deleting {
				_, err := service.DeletePracticeComment(context.Background(), "Bearer session", "practice:activity", "comment", "delete-comment-001")
				if !errors.Is(err, errInvalidDependencies) {
					t.Fatalf("delete error=%v result=%+v", err, test.deleted)
				}
				return
			}
			got, err := service.CreatePracticeComment(context.Background(), "Bearer session", "practice:activity", "hello", "create-comment-0001")
			if !errors.Is(err, errInvalidDependencies) || got != (domain.Comment{}) {
				t.Fatalf("comment=%+v error=%v result=%+v", got, err, test.result)
			}
		})
	}
}
