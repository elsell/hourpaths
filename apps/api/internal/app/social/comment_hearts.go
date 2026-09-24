package social

import (
	"context"
	"time"

	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	SetPracticeCommentHeartOperation    = "social.practice_comment_heart.set"
	RemovePracticeCommentHeartOperation = "social.practice_comment_heart.remove"
	commentHeartRosterCursorDomain      = "social-practice-comment-hearts:"
)

type CommentHeartSummary struct {
	CommentID       string
	HeartCount      int64
	HeartedByViewer bool
}

func (summary CommentHeartSummary) valid(commentID string) bool {
	return summary.CommentID == commentID && summary.HeartCount >= 0 && (!summary.HeartedByViewer || summary.HeartCount > 0)
}

type CommentHeartCommand struct {
	ActorUserID            string
	Target                 PracticeCommentTarget
	Comment                domain.Comment
	OccurredAt             time.Time
	NotificationEligibleAt time.Time
	NotifyCommentAuthor    bool
	Idempotency            ports.Idempotency
	Audit                  audit.Event
}

type PracticeCommentHeartReplay struct {
	ActorUserID string
	EventID     string
	CommentID   string
	Hearted     bool
	Summary     CommentHeartSummary
}

type CommentHeartRosterPageRequest struct {
	AfterUserID  string
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
}

type CommentHeartRosterItem struct {
	Profile   domain.PublicProfile
	HeartedAt time.Time
}

type CommentHeartRosterPage struct {
	Items   []CommentHeartRosterItem
	HasMore bool
}

func (service *Service) SetPracticeCommentHeart(ctx context.Context, authorization, eventID, commentID, idempotencyKey string) (CommentHeartSummary, error) {
	return service.mutatePracticeCommentHeart(ctx, authorization, eventID, commentID, idempotencyKey, true)
}

func (service *Service) RemovePracticeCommentHeart(ctx context.Context, authorization, eventID, commentID, idempotencyKey string) (CommentHeartSummary, error) {
	return service.mutatePracticeCommentHeart(ctx, authorization, eventID, commentID, idempotencyKey, false)
}

func (service *Service) mutatePracticeCommentHeart(ctx context.Context, authorization, eventID, commentID, idempotencyKey string, hearted bool) (CommentHeartSummary, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return CommentHeartSummary{}, err
	}
	if !validOpaqueID(eventID) || !validOpaqueID(commentID) || !validRelationshipIdempotencyKey(idempotencyKey) {
		return CommentHeartSummary{}, ports.ErrInvalidArgument
	}
	now, err := service.commentReady(principal.UserID, "practice_comment:"+commentID, false)
	if err != nil {
		return CommentHeartSummary{}, err
	}
	operation := RemovePracticeCommentHeartOperation
	if hearted {
		operation = SetPracticeCommentHeartOperation
	}
	idempotency := commentIdempotency(principal.UserID, operation, idempotencyKey, eventID, commentID, 0, "")
	resolved, err := service.resolveComment(ctx, principal.UserID, eventID, commentID, now)
	if err != nil {
		return CommentHeartSummary{}, err
	}
	if err := service.authorizeCommentTarget(ctx, principal.UserID, resolved.Target, now); err != nil {
		return CommentHeartSummary{}, err
	}
	action := audit.ResourceDeleted
	if hearted {
		action = audit.ResourceCreated
	}
	command := CommentHeartCommand{
		ActorUserID: principal.UserID, Target: resolved.Target, Comment: resolved.Comment, OccurredAt: now,
		Idempotency: idempotency,
		Audit:       shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, action, "practice_comment_heart", commentID, audit.Succeeded),
	}
	command.Audit.OccurredAt = now
	if hearted {
		command.NotificationEligibleAt = now.Add(commentNotificationGrace)
		command.NotifyCommentAuthor = principal.UserID != resolved.Comment.AuthorID
	}
	var summary CommentHeartSummary
	if hearted {
		summary, err = service.Comments.SetPracticeCommentHeart(ctx, command)
	} else {
		summary, err = service.Comments.RemovePracticeCommentHeart(ctx, command)
	}
	if err != nil {
		return CommentHeartSummary{}, service.commentError(ctx, principal.UserID, err, now)
	}
	if !summary.valid(commentID) || summary.HeartedByViewer != hearted {
		return CommentHeartSummary{}, errInvalidDependencies
	}
	return summary, nil
}

func (service *Service) ListPracticeCommentHearts(ctx context.Context, authorization, eventID, commentID, cursor string, limit int) ([]domain.PublicProfile, string, error) {
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
	request := CommentHeartRosterPageRequest{Snapshot: now, Limit: limit}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != commentCursorDomain(commentHeartRosterCursorDomain, eventID, commentID) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterUserID, request.AfterCreated, request.Snapshot = payload.AfterID, payload.AfterCreated, payload.Snapshot
	}
	resolved, err := service.resolveComment(ctx, principal.UserID, eventID, commentID, now)
	if err != nil {
		return nil, "", err
	}
	if err := service.authorizeCommentTarget(ctx, principal.UserID, resolved.Target, now); err != nil {
		return nil, "", err
	}
	page, err := service.Comments.ListPracticeCommentHearts(ctx, principal.UserID, eventID, commentID, request)
	if err != nil {
		return nil, "", service.commentError(ctx, principal.UserID, err, now)
	}
	if !validCommentHeartRosterPage(page, request) {
		return nil, "", errInvalidDependencies
	}
	profiles := make([]domain.PublicProfile, 0, len(page.Items))
	for _, item := range page.Items {
		profiles = append(profiles, item.Profile)
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "practice_comment_heart", commentID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		next, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: commentCursorDomain(commentHeartRosterCursorDomain, eventID, commentID), AfterID: last.Profile.ID, AfterCreated: last.HeartedAt, Snapshot: request.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	return profiles, next, nil
}

func validCommentHeartRosterPage(page CommentHeartRosterPage, request CommentHeartRosterPageRequest) bool {
	if len(page.Items) > request.Limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	previousTime, previousID := request.AfterCreated, request.AfterUserID
	for _, item := range page.Items {
		if !item.Profile.Valid() || item.HeartedAt.IsZero() || item.HeartedAt.Location() != time.UTC || item.HeartedAt.After(request.Snapshot) ||
			(!previousTime.IsZero() && (item.HeartedAt.Before(previousTime) || (item.HeartedAt.Equal(previousTime) && item.Profile.ID <= previousID))) {
			return false
		}
		previousTime, previousID = item.HeartedAt, item.Profile.ID
	}
	return true
}
