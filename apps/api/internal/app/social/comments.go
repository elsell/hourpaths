package social

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	CreatePracticeCommentOperation = "social.practice_comment.create"
	EditPracticeCommentOperation   = "social.practice_comment.edit"
	DeletePracticeCommentOperation = "social.practice_comment.delete"
	commentNotificationGrace       = 5 * time.Second
	commentListCursorDomain        = "social-practice-comments:"
	commentHistoryCursorDomain     = "social-practice-comment-history:"
)

type PracticeCommentTarget struct {
	EventID     string
	PathID      string
	OwnerUserID string
}

type ResolvedPracticeComment struct {
	Target  PracticeCommentTarget
	Comment domain.Comment
}

type CommentPageRequest struct {
	AfterID      string
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
}

type CommentPage struct {
	Items   []PracticeCommentItem
	HasMore bool
}

// PracticeCommentItem combines immutable comment state with the author's
// viewer-safe public identity; private profile fields never enter the comment
// domain entity.
type PracticeCommentItem struct {
	Comment         domain.Comment
	Author          domain.PublicProfile
	HeartCount      int64
	HeartedByViewer bool
}

func (item PracticeCommentItem) valid() bool {
	return item.Comment.Valid() && item.Author.Valid() && item.Author.ID == item.Comment.AuthorID && item.HeartCount >= 0 && (!item.HeartedByViewer || item.HeartCount > 0)
}

type CommentHistoryPageRequest struct {
	AfterVersion int64
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
}

type CommentHistoryPage struct {
	Versions []domain.CommentVersion
	HasMore  bool
}

type CommentCommand struct {
	ActorUserID string
	Target      PracticeCommentTarget
	CommentID   string
	Text        string
	// ExpectedVersion is positive for edits so persistence can compare-and-swap.
	// Deletes intentionally operate on the currently authorized comment identity.
	ExpectedVersion        int64
	OccurredAt             time.Time
	NotificationEligibleAt time.Time
	NotifyEventOwner       bool
	Idempotency            ports.Idempotency
	Audit                  audit.Event
}

type CommentMutationResult struct {
	Comment  domain.Comment
	Replayed bool
}

type CommentDeleteResult struct {
	CommentID string
	Deleted   bool
	Replayed  bool
}

// PracticeCommentReplay is the immutable mutation result retained for an
// idempotency key. Deleted replays retain the prior Comment as a tombstone
// snapshot so the application can validate identity without resolving live data.
type PracticeCommentReplay struct {
	ActorUserID string
	EventID     string
	Comment     domain.Comment
	Deleted     bool
}

type CommentRepository interface {
	FindPracticeCommentReplay(context.Context, ports.Idempotency) (PracticeCommentReplay, bool, error)
	ResolvePracticeCommentTarget(context.Context, string, string) (PracticeCommentTarget, error)
	ResolvePracticeComment(context.Context, string, string, string) (ResolvedPracticeComment, error)
	ListPracticeComments(context.Context, string, string, CommentPageRequest) (CommentPage, error)
	ListCommentHistory(context.Context, string, string, string, CommentHistoryPageRequest) (CommentHistoryPage, error)
	CreatePracticeComment(context.Context, CommentCommand) (CommentMutationResult, error)
	EditPracticeComment(context.Context, CommentCommand) (CommentMutationResult, error)
	DeletePracticeComment(context.Context, CommentCommand) (CommentDeleteResult, error)
	SetPracticeCommentHeart(context.Context, CommentHeartCommand) (CommentHeartSummary, error)
	RemovePracticeCommentHeart(context.Context, CommentHeartCommand) (CommentHeartSummary, error)
	ListPracticeCommentHearts(context.Context, string, string, string, CommentHeartRosterPageRequest) (CommentHeartRosterPage, error)
}

func (service *Service) ListPracticeComments(ctx context.Context, authorization, eventID, cursor string, limit int) ([]PracticeCommentItem, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if !validOpaqueID(eventID) || limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	now, err := service.commentReady(principal.UserID, "practice_event:"+eventID, true)
	if err != nil {
		return nil, "", err
	}
	request := CommentPageRequest{Snapshot: now, Limit: limit}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != commentCursorDomain(commentListCursorDomain, eventID) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterID, request.AfterCreated, request.Snapshot = payload.AfterID, payload.AfterCreated, payload.Snapshot
	}
	target, err := service.resolveCommentTarget(ctx, principal.UserID, eventID, now)
	if err != nil {
		return nil, "", err
	}
	if err := service.authorizeCommentTarget(ctx, principal.UserID, target, now); err != nil {
		return nil, "", err
	}
	page, err := service.Comments.ListPracticeComments(ctx, principal.UserID, eventID, request)
	if err != nil {
		return nil, "", service.commentError(ctx, principal.UserID, err, now)
	}
	if !validCommentPage(page, eventID, request) {
		return nil, "", errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "practice_comment", eventID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1].Comment
		next, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{
			Version: 1, Owner: principal.UserID, Domain: commentCursorDomain(commentListCursorDomain, eventID),
			AfterID: last.ID, AfterCreated: last.CreatedAt, Snapshot: request.Snapshot,
		})
		if err != nil {
			return nil, "", err
		}
	}
	return page.Items, next, nil
}

func (service *Service) ListCommentHistory(ctx context.Context, authorization, eventID, commentID, cursor string, limit int) ([]domain.CommentVersion, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if !validOpaqueID(eventID) || !validOpaqueID(commentID) || limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	now, err := service.commentReady(principal.UserID, "practice_comment:"+commentID, true)
	if err != nil {
		return nil, "", err
	}
	request := CommentHistoryPageRequest{Snapshot: now, Limit: limit}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		afterVersion, parseErr := strconv.ParseInt(payload.AfterID, 10, 64)
		if decodeErr != nil || parseErr != nil || afterVersion < 1 || payload.Owner != principal.UserID || payload.Domain != commentCursorDomain(commentHistoryCursorDomain, eventID, commentID) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterVersion, request.AfterCreated, request.Snapshot = afterVersion, payload.AfterCreated, payload.Snapshot
	}
	resolved, err := service.resolveComment(ctx, principal.UserID, eventID, commentID, now)
	if err != nil {
		return nil, "", err
	}
	if err := service.authorizeCommentTarget(ctx, principal.UserID, resolved.Target, now); err != nil {
		return nil, "", err
	}
	page, err := service.Comments.ListCommentHistory(ctx, principal.UserID, eventID, commentID, request)
	if err != nil {
		return nil, "", service.commentError(ctx, principal.UserID, err, now)
	}
	if !validCommentHistoryPage(page, commentID, request) {
		return nil, "", errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "practice_comment_history", commentID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if page.HasMore {
		last := page.Versions[len(page.Versions)-1]
		next, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{
			Version: 1, Owner: principal.UserID, Domain: commentCursorDomain(commentHistoryCursorDomain, eventID, commentID),
			AfterID: strconv.FormatInt(last.Version, 10), AfterCreated: last.CreatedAt, Snapshot: request.Snapshot,
		})
		if err != nil {
			return nil, "", err
		}
	}
	return page.Versions, next, nil
}

func (service *Service) CreatePracticeComment(ctx context.Context, authorization, eventID, text, idempotencyKey string) (domain.Comment, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return domain.Comment{}, err
	}
	text, err = domain.NormalizeCommentText(text)
	if !validOpaqueID(eventID) || err != nil || !validRelationshipIdempotencyKey(idempotencyKey) {
		return domain.Comment{}, ports.ErrInvalidArgument
	}
	now, err := service.commentReady(principal.UserID, "practice_event:"+eventID, false)
	if err != nil {
		return domain.Comment{}, err
	}
	idempotency := commentIdempotency(principal.UserID, CreatePracticeCommentOperation, idempotencyKey, eventID, "", 0, text)
	if replay, found, replayErr := service.Comments.FindPracticeCommentReplay(ctx, idempotency); replayErr != nil {
		return domain.Comment{}, replayErr
	} else if found {
		if !validPracticeCommentReplay(replay, principal.UserID, eventID, "", text, 0, false, now) {
			return domain.Comment{}, errInvalidDependencies
		}
		return replay.Comment, nil
	}
	target, err := service.resolveCommentTarget(ctx, principal.UserID, eventID, now)
	if err != nil {
		return domain.Comment{}, err
	}
	if err := service.authorizeCommentTarget(ctx, principal.UserID, target, now); err != nil {
		return domain.Comment{}, err
	}
	command := CommentCommand{
		ActorUserID: principal.UserID, Target: target, Text: text, OccurredAt: now,
		NotificationEligibleAt: now.Add(commentNotificationGrace), NotifyEventOwner: principal.UserID != target.OwnerUserID,
		Idempotency: idempotency,
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceCreated, "practice_comment", eventID, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	result, err := service.Comments.CreatePracticeComment(ctx, command)
	if err != nil {
		return domain.Comment{}, service.commentError(ctx, principal.UserID, err, now)
	}
	if !validCreatedCommentResult(result, principal.UserID, eventID, text, now) {
		return domain.Comment{}, errInvalidDependencies
	}
	return result.Comment, nil
}

func (service *Service) EditPracticeComment(ctx context.Context, authorization, eventID, commentID, text string, expectedVersion int64, idempotencyKey string) (domain.Comment, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return domain.Comment{}, err
	}
	text, err = domain.NormalizeCommentText(text)
	if !validOpaqueID(eventID) || !validOpaqueID(commentID) || err != nil || expectedVersion < 1 || !validRelationshipIdempotencyKey(idempotencyKey) {
		return domain.Comment{}, ports.ErrInvalidArgument
	}
	now, err := service.commentReady(principal.UserID, "practice_comment:"+commentID, false)
	if err != nil {
		return domain.Comment{}, err
	}
	idempotency := commentIdempotency(principal.UserID, EditPracticeCommentOperation, idempotencyKey, eventID, commentID, expectedVersion, text)
	if replay, found, replayErr := service.Comments.FindPracticeCommentReplay(ctx, idempotency); replayErr != nil {
		return domain.Comment{}, replayErr
	} else if found {
		if !validPracticeCommentReplay(replay, principal.UserID, eventID, commentID, text, expectedVersion, false, now) {
			return domain.Comment{}, errInvalidDependencies
		}
		return replay.Comment, nil
	}
	resolved, err := service.resolveComment(ctx, principal.UserID, eventID, commentID, now)
	if err != nil {
		return domain.Comment{}, err
	}
	if err := service.authorizeCommentTarget(ctx, principal.UserID, resolved.Target, now); err != nil {
		return domain.Comment{}, err
	}
	if resolved.Comment.AuthorID != principal.UserID {
		return domain.Comment{}, service.commentError(ctx, principal.UserID, ports.ErrNotFound, now)
	}
	command := CommentCommand{
		ActorUserID: principal.UserID, Target: resolved.Target, CommentID: commentID, Text: text,
		ExpectedVersion: expectedVersion, OccurredAt: now,
		Idempotency: idempotency,
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "practice_comment", commentID, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	result, err := service.Comments.EditPracticeComment(ctx, command)
	if err != nil {
		return domain.Comment{}, service.commentError(ctx, principal.UserID, err, now)
	}
	if !validEditedCommentResult(result, resolved.Comment, text, now) {
		return domain.Comment{}, errInvalidDependencies
	}
	return result.Comment, nil
}

func (service *Service) DeletePracticeComment(ctx context.Context, authorization, eventID, commentID, idempotencyKey string) (CommentDeleteResult, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return CommentDeleteResult{}, err
	}
	if !validOpaqueID(eventID) || !validOpaqueID(commentID) || !validRelationshipIdempotencyKey(idempotencyKey) {
		return CommentDeleteResult{}, ports.ErrInvalidArgument
	}
	now, err := service.commentReady(principal.UserID, "practice_comment:"+commentID, false)
	if err != nil {
		return CommentDeleteResult{}, err
	}
	idempotency := commentIdempotency(principal.UserID, DeletePracticeCommentOperation, idempotencyKey, eventID, commentID, 0, "")
	if replay, found, replayErr := service.Comments.FindPracticeCommentReplay(ctx, idempotency); replayErr != nil {
		return CommentDeleteResult{}, replayErr
	} else if found {
		if !validPracticeCommentReplay(replay, principal.UserID, eventID, commentID, "", 0, true, now) {
			return CommentDeleteResult{}, errInvalidDependencies
		}
		return CommentDeleteResult{CommentID: commentID, Deleted: true, Replayed: true}, nil
	}
	resolved, err := service.resolveComment(ctx, principal.UserID, eventID, commentID, now)
	if err != nil {
		return CommentDeleteResult{}, err
	}
	if err := service.authorizeCommentTarget(ctx, principal.UserID, resolved.Target, now); err != nil {
		return CommentDeleteResult{}, err
	}
	if resolved.Comment.AuthorID != principal.UserID && resolved.Target.OwnerUserID != principal.UserID {
		return CommentDeleteResult{}, service.commentError(ctx, principal.UserID, ports.ErrNotFound, now)
	}
	command := CommentCommand{
		ActorUserID: principal.UserID, Target: resolved.Target, CommentID: commentID, OccurredAt: now,
		Idempotency: idempotency,
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceDeleted, "practice_comment", commentID, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	result, err := service.Comments.DeletePracticeComment(ctx, command)
	if err != nil {
		return CommentDeleteResult{}, service.commentError(ctx, principal.UserID, err, now)
	}
	if result.CommentID != commentID || !result.Deleted {
		return CommentDeleteResult{}, errInvalidDependencies
	}
	return result, nil
}

func (service *Service) commentReady(actor, target string, cursorRequired bool) (time.Time, error) {
	if service.Comments == nil || service.Authorizer == nil || service.CommentRateLimiter == nil || service.Audits == nil || service.Clock == nil ||
		(cursorRequired && len(service.CursorSigningKey) < 32) {
		return time.Time{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return time.Time{}, errInvalidDependencies
	}
	if !service.CommentRateLimiter.Allow(actor, target, now) {
		return time.Time{}, platformapp.ErrRateLimited
	}
	return now, nil
}

func (service *Service) resolveCommentTarget(ctx context.Context, actor, eventID string, now time.Time) (PracticeCommentTarget, error) {
	target, err := service.Comments.ResolvePracticeCommentTarget(ctx, actor, eventID)
	if err != nil {
		return PracticeCommentTarget{}, service.commentError(ctx, actor, err, now)
	}
	if !validPracticeCommentTarget(target, eventID) {
		return PracticeCommentTarget{}, errInvalidDependencies
	}
	return target, nil
}

func (service *Service) resolveComment(ctx context.Context, actor, eventID, commentID string, now time.Time) (ResolvedPracticeComment, error) {
	resolved, err := service.Comments.ResolvePracticeComment(ctx, actor, eventID, commentID)
	if err != nil {
		return ResolvedPracticeComment{}, service.commentError(ctx, actor, err, now)
	}
	if !validPracticeCommentTarget(resolved.Target, eventID) || !resolved.Comment.Valid() || resolved.Comment.ID != commentID || resolved.Comment.EventID != eventID {
		return ResolvedPracticeComment{}, errInvalidDependencies
	}
	return resolved, nil
}

func (service *Service) authorizeCommentTarget(ctx context.Context, actor string, target PracticeCommentTarget, now time.Time) error {
	allowed, err := service.Authorizer.Check(ctx, "path", target.PathID, "view", actor)
	if err != nil {
		return err
	}
	if !allowed {
		return service.commentError(ctx, actor, ports.ErrNotFound, now)
	}
	return nil
}

func (service *Service) commentError(ctx context.Context, actor string, repositoryError error, now time.Time) error {
	if !errors.Is(repositoryError, ports.ErrNotFound) {
		return repositoryError
	}
	denial := shared.NewAuditEvent(ctx, service.Clock, actor, actor, audit.ResourceAccessDenied, "practice_comment", "hidden", audit.Denied)
	denial.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, denial); err != nil {
		return err
	}
	return ports.ErrNotFound
}

func validPracticeCommentTarget(target PracticeCommentTarget, eventID string) bool {
	return target.EventID == eventID && validOpaqueID(target.PathID) && validOpaqueID(target.OwnerUserID)
}

func validCommentPage(page CommentPage, eventID string, request CommentPageRequest) bool {
	if len(page.Items) > request.Limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	previousTime, previousID := request.AfterCreated, request.AfterID
	for _, item := range page.Items {
		comment := item.Comment
		if !item.valid() || comment.EventID != eventID || comment.CreatedAt.After(request.Snapshot) ||
			(!previousTime.IsZero() && (comment.CreatedAt.Before(previousTime) || (comment.CreatedAt.Equal(previousTime) && comment.ID <= previousID))) {
			return false
		}
		previousTime, previousID = comment.CreatedAt, comment.ID
	}
	return true
}

func validCommentHistoryPage(page CommentHistoryPage, commentID string, request CommentHistoryPageRequest) bool {
	if len(page.Versions) > request.Limit || (page.HasMore && len(page.Versions) == 0) {
		return false
	}
	previousVersion, previousTime := request.AfterVersion, request.AfterCreated
	for _, version := range page.Versions {
		if !version.Valid() || version.CommentID != commentID || version.CreatedAt.After(request.Snapshot) ||
			version.Version <= previousVersion || (!previousTime.IsZero() && version.CreatedAt.Before(previousTime)) {
			return false
		}
		previousVersion, previousTime = version.Version, version.CreatedAt
	}
	return true
}

func validCreatedCommentResult(result CommentMutationResult, actor, eventID, text string, now time.Time) bool {
	comment := result.Comment
	if !comment.Valid() || comment.EventID != eventID || comment.AuthorID != actor || comment.Text != text || comment.Version != 1 || comment.CreatedAt.After(now) || comment.UpdatedAt.After(now) {
		return false
	}
	return result.Replayed || (comment.CreatedAt == now && comment.UpdatedAt == now)
}

func validEditedCommentResult(result CommentMutationResult, prior domain.Comment, text string, now time.Time) bool {
	comment := result.Comment
	if !comment.Valid() || comment.ID != prior.ID || comment.EventID != prior.EventID || comment.AuthorID != prior.AuthorID || comment.Text != text || comment.Version != prior.Version+1 ||
		comment.CreatedAt != prior.CreatedAt || comment.CreatedAt.After(now) || comment.UpdatedAt.After(now) {
		return false
	}
	return result.Replayed || comment.UpdatedAt == now
}

func validPracticeCommentReplay(replay PracticeCommentReplay, actor, eventID, commentID, text string, expectedVersion int64, deleted bool, now time.Time) bool {
	comment := replay.Comment
	if replay.ActorUserID != actor || replay.EventID != eventID || replay.Deleted != deleted || !comment.Valid() ||
		comment.EventID != eventID || comment.CreatedAt.After(now) || comment.UpdatedAt.After(now) {
		return false
	}
	if commentID != "" && comment.ID != commentID {
		return false
	}
	if deleted {
		return true
	}
	if comment.AuthorID != actor || comment.Text != text {
		return false
	}
	if expectedVersion == 0 {
		return comment.Version == 1
	}
	return comment.Version == expectedVersion+1
}

func commentIdempotency(actor, operation, key, eventID, commentID string, expectedVersion int64, text string) ports.Idempotency {
	payload := operation + "\x00" + eventID
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
	return ports.Idempotency{PrincipalID: actor, Operation: operation, Key: key, RequestHash: digest[:]}
}

func commentCursorDomain(prefix string, values ...string) string {
	digest := sha256.New()
	for _, value := range values {
		_, _ = digest.Write([]byte(strconv.Itoa(len(value))))
		_, _ = digest.Write([]byte{0})
		_, _ = digest.Write([]byte(value))
	}
	return prefix + hex.EncodeToString(digest.Sum(nil))
}
