package social

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledFeed struct {
	pages    []FeedCandidatePage
	requests []FeedPageRequest
	viewers  []string
	err      error
	event    PracticeFeedItem
}

func (feed *controlledFeed) GetPracticeCandidate(_ context.Context, viewer, eventID string, snapshot time.Time) (PracticeFeedItem, error) {
	feed.viewers = append(feed.viewers, viewer)
	if feed.err != nil {
		return PracticeFeedItem{}, feed.err
	}
	return feed.event, nil
}

type controlledActiveFollowing struct {
	pages    []ActiveFollowingCandidatePage
	viewers  []string
	requests []ActiveFollowingPageRequest
	err      error
}

func (active *controlledActiveFollowing) ListActiveTimerCandidates(_ context.Context, viewer string, request ActiveFollowingPageRequest) (ActiveFollowingCandidatePage, error) {
	active.viewers = append(active.viewers, viewer)
	active.requests = append(active.requests, request)
	if active.err != nil {
		return ActiveFollowingCandidatePage{}, active.err
	}
	page := active.pages[0]
	active.pages = active.pages[1:]
	return page, nil
}

func (feed *controlledFeed) ListPracticeCandidates(_ context.Context, viewer string, request FeedPageRequest) (FeedCandidatePage, error) {
	feed.viewers = append(feed.viewers, viewer)
	feed.requests = append(feed.requests, request)
	if feed.err != nil {
		return FeedCandidatePage{}, feed.err
	}
	page := feed.pages[0]
	feed.pages = feed.pages[1:]
	return page, nil
}

type feedAuthorizer struct {
	allowed map[string]bool
	err     error
	checks  []string
}

func (*feedAuthorizer) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (*feedAuthorizer) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (authorizer *feedAuthorizer) Check(_ context.Context, resourceType, resourceID, permission, userID string) (bool, error) {
	authorizer.checks = append(authorizer.checks, resourceType+":"+resourceID+":"+permission+":"+userID)
	return authorizer.allowed[resourceID], authorizer.err
}

func feedItem(id, path string, published time.Time) PracticeFeedItem {
	return PracticeFeedItem{ID: id, Type: FeedEventPracticeSession, PublishedAt: published, ParticipantID: "person", Username: "alex", DisplayName: "Alex", PathID: path, PathName: "Piano", ActivityID: "activity-" + id, DurationSeconds: 1800}
}

func achievementFeedItem(id, path string, published time.Time, kind AchievementKind) PracticeFeedItem {
	item := PracticeFeedItem{
		ID: id, Type: FeedEventGoalAchievement, PublishedAt: published,
		ParticipantID: "person", Username: "alex", DisplayName: "Alex", PathID: path, PathName: "Piano",
		Achievement: &GoalAchievement{Kind: kind, TargetSeconds: 3600},
	}
	if kind == AchievementInterval {
		item.Achievement.IntervalStartedAt = published.Add(-7 * 24 * time.Hour)
		item.Achievement.IntervalEndedAt = published.Add(24 * time.Hour)
	}
	return item
}

func feedService(feed *controlledFeed, authorizer *feedAuthorizer, audits *controlledAudits) *Service {
	service := testService(&controlledProfiles{}, audits)
	service.Feed, service.Authorizer = feed, authorizer
	return service
}

func TestGetPracticeFeedEventAppliesAuthoritativePathCheckAndAudits(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	item := feedItem("practice:old", "path", now.Add(-24*time.Hour))
	item.CommentsEnabled, item.ReactionsEnabled = false, true
	audits := &controlledAudits{}
	service := feedService(&controlledFeed{event: item}, &feedAuthorizer{allowed: map[string]bool{"path": true}}, audits)
	got, err := service.GetPracticeFeedEvent(context.Background(), "Bearer session", item.ID)
	if err != nil || got != item || len(audits.events) != 1 || audits.events[0].TargetType != "social_feed_event" || audits.events[0].TargetID != item.ID {
		t.Fatalf("got=%+v err=%v audits=%+v", got, err, audits.events)
	}
}

func TestGetPracticeFeedEventReturnsOpaqueNotFoundWhenPathCheckDenies(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	item := feedItem("achievement:old", "hidden", now.Add(-time.Hour))
	_, err := feedService(&controlledFeed{event: item}, &feedAuthorizer{allowed: map[string]bool{}}, &controlledAudits{}).GetPracticeFeedEvent(context.Background(), "Bearer session", item.ID)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestPracticeFeedFillsPageAfterCurrentSpiceDBFilteringAndSignsLastScannedCandidate(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	feed := &controlledFeed{pages: []FeedCandidatePage{
		{Items: []PracticeFeedItem{feedItem("event-3", "hidden", now.Add(-time.Minute)), feedItem("event-2", "shared", now.Add(-2*time.Minute))}, HasMore: true},
		{Items: []PracticeFeedItem{feedItem("event-1", "followed", now.Add(-3*time.Minute))}},
	}}
	authorizer := &feedAuthorizer{allowed: map[string]bool{"shared": true, "followed": true}}
	audits := &controlledAudits{}
	items, next, err := feedService(feed, authorizer, audits).ListPracticeFeed(context.Background(), "Bearer session", "", 2)
	if err != nil || len(items) != 2 || items[0].ID != "event-2" || items[1].ID != "event-1" || next != "" {
		t.Fatalf("items=%+v next=%q err=%v", items, next, err)
	}
	if len(feed.requests) != 2 || feed.requests[1].AfterID != "event-2" || feed.requests[1].Limit != 1 {
		t.Fatalf("requests=%+v", feed.requests)
	}
	if len(authorizer.checks) != 3 || len(audits.events) != 1 || audits.events[0].TargetType != "social_feed" || audits.events[0].TargetID != "feed" {
		t.Fatalf("checks=%+v audits=%+v", authorizer.checks, audits.events)
	}
}

func TestPracticeFeedMixesAuthorizedAchievementAndPracticeEventsWithoutChangingStableOrder(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	feed := &controlledFeed{pages: []FeedCandidatePage{{Items: []PracticeFeedItem{
		feedItem("practice:activity-2", "path", now.Add(-time.Minute)),
		func() PracticeFeedItem {
			item := achievementFeedItem("achievement:overall:activity-2", "path", now.Add(-time.Minute), AchievementOverall)
			item.Reactions = domain.ReactionSummary{Counts: domain.ReactionCounts{Celebrate: 2}, ViewerReaction: domain.ReactionCelebrate}
			return item
		}(),
		achievementFeedItem("achievement:interval:activity-1", "path", now.Add(-2*time.Minute), AchievementInterval),
	}}}}
	for _, item := range feed.pages[0].Items {
		if !item.valid() {
			t.Fatalf("invalid fixture: %+v", item)
		}
	}
	service := feedService(feed, &feedAuthorizer{allowed: map[string]bool{"path": true}}, &controlledAudits{})

	items, cursor, err := service.ListPracticeFeed(context.Background(), "Bearer session", "", 3)
	if err != nil || cursor != "" || len(items) != 3 ||
		items[0].Type != FeedEventPracticeSession || items[1].Achievement.Kind != AchievementOverall ||
		items[1].Type != FeedEventGoalAchievement || items[1].Reactions.Counts.Celebrate != 2 ||
		items[1].Reactions.ViewerReaction != domain.ReactionCelebrate || items[2].Achievement.Kind != AchievementInterval {
		t.Fatalf("items=%+v cursor=%q err=%v", items, cursor, err)
	}
}

func TestPracticeFeedCursorIsViewerBoundAndUsesStablePublicationOrder(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	feed := &controlledFeed{pages: []FeedCandidatePage{{Items: []PracticeFeedItem{feedItem("event-b", "path", now.Add(-time.Minute))}, HasMore: true}}}
	service := feedService(feed, &feedAuthorizer{allowed: map[string]bool{"path": true}}, &controlledAudits{})
	_, cursor, err := service.ListPracticeFeed(context.Background(), "Bearer session", "", 1)
	if err != nil || cursor == "" {
		t.Fatalf("cursor=%q err=%v", cursor, err)
	}
	feed.pages = []FeedCandidatePage{{Items: []PracticeFeedItem{feedItem("event-a", "path", now.Add(-time.Minute))}}}
	items, _, err := service.ListPracticeFeed(context.Background(), "Bearer session", cursor, 1)
	if err != nil || len(items) != 1 || feed.requests[1].AfterID != "event-b" || !feed.requests[1].AfterPublished.Equal(now.Add(-time.Minute)) {
		t.Fatalf("items=%+v requests=%+v err=%v", items, feed.requests, err)
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	if items, _, err := service.ListPracticeFeed(context.Background(), "Bearer session", cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) || items != nil {
		t.Fatalf("cross-viewer items=%+v err=%v", items, err)
	}
}

func TestPracticeFeedFailsClosedForInvalidDependenciesPolicyOrCandidateData(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	dependencyErr := errors.New("spicedb unavailable")
	tests := []struct {
		name      string
		configure func(*Service, *controlledFeed, *feedAuthorizer)
		want      error
	}{
		{name: "invalid limit", configure: func(_ *Service, _ *controlledFeed, _ *feedAuthorizer) {}, want: ports.ErrInvalidArgument},
		{name: "missing repository", configure: func(service *Service, _ *controlledFeed, _ *feedAuthorizer) { service.Feed = nil }, want: errInvalidDependencies},
		{name: "rate limited", configure: func(service *Service, _ *controlledFeed, _ *feedAuthorizer) {
			service.AuditRateLimiter = allowLimiter{}
		}, want: platformapp.ErrRateLimited},
		{name: "policy outage", configure: func(_ *Service, _ *controlledFeed, authorizer *feedAuthorizer) { authorizer.err = dependencyErr }, want: dependencyErr},
		{name: "private field shape invalid", configure: func(_ *Service, feed *controlledFeed, _ *feedAuthorizer) { feed.pages[0].Items[0].DurationSeconds = 0 }, want: errInvalidDependencies},
		{name: "mixed event shape invalid", configure: func(_ *Service, feed *controlledFeed, _ *feedAuthorizer) {
			feed.pages[0].Items[0] = achievementFeedItem("achievement", "path", now.Add(-time.Minute), AchievementOverall)
			feed.pages[0].Items[0].ActivityID = "private-activity-shape"
		}, want: errInvalidDependencies},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			feed := &controlledFeed{pages: []FeedCandidatePage{{Items: []PracticeFeedItem{feedItem("event", "path", now.Add(-time.Minute))}}}}
			authorizer := &feedAuthorizer{allowed: map[string]bool{"path": true}}
			service := feedService(feed, authorizer, &controlledAudits{})
			test.configure(service, feed, authorizer)
			limit := 1
			if test.name == "invalid limit" {
				limit = 0
			}
			items, cursor, err := service.ListPracticeFeed(context.Background(), "Bearer session", "", limit)
			if !errors.Is(err, test.want) || items != nil || cursor != "" {
				t.Fatalf("items=%+v cursor=%q err=%v", items, cursor, err)
			}
		})
	}
}

func TestActiveFollowingAuthorizesEveryPathAndGroupsOnlyVisibleTimers(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	active := &controlledActiveFollowing{pages: []ActiveFollowingCandidatePage{{Items: []ActiveFollowingCandidate{
		{ParticipantID: "person", Username: "alex", DisplayName: "Alex", Timers: []ActiveFollowingTimer{{ID: "timer-hidden", PathID: "hidden", PathName: "Private", StartedAt: now.Add(-time.Hour)}, {ID: "timer-piano", PathID: "piano", PathName: "Piano", StartedAt: now.Add(-30 * time.Minute)}, {ID: "timer-run", PathID: "run", PathName: "Running", StartedAt: now.Add(-10 * time.Minute)}}},
		{ParticipantID: "reader", Username: "sam", DisplayName: "Sam", Timers: []ActiveFollowingTimer{{ID: "timer-read", PathID: "read", PathName: "Reading", StartedAt: now.Add(-5 * time.Minute)}}},
	}}}}
	authorizer := &feedAuthorizer{allowed: map[string]bool{"piano": true, "run": true, "read": true}}
	audits := &controlledAudits{}
	service := feedService(&controlledFeed{}, authorizer, audits)
	service.ActiveFollowing = active

	items, next, err := service.ListActiveFollowing(context.Background(), "Bearer session", "", 2)
	if err != nil || next != "" || len(items) != 2 || len(items[0].Timers) != 2 || items[0].ParticipantID != "person" || items[0].Timers[0].ID != "timer-piano" || items[1].ParticipantID != "reader" {
		t.Fatalf("items=%+v next=%q err=%v", items, next, err)
	}
	if len(authorizer.checks) != 4 || len(audits.events) != 1 || audits.events[0].TargetType != "social_feed" || audits.events[0].TargetID != "active_following" {
		t.Fatalf("checks=%+v audits=%+v", authorizer.checks, audits.events)
	}
}

func TestActiveFollowingFailsClosedForPolicyOutageOrInvalidCandidate(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	dependencyErr := errors.New("spicedb unavailable")
	for _, test := range []struct {
		name      string
		configure func(*Service, *controlledActiveFollowing, *feedAuthorizer)
		want      error
	}{
		{name: "missing repository", configure: func(service *Service, _ *controlledActiveFollowing, _ *feedAuthorizer) { service.ActiveFollowing = nil }, want: errInvalidDependencies},
		{name: "policy outage", configure: func(_ *Service, _ *controlledActiveFollowing, authorizer *feedAuthorizer) {
			authorizer.err = dependencyErr
		}, want: dependencyErr},
		{name: "invalid candidate", configure: func(_ *Service, active *controlledActiveFollowing, _ *feedAuthorizer) {
			active.pages[0].Items[0].Timers[0].StartedAt = time.Time{}
		}, want: errInvalidDependencies},
	} {
		t.Run(test.name, func(t *testing.T) {
			active := &controlledActiveFollowing{pages: []ActiveFollowingCandidatePage{{Items: []ActiveFollowingCandidate{{ParticipantID: "person", Username: "alex", DisplayName: "Alex", Timers: []ActiveFollowingTimer{{ID: "timer", PathID: "path", PathName: "Piano", StartedAt: now}}}}}}}
			authorizer := &feedAuthorizer{allowed: map[string]bool{"path": true}}
			service := feedService(&controlledFeed{}, authorizer, &controlledAudits{})
			service.ActiveFollowing = active
			test.configure(service, active, authorizer)
			items, cursor, err := service.ListActiveFollowing(context.Background(), "Bearer session", "", 1)
			if !errors.Is(err, test.want) || items != nil || cursor != "" {
				t.Fatalf("items=%+v cursor=%q err=%v", items, cursor, err)
			}
		})
	}
}

func TestActiveFollowingFillsVisibleParticipantPageAndBindsCursorToViewer(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	active := &controlledActiveFollowing{pages: []ActiveFollowingCandidatePage{
		{Items: []ActiveFollowingCandidate{{ParticipantID: "a", Username: "a-user", DisplayName: "A", Timers: []ActiveFollowingTimer{{ID: "a-timer", PathID: "hidden", PathName: "Hidden", StartedAt: now}}}}, HasMore: true},
		{Items: []ActiveFollowingCandidate{{ParticipantID: "b", Username: "b-user", DisplayName: "B", Timers: []ActiveFollowingTimer{{ID: "b-timer", PathID: "visible", PathName: "Visible", StartedAt: now}}}}, HasMore: true},
	}}
	service := feedService(&controlledFeed{}, &feedAuthorizer{allowed: map[string]bool{"visible": true}}, &controlledAudits{})
	service.ActiveFollowing = active
	items, cursor, err := service.ListActiveFollowing(context.Background(), "Bearer session", "", 1)
	if err != nil || len(items) != 1 || items[0].ParticipantID != "b" || cursor == "" || len(active.requests) != 2 || active.requests[1].AfterParticipantID != "a" {
		t.Fatalf("items=%+v cursor=%q requests=%+v err=%v", items, cursor, active.requests, err)
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	if items, next, err := service.ListActiveFollowing(context.Background(), "Bearer session", cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) || items != nil || next != "" {
		t.Fatalf("cross-viewer items=%+v next=%q err=%v", items, next, err)
	}
}
