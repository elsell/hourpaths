package social

import (
	"context"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const activeFollowingCursorDomain = "social-active-following"

type ActiveFollowingTimer struct {
	ID        string
	PathID    string
	PathName  string
	StartedAt time.Time
}

func (timer ActiveFollowingTimer) valid() bool {
	return strings.TrimSpace(timer.ID) != "" && strings.TrimSpace(timer.PathID) != "" && strings.TrimSpace(timer.PathName) != "" &&
		!timer.StartedAt.IsZero() && timer.StartedAt.Location() == time.UTC
}

type ActiveFollowingCandidate struct {
	ParticipantID     string
	Username          string
	DisplayName       string
	ProfilePictureURL string
	Timers            []ActiveFollowingTimer
}

func (candidate ActiveFollowingCandidate) valid() bool {
	if strings.TrimSpace(candidate.ParticipantID) == "" || strings.TrimSpace(candidate.Username) == "" || strings.TrimSpace(candidate.DisplayName) == "" || len(candidate.Timers) == 0 {
		return false
	}
	for index, timer := range candidate.Timers {
		if !timer.valid() || (index > 0 && (timer.StartedAt.Before(candidate.Timers[index-1].StartedAt) || (timer.StartedAt.Equal(candidate.Timers[index-1].StartedAt) && timer.ID <= candidate.Timers[index-1].ID))) {
			return false
		}
	}
	return true
}

type ActiveFollowingPageRequest struct {
	AfterParticipantID string
	Limit              int
}

type ActiveFollowingCandidatePage struct {
	Items   []ActiveFollowingCandidate
	HasMore bool
}

type ActiveFollowingRepository interface {
	ListActiveTimerCandidates(context.Context, string, ActiveFollowingPageRequest) (ActiveFollowingCandidatePage, error)
}

func (service *Service) ListActiveFollowing(ctx context.Context, authorization, cursor string, limit int) ([]ActiveFollowingCandidate, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if service.ActiveFollowing == nil || service.Authorizer == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil || len(service.CursorSigningKey) < 32 {
		return nil, "", errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return nil, "", errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	request := ActiveFollowingPageRequest{Limit: limit}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != activeFollowingCursorDomain || payload.AfterID == "" {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterParticipantID = payload.AfterID
	}

	items := make([]ActiveFollowingCandidate, 0, limit)
	hasMore := true
	for len(items) < limit && hasMore {
		request.Limit = limit - len(items)
		page, listErr := service.ActiveFollowing.ListActiveTimerCandidates(ctx, principal.UserID, request)
		if listErr != nil {
			return nil, "", listErr
		}
		if !validActiveFollowingCandidatePage(page, request) {
			return nil, "", errInvalidDependencies
		}
		for _, candidate := range page.Items {
			request.AfterParticipantID = candidate.ParticipantID
			visible := candidate
			visible.Timers = make([]ActiveFollowingTimer, 0, len(candidate.Timers))
			for _, timer := range candidate.Timers {
				allowed, checkErr := service.Authorizer.Check(ctx, "path", timer.PathID, "view", principal.UserID)
				if checkErr != nil {
					return nil, "", checkErr
				}
				if allowed {
					visible.Timers = append(visible.Timers, timer)
				}
			}
			if len(visible.Timers) > 0 {
				items = append(items, visible)
			}
		}
		hasMore = page.HasMore
	}

	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "social_feed", "active_following", audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if hasMore {
		next, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{Version: 1, Owner: principal.UserID, Domain: activeFollowingCursorDomain, AfterID: request.AfterParticipantID})
		if err != nil {
			return nil, "", err
		}
	}
	return items, next, nil
}

func validActiveFollowingCandidatePage(page ActiveFollowingCandidatePage, request ActiveFollowingPageRequest) bool {
	if len(page.Items) > request.Limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	previous := request.AfterParticipantID
	for _, item := range page.Items {
		if !item.valid() || item.ParticipantID <= previous {
			return false
		}
		previous = item.ParticipantID
	}
	return true
}
