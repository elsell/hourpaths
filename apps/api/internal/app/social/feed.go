package social

import (
	"context"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const feedCursorDomain = "social-practice-feed"

type FeedEventType string

const (
	FeedEventPracticeSession FeedEventType = "practice_session"
	FeedEventGoalAchievement FeedEventType = "goal_achievement"
)

type AchievementKind string

const (
	AchievementInterval AchievementKind = "interval"
	AchievementOverall  AchievementKind = "overall"
)

type GoalAchievement struct {
	Kind              AchievementKind
	TargetSeconds     int64
	IntervalStartedAt time.Time
	IntervalEndedAt   time.Time
}

func (achievement GoalAchievement) valid() bool {
	if achievement.TargetSeconds < 1 {
		return false
	}
	switch achievement.Kind {
	case AchievementOverall:
		return achievement.IntervalStartedAt.IsZero() && achievement.IntervalEndedAt.IsZero()
	case AchievementInterval:
		return !achievement.IntervalStartedAt.IsZero() && !achievement.IntervalEndedAt.IsZero() &&
			achievement.IntervalStartedAt.Location() == time.UTC && achievement.IntervalEndedAt.Location() == time.UTC &&
			achievement.IntervalEndedAt.After(achievement.IntervalStartedAt)
	default:
		return false
	}
}

type PracticeFeedItem struct {
	ID                string
	Type              FeedEventType
	PublishedAt       time.Time
	ParticipantID     string
	Username          string
	DisplayName       string
	ProfilePictureURL string
	PathID            string
	PathName          string
	ActivityID        string
	DurationSeconds   int64
	Edited            bool
	Achievement       *GoalAchievement
	Reactions         domain.ReactionSummary
	CommentsEnabled   bool
	ReactionsEnabled  bool
}

func (item PracticeFeedItem) valid() bool {
	common := strings.TrimSpace(item.ID) != "" && !item.PublishedAt.IsZero() && item.PublishedAt.Location() == time.UTC &&
		strings.TrimSpace(item.ParticipantID) != "" && strings.TrimSpace(item.Username) != "" && strings.TrimSpace(item.DisplayName) != "" &&
		strings.TrimSpace(item.PathID) != "" && strings.TrimSpace(item.PathName) != "" && item.Reactions.Valid()
	if !common {
		return false
	}
	switch item.Type {
	case FeedEventPracticeSession:
		return strings.TrimSpace(item.ActivityID) != "" && item.DurationSeconds > 0 && item.Achievement == nil
	case FeedEventGoalAchievement:
		return item.ActivityID == "" && item.DurationSeconds == 0 && !item.Edited && item.Achievement != nil &&
			item.Achievement.valid()
	default:
		return false
	}
}

type FeedPageRequest struct {
	AfterID        string
	AfterPublished time.Time
	Snapshot       time.Time
	Limit          int
}

type FeedCandidatePage struct {
	Items   []PracticeFeedItem
	HasMore bool
}

type FeedRepository interface {
	ListPracticeCandidates(context.Context, string, FeedPageRequest) (FeedCandidatePage, error)
	GetPracticeCandidate(context.Context, string, string, time.Time) (PracticeFeedItem, error)
}

func (service *Service) GetPracticeFeedEvent(ctx context.Context, authorization, eventID string) (PracticeFeedItem, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return PracticeFeedItem{}, err
	}
	if strings.TrimSpace(eventID) == "" {
		return PracticeFeedItem{}, ports.ErrInvalidArgument
	}
	if service.Feed == nil || service.Authorizer == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil {
		return PracticeFeedItem{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return PracticeFeedItem{}, errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return PracticeFeedItem{}, platformapp.ErrRateLimited
	}
	item, err := service.Feed.GetPracticeCandidate(ctx, principal.UserID, eventID, now)
	if err != nil {
		return PracticeFeedItem{}, err
	}
	if !item.valid() || item.ID != eventID || item.PublishedAt.After(now) {
		return PracticeFeedItem{}, errInvalidDependencies
	}
	allowed, err := service.Authorizer.Check(ctx, "path", item.PathID, "view", principal.UserID)
	if err != nil {
		return PracticeFeedItem{}, err
	}
	if !allowed {
		return PracticeFeedItem{}, ports.ErrNotFound
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "social_feed_event", eventID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return PracticeFeedItem{}, err
	}
	return item, nil
}

func (service *Service) ListPracticeFeed(ctx context.Context, authorization, cursor string, limit int) ([]PracticeFeedItem, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	if limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if service.Feed == nil || service.Authorizer == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil || len(service.CursorSigningKey) < 32 {
		return nil, "", errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return nil, "", errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(principal.UserID, now) {
		return nil, "", platformapp.ErrRateLimited
	}
	request := FeedPageRequest{Snapshot: now, Limit: limit}
	if cursor != "" {
		payload, decodeErr := shared.DecodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || payload.Owner != principal.UserID || payload.Domain != feedCursorDomain {
			return nil, "", ports.ErrInvalidArgument
		}
		request.AfterID = payload.AfterID
		request.AfterPublished = payload.AfterCreated
		request.Snapshot = payload.Snapshot
	}

	items := make([]PracticeFeedItem, 0, limit)
	hasMore := true
	for len(items) < limit && hasMore {
		request.Limit = limit - len(items)
		page, listErr := service.Feed.ListPracticeCandidates(ctx, principal.UserID, request)
		if listErr != nil {
			return nil, "", listErr
		}
		if !validFeedCandidatePage(page, request) {
			return nil, "", errInvalidDependencies
		}
		for _, candidate := range page.Items {
			allowed, checkErr := service.Authorizer.Check(ctx, "path", candidate.PathID, "view", principal.UserID)
			if checkErr != nil {
				return nil, "", checkErr
			}
			request.AfterPublished, request.AfterID = candidate.PublishedAt, candidate.ID
			if allowed {
				items = append(items, candidate)
			}
		}
		hasMore = page.HasMore
	}

	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "social_feed", "feed", audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if hasMore {
		next, err = shared.EncodeCursor(service.CursorSigningKey, shared.CursorPayload{
			Version: 1, Owner: principal.UserID, Domain: feedCursorDomain,
			AfterID: request.AfterID, AfterCreated: request.AfterPublished, Snapshot: request.Snapshot,
		})
		if err != nil {
			return nil, "", err
		}
	}
	return items, next, nil
}

func validFeedCandidatePage(page FeedCandidatePage, request FeedPageRequest) bool {
	if len(page.Items) > request.Limit || (page.HasMore && len(page.Items) == 0) {
		return false
	}
	previousTime, previousID := request.AfterPublished, request.AfterID
	for _, item := range page.Items {
		if !item.valid() || item.PublishedAt.After(request.Snapshot) {
			return false
		}
		if !previousTime.IsZero() && (item.PublishedAt.After(previousTime) || (item.PublishedAt.Equal(previousTime) && item.ID >= previousID)) {
			return false
		}
		previousTime, previousID = item.PublishedAt, item.ID
	}
	return true
}
