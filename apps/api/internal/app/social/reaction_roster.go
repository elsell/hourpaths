package social

import (
	"context"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const reactionRosterCursorDomain = "social-practice-reactions:"

type ReactionRosterPageRequest struct {
	AfterUserID  string
	AfterCreated time.Time
	Snapshot     time.Time
	Limit        int
}

type ReactionRosterItem struct {
	Profile   domain.PublicProfile
	ReactedAt time.Time
}

type ReactionRosterPage struct {
	Items   []ReactionRosterItem
	HasMore bool
}

func (service *Service) ListPracticeReactions(ctx context.Context, authorization, eventID string, reaction domain.Reaction, cursor string, limit int) ([]domain.PublicProfile, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if !reaction.Valid() {
		canonical, valid := domain.CanonicalEmojiReaction(string(reaction))
		if !valid {
			return nil, "", ports.ErrInvalidArgument
		}
		reaction = canonical
	}
	if !validOpaqueID(eventID) || !reaction.ValidStored() || limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	now, err := service.reactionRosterReady(principal.UserID, eventID)
	if err != nil {
		return nil, "", err
	}
	request := ReactionRosterPageRequest{Snapshot: now, Limit: limit}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != commentCursorDomain(reactionRosterCursorDomain, eventID, string(reaction)) {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterUserID, request.AfterCreated, request.Snapshot = payload.AfterID, payload.AfterCreated, payload.Snapshot
	}
	target, err := service.Reactions.ResolvePracticeReactionTarget(ctx, principal.UserID, eventID)
	if err != nil {
		return nil, "", service.reactionError(ctx, principal.UserID, err, now)
	}
	if !validReactionTarget(target, eventID) {
		return nil, "", errInvalidDependencies
	}
	allowed, err := service.Authorizer.Check(ctx, "path", target.PathID, "view", principal.UserID)
	if err != nil {
		return nil, "", err
	}
	if !allowed {
		return nil, "", service.reactionError(ctx, principal.UserID, ports.ErrNotFound, now)
	}
	page, err := service.Reactions.ListPracticeReactions(ctx, principal.UserID, eventID, reaction, request)
	if err != nil {
		return nil, "", service.reactionError(ctx, principal.UserID, err, now)
	}
	if !validReactionRosterPage(page, request) {
		return nil, "", errInvalidDependencies
	}
	profiles := make([]domain.PublicProfile, 0, len(page.Items))
	for _, item := range page.Items {
		profiles = append(profiles, item.Profile)
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "practice_reaction", eventID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		next, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: commentCursorDomain(reactionRosterCursorDomain, eventID, string(reaction)), AfterID: last.Profile.ID, AfterCreated: last.ReactedAt, Snapshot: request.Snapshot})
		if err != nil {
			return nil, "", err
		}
	}
	return profiles, next, nil
}

func validReactionRosterPage(page ReactionRosterPage, request ReactionRosterPageRequest) bool {
	if len(page.Items) > request.Limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	previousTime, previousID := request.AfterCreated, request.AfterUserID
	for _, item := range page.Items {
		if !item.Profile.Valid() || item.ReactedAt.IsZero() || item.ReactedAt.Location() != time.UTC || item.ReactedAt.After(request.Snapshot) ||
			(!previousTime.IsZero() && (item.ReactedAt.Before(previousTime) || (item.ReactedAt.Equal(previousTime) && item.Profile.ID <= previousID))) {
			return false
		}
		previousTime, previousID = item.ReactedAt, item.Profile.ID
	}
	return true
}

func (service *Service) reactionRosterReady(actor, eventID string) (time.Time, error) {
	if service.Reactions == nil || service.Authorizer == nil || service.ReactionRateLimiter == nil || service.Audits == nil || service.Clock == nil || len(service.CursorSigningKey) < 32 {
		return time.Time{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return time.Time{}, errInvalidDependencies
	}
	if !service.ReactionRateLimiter.Allow(actor, "practice_event:"+eventID, now) {
		return time.Time{}, platformapp.ErrRateLimited
	}
	return now, nil
}
