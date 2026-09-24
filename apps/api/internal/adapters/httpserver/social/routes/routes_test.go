package routes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/dto"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledService struct {
	profiles                                        []domain.PublicProfile
	profile                                         domain.PublicProfile
	cursor                                          string
	err                                             error
	authorization, query, username, receivedCursor  string
	limit                                           int
	relationship                                    application.RelationshipResult
	review                                          application.ReviewResult
	requests                                        []domain.FollowRequest
	feedItems                                       []application.PracticeFeedItem
	feedEvent                                       application.PracticeFeedItem
	activeItems                                     []application.ActiveFollowingCandidate
	reactionSummary                                 domain.ReactionSummary
	eventID, reactionKey                            string
	reaction                                        domain.Reaction
	commentItems                                    []application.PracticeCommentItem
	commentVersions                                 []domain.CommentVersion
	comment                                         domain.Comment
	commentDelete                                   application.CommentDeleteResult
	commentID, commentText, commentKey, commentCall string
	expectedVersion                                 int64
	heartSummary                                    application.CommentHeartSummary
	heartProfiles                                   []domain.PublicProfile
	interactionSettings                             application.InteractionSettings
	interactionKey                                  string
	blockReview                                     application.BlockReview
	blockResult                                     application.BlockMutationResult
	blockedAccounts                                 []domain.BlockedAccount
	blockKey, targetUserID                          string
	blockAcknowledgement                            application.BlockReviewAcknowledgement
	nudgePreference                                 application.NudgeAudiencePreference
	nudgeEligibility                                application.NudgeEligibility
	nudge                                           domain.Nudge
	nudgePathID, nudgeRecipientID, nudgeKey         string
	nudgeAudience                                   domain.NudgeAudience
	nudgeContent                                    domain.NudgeContent
	nudgeExpectedRevision                           int64
	nudgeNotificationChannel                        application.NudgeNotificationChannelPreference
	nudgeNotificationKey                            string
	nudgeNotificationExpectedRevision               int64
	nudgeNotificationEnabled                        bool
}

func (service *controlledService) GetNudgeNotificationChannel(_ context.Context, authorization string) (application.NudgeNotificationChannelPreference, error) {
	service.authorization = authorization
	return service.nudgeNotificationChannel, service.err
}

func (service *controlledService) UpdateNudgeNotificationChannel(_ context.Context, authorization, key string, expectedRevision int64, enabled bool) (application.NudgeNotificationChannelPreference, error) {
	service.authorization, service.nudgeNotificationKey = authorization, key
	service.nudgeNotificationExpectedRevision, service.nudgeNotificationEnabled = expectedRevision, enabled
	return service.nudgeNotificationChannel, service.err
}

func (service *controlledService) GetNudgeAudience(_ context.Context, authorization, pathID string) (application.NudgeAudiencePreference, error) {
	service.authorization, service.nudgePathID = authorization, pathID
	return service.nudgePreference, service.err
}

func (service *controlledService) UpdateNudgeAudience(_ context.Context, authorization, pathID, key string, expectedRevision int64, audience domain.NudgeAudience) (application.NudgeAudiencePreference, error) {
	service.authorization, service.nudgePathID, service.nudgeKey = authorization, pathID, key
	service.nudgeExpectedRevision, service.nudgeAudience = expectedRevision, audience
	return service.nudgePreference, service.err
}

func (service *controlledService) GetNudgeEligibility(_ context.Context, authorization, pathID, recipientUserID string) (application.NudgeEligibility, error) {
	service.authorization, service.nudgePathID, service.nudgeRecipientID = authorization, pathID, recipientUserID
	return service.nudgeEligibility, service.err
}

func (service *controlledService) SendNudge(_ context.Context, authorization, pathID, recipientUserID string, content domain.NudgeContent, key string) (domain.Nudge, error) {
	service.authorization, service.nudgePathID, service.nudgeRecipientID, service.nudgeKey = authorization, pathID, recipientUserID, key
	service.nudgeContent = content
	return service.nudge, service.err
}

func (service *controlledService) ReviewBlock(_ context.Context, authorization, username string) (application.BlockReview, error) {
	service.authorization, service.username = authorization, username
	return service.blockReview, service.err
}
func (service *controlledService) BlockUser(_ context.Context, authorization, username, key string, acknowledgement application.BlockReviewAcknowledgement) (application.BlockMutationResult, error) {
	service.authorization, service.username, service.blockKey = authorization, username, key
	service.blockAcknowledgement = acknowledgement
	return service.blockResult, service.err
}
func (service *controlledService) ListBlockedAccounts(_ context.Context, authorization, cursor string, limit int) ([]domain.BlockedAccount, string, error) {
	service.authorization, service.receivedCursor, service.limit = authorization, cursor, limit
	return service.blockedAccounts, service.cursor, service.err
}
func (service *controlledService) UnblockUser(_ context.Context, authorization, targetUserID, key string) (application.BlockMutationResult, error) {
	service.authorization, service.targetUserID, service.blockKey = authorization, targetUserID, key
	return service.blockResult, service.err
}
func (service *controlledService) GetInteractionSettings(_ context.Context, authorization string) (application.InteractionSettings, error) {
	service.authorization = authorization
	return service.interactionSettings, service.err
}
func (service *controlledService) UpdateInteractionSettings(_ context.Context, authorization, key string, settings application.InteractionSettings) (application.InteractionSettings, error) {
	service.authorization, service.interactionKey, service.interactionSettings = authorization, key, settings
	return settings, service.err
}
func (service *controlledService) Search(_ context.Context, authorization, query, cursor string, limit int) ([]domain.PublicProfile, string, error) {
	service.authorization, service.query, service.receivedCursor, service.limit = authorization, query, cursor, limit
	return service.profiles, service.cursor, service.err
}
func (service *controlledService) Get(_ context.Context, authorization, username string) (domain.PublicProfile, error) {
	service.authorization, service.username = authorization, username
	return service.profile, service.err
}
func (service *controlledService) Follow(_ context.Context, authorization, username, _ string) (application.RelationshipResult, error) {
	service.authorization, service.username = authorization, username
	return service.relationship, service.err
}
func (service *controlledService) CancelRequest(_ context.Context, authorization, username, _ string) (application.RelationshipResult, error) {
	service.authorization, service.username = authorization, username
	return service.relationship, service.err
}
func (service *controlledService) Unfollow(_ context.Context, authorization, username, _ string) (application.RelationshipResult, error) {
	service.authorization, service.username = authorization, username
	return service.relationship, service.err
}
func (service *controlledService) AcceptRequest(_ context.Context, authorization, requestID, _ string) (application.ReviewResult, error) {
	service.authorization, service.username = authorization, requestID
	return service.review, service.err
}
func (service *controlledService) RejectRequest(_ context.Context, authorization, requestID, _ string) (application.ReviewResult, error) {
	service.authorization, service.username = authorization, requestID
	return service.review, service.err
}
func (service *controlledService) ListIncomingRequests(_ context.Context, authorization, cursor string, limit int) ([]domain.FollowRequest, string, error) {
	service.authorization, service.receivedCursor, service.limit = authorization, cursor, limit
	return service.requests, service.cursor, service.err
}
func (service *controlledService) ListPracticeFeed(_ context.Context, authorization, cursor string, limit int) ([]application.PracticeFeedItem, string, error) {
	service.authorization, service.receivedCursor, service.limit = authorization, cursor, limit
	return service.feedItems, service.cursor, service.err
}
func (service *controlledService) GetPracticeFeedEvent(_ context.Context, authorization, eventID string) (application.PracticeFeedItem, error) {
	service.authorization, service.eventID = authorization, eventID
	return service.feedEvent, service.err
}
func (service *controlledService) ListActiveFollowing(_ context.Context, authorization, cursor string, limit int) ([]application.ActiveFollowingCandidate, string, error) {
	service.authorization, service.receivedCursor, service.limit = authorization, cursor, limit
	return service.activeItems, service.cursor, service.err
}
func (service *controlledService) SetPracticeReaction(_ context.Context, authorization, eventID string, reaction domain.Reaction, idempotencyKey string) (domain.ReactionSummary, error) {
	service.authorization, service.eventID, service.reaction, service.reactionKey = authorization, eventID, reaction, idempotencyKey
	return service.reactionSummary, service.err
}
func (service *controlledService) RemovePracticeReaction(_ context.Context, authorization, eventID, idempotencyKey string) (domain.ReactionSummary, error) {
	service.authorization, service.eventID, service.reactionKey = authorization, eventID, idempotencyKey
	return service.reactionSummary, service.err
}

func (service *controlledService) ListPracticeComments(_ context.Context, authorization, eventID, cursor string, limit int) ([]application.PracticeCommentItem, string, error) {
	service.authorization, service.eventID, service.receivedCursor, service.limit, service.commentCall = authorization, eventID, cursor, limit, "list"
	return service.commentItems, service.cursor, service.err
}

func (service *controlledService) ListCommentHistory(_ context.Context, authorization, eventID, commentID, cursor string, limit int) ([]domain.CommentVersion, string, error) {
	service.authorization, service.eventID, service.commentID, service.receivedCursor, service.limit, service.commentCall = authorization, eventID, commentID, cursor, limit, "history"
	return service.commentVersions, service.cursor, service.err
}

func (service *controlledService) CreatePracticeComment(_ context.Context, authorization, eventID, text, idempotencyKey string) (domain.Comment, error) {
	service.authorization, service.eventID, service.commentText, service.commentKey, service.commentCall = authorization, eventID, text, idempotencyKey, "create"
	return service.comment, service.err
}

func (service *controlledService) EditPracticeComment(_ context.Context, authorization, eventID, commentID, text string, expectedVersion int64, idempotencyKey string) (domain.Comment, error) {
	service.authorization, service.eventID, service.commentID, service.commentText, service.expectedVersion, service.commentKey, service.commentCall = authorization, eventID, commentID, text, expectedVersion, idempotencyKey, "edit"
	return service.comment, service.err
}

func (service *controlledService) DeletePracticeComment(_ context.Context, authorization, eventID, commentID, idempotencyKey string) (application.CommentDeleteResult, error) {
	service.authorization, service.eventID, service.commentID, service.commentKey, service.commentCall = authorization, eventID, commentID, idempotencyKey, "delete"
	return service.commentDelete, service.err
}

func (service *controlledService) SetPracticeCommentHeart(_ context.Context, authorization, eventID, commentID, idempotencyKey string) (application.CommentHeartSummary, error) {
	service.authorization, service.eventID, service.commentID, service.commentKey, service.commentCall = authorization, eventID, commentID, idempotencyKey, "heart"
	return service.heartSummary, service.err
}
func (service *controlledService) RemovePracticeCommentHeart(_ context.Context, authorization, eventID, commentID, idempotencyKey string) (application.CommentHeartSummary, error) {
	service.authorization, service.eventID, service.commentID, service.commentKey, service.commentCall = authorization, eventID, commentID, idempotencyKey, "unheart"
	return service.heartSummary, service.err
}
func (service *controlledService) ListPracticeCommentHearts(_ context.Context, authorization, eventID, commentID, cursor string, limit int) ([]domain.PublicProfile, string, error) {
	service.authorization, service.eventID, service.commentID, service.receivedCursor, service.limit, service.commentCall = authorization, eventID, commentID, cursor, limit, "hearts"
	return service.heartProfiles, service.cursor, service.err
}

func handler(service Service) http.Handler {
	handler, _ := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, service) }}})
	return handler
}

func TestSearchProfilesReturnsOnlySafeProjectionAndPagination(t *testing.T) {
	service := &controlledService{profiles: []domain.PublicProfile{{ID: "user-1", Username: "alice", DisplayName: "Alice", ProfilePictureURL: "https://images.example/alice.webp", Description: "Runs", FollowerCount: 2, FollowingCount: 3, Relationship: domain.RelationshipNone}}, cursor: "next"}
	request := httptest.NewRequest(http.MethodGet, "/v1/profiles?query=al&cursor=previous&limit=20", nil)
	request.Header.Set("Authorization", "Bearer application-session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.authorization != "Bearer application-session" || service.query != "al" || service.receivedCursor != "previous" || service.limit != 20 {
		t.Fatalf("service=%+v", service)
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	encoded := response.Body.String()
	for _, forbidden := range []string{"email", "profileVisibility", "status", "path", "goal", "activity", "provider"} {
		if json.Valid(response.Body.Bytes()) && containsJSONKey(body, forbidden) {
			t.Fatalf("response exposes forbidden %q: %s", forbidden, encoded)
		}
	}
	data := body["data"].([]any)[0].(map[string]any)
	if data["id"] != "user-1" || data["followerCount"] != float64(2) || body["meta"].(map[string]any)["nextCursor"] != "next" {
		t.Fatalf("body=%s", encoded)
	}
}

func containsJSONKey(value any, forbidden string) bool {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == forbidden || containsJSONKey(child, forbidden) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsJSONKey(child, forbidden) {
				return true
			}
		}
	}
	return false
}

func TestGetProfileMapsUnavailableAndHiddenProfilesWithoutLeakage(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want int
	}{{"hidden", ports.ErrNotFound, http.StatusNotFound}, {"dependency", fmt.Errorf("database: %w", ports.ErrUnavailable), http.StatusServiceUnavailable}} {
		t.Run(test.name, func(t *testing.T) {
			service := &controlledService{err: test.err}
			request := httptest.NewRequest(http.MethodGet, "/v1/profiles/alice", nil)
			request.Header.Set("Authorization", "Bearer session")
			response := httptest.NewRecorder()
			handler(service).ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestProfileDiscoveryContractIsAuthenticatedAndBounded(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	search := api.OpenAPI().Paths["/v1/profiles"].Get
	detail := api.OpenAPI().Paths["/v1/profiles/{username}"].Get
	if search == nil || detail == nil || len(search.Security) == 0 || len(detail.Security) == 0 {
		t.Fatal("profile discovery routes must be authenticated")
	}
	if search.OperationID != "search-profiles" || detail.OperationID != "get-profile" {
		t.Fatalf("operation IDs=%q,%q", search.OperationID, detail.OperationID)
	}
}

func TestPracticeFeedReturnsOnlyTheCompactPublicProjection(t *testing.T) {
	publishedAt := time.Date(2026, 7, 27, 15, 4, 5, 0, time.UTC)
	service := &controlledService{feedItems: []application.PracticeFeedItem{{
		ID: "practice:activity-1", Type: application.FeedEventPracticeSession, PublishedAt: publishedAt,
		ParticipantID: "user-1", Username: "alice", DisplayName: "Alice", ProfilePictureURL: "https://images.example/alice.webp",
		PathID: "path-1", PathName: "Piano", ActivityID: "activity-1", DurationSeconds: 1800, Edited: true,
		Reactions:       domain.ReactionSummary{Counts: domain.ReactionCounts{Heart: 2, Fire: 1}, ViewerReaction: domain.ReactionFire},
		CommentsEnabled: true, ReactionsEnabled: true,
	}}, cursor: "next-signed"}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed?cursor=previous-signed&limit=20", nil)
	request.Header.Set("Authorization", "Bearer application-session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.authorization != "Bearer application-session" || service.receivedCursor != "previous-signed" || service.limit != 20 {
		t.Fatalf("service=%+v", service)
	}
	var body struct {
		Data struct {
			Items []struct {
				ID          string    `json:"id"`
				Type        string    `json:"type"`
				PublishedAt time.Time `json:"publishedAt"`
				Participant struct {
					UserID            string `json:"userId"`
					Username          string `json:"username"`
					DisplayName       string `json:"displayName"`
					ProfilePictureURL string `json:"profilePictureURL"`
				} `json:"participant"`
				Path     struct{ ID, Name string } `json:"path"`
				Activity *struct {
					ID              string `json:"id"`
					DurationSeconds int64  `json:"durationSeconds"`
					Edited          bool   `json:"edited"`
				} `json:"activity"`
				Reactions struct {
					Heart, Applause, Fire, Strong, Celebrate int64
				} `json:"reactions"`
				ViewerReaction   *string `json:"viewerReaction"`
				CommentsEnabled  bool    `json:"commentsEnabled"`
				ReactionsEnabled bool    `json:"reactionsEnabled"`
			} `json:"items"`
		} `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if !body.Data.Items[0].CommentsEnabled || !body.Data.Items[0].ReactionsEnabled {
		t.Fatalf("feed must project authoritative interaction flags: %s", response.Body.String())
	}
	if len(body.Data.Items) != 1 || body.Data.Items[0].ID != "practice:activity-1" || body.Data.Items[0].Type != "practice_session" ||
		body.Data.Items[0].PublishedAt != publishedAt || body.Data.Items[0].Participant.UserID != "user-1" ||
		body.Data.Items[0].Path.ID != "path-1" || body.Data.Items[0].Activity == nil || body.Data.Items[0].Activity.DurationSeconds != 1800 || !body.Data.Items[0].Activity.Edited ||
		body.Data.Items[0].Reactions.Heart != 2 || body.Data.Items[0].Reactions.Fire != 1 || body.Data.Items[0].ViewerReaction == nil || *body.Data.Items[0].ViewerReaction != "fire" ||
		body.Meta.NextCursor != "next-signed" {
		t.Fatalf("body=%s", response.Body.String())
	}
	decoded := mustJSON(t, response.Body.Bytes())
	for _, forbidden := range []string{"note", "occurrenceTimeZone", "startedAt", "endedAt", "email", "profileVisibility"} {
		if containsJSONKey(decoded, forbidden) {
			t.Fatalf("feed exposes forbidden %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestPracticeFeedEventRouteReturnsSameStrictProjection(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	service := &controlledService{feedEvent: application.PracticeFeedItem{ID: "practice:old", Type: application.FeedEventPracticeSession, PublishedAt: now, ParticipantID: "owner", Username: "owner", DisplayName: "Owner", PathID: "path", PathName: "Piano", ActivityID: "activity", DurationSeconds: 60, CommentsEnabled: false, ReactionsEnabled: true}}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed/practice%3Aold", nil)
	request.Header.Set("Authorization", "Bearer viewer")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	item := mustJSON(t, response.Body.Bytes()).(map[string]any)["data"].(map[string]any)
	if item["id"] != "practice:old" || item["commentsEnabled"] != false || item["reactionsEnabled"] != true || service.eventID != "practice:old" {
		t.Fatalf("item=%+v service=%+v", item, service)
	}
}

func TestGoalAchievementFeedReturnsDiscriminatedCompactProjection(t *testing.T) {
	publishedAt := time.Date(2026, 7, 28, 15, 4, 5, 0, time.UTC)
	intervalStart := time.Date(2026, 7, 27, 4, 0, 0, 0, time.UTC)
	intervalEnd := intervalStart.Add(7 * 24 * time.Hour)
	service := &controlledService{feedItems: []application.PracticeFeedItem{{
		ID: "achievement:interval:activity-1", Type: application.FeedEventGoalAchievement, PublishedAt: publishedAt,
		ParticipantID: "user-1", Username: "alice", DisplayName: "Alice",
		PathID: "path-1", PathName: "Piano",
		Achievement: &application.GoalAchievement{
			Kind: application.AchievementInterval, TargetSeconds: 3600,
			IntervalStartedAt: intervalStart, IntervalEndedAt: intervalEnd,
		},
		CommentsEnabled: true, ReactionsEnabled: true,
	}}}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed?limit=25", nil)
	request.Header.Set("Authorization", "Bearer application-session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	decoded := mustJSON(t, response.Body.Bytes())
	data := decoded.(map[string]any)["data"].(map[string]any)
	item := data["items"].([]any)[0].(map[string]any)
	achievement, exists := item["achievement"].(map[string]any)
	if !exists {
		t.Fatalf("achievement missing: %s", response.Body.String())
	}
	if item["type"] != "goal_achievement" || item["id"] != "achievement:interval:activity-1" ||
		achievement["kind"] != "interval" || achievement["targetSeconds"] != float64(3600) ||
		achievement["intervalStartedAt"] != intervalStart.Format(time.RFC3339) ||
		achievement["intervalEndedAt"] != intervalEnd.Format(time.RFC3339) {
		t.Fatalf("body=%s", response.Body.String())
	}
	for _, forbidden := range []string{"activity", "note", "occurrenceTimeZone"} {
		if _, exists := item[forbidden]; exists {
			t.Fatalf("achievement exposes forbidden %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestPracticeReactionMutationsAreAuthenticatedIdempotentAndAuthoritative(t *testing.T) {
	for _, test := range []struct {
		name, method, body, reaction string
	}{
		{name: "set", method: http.MethodPut, body: `{"reaction":"fire"}`, reaction: "fire"},
		{name: "remove", method: http.MethodDelete},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &controlledService{reactionSummary: domain.ReactionSummary{
				Counts:         domain.ReactionCounts{Heart: 2, Applause: 3, Fire: 4, Strong: 5, Celebrate: 6},
				ViewerReaction: domain.Reaction(test.reaction),
			}}
			request := httptest.NewRequest(test.method, "/v1/social/feed/practice%3Aactivity-1/reaction", strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer application-session")
			request.Header.Set("Idempotency-Key", "0123456789abcdef")
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler(service).ServeHTTP(response, request)
			if response.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if service.authorization != "Bearer application-session" || service.eventID != "practice:activity-1" || service.reactionKey != "0123456789abcdef" || string(service.reaction) != test.reaction {
				t.Fatalf("service=%+v", service)
			}
			var body struct {
				Data struct {
					Reactions      struct{ Heart, Applause, Fire, Strong, Celebrate int64 } `json:"reactions"`
					ViewerReaction *string                                                  `json:"viewerReaction"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Data.Reactions.Heart != 2 || body.Data.Reactions.Applause != 3 || body.Data.Reactions.Fire != 4 || body.Data.Reactions.Strong != 5 || body.Data.Reactions.Celebrate != 6 {
				t.Fatalf("body=%s", response.Body.String())
			}
			if test.reaction == "" && body.Data.ViewerReaction != nil {
				t.Fatalf("remove must return null viewerReaction: %s", response.Body.String())
			}
			if test.reaction != "" && (body.Data.ViewerReaction == nil || *body.Data.ViewerReaction != test.reaction) {
				t.Fatalf("set must return authoritative viewerReaction: %s", response.Body.String())
			}
		})
	}
}

func TestPracticeReactionContractRejectsMissingKeyAndUncuratedReaction(t *testing.T) {
	for _, test := range []struct{ name, method, body string }{
		{name: "missing key", method: http.MethodPut, body: `{"reaction":"heart"}`},
		{name: "custom reaction", method: http.MethodPut, body: `{"reaction":"thumbs_down"}`},
		{name: "remove missing key", method: http.MethodDelete},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "/v1/social/feed/practice%3Aactivity-1/reaction", strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer session")
			request.Header.Set("Content-Type", "application/json")
			if test.name == "custom reaction" {
				request.Header.Set("Idempotency-Key", "0123456789abcdef")
			}
			response := httptest.NewRecorder()
			handler(&controlledService{}).ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestPracticeReactionContractRegistersSecuredPutAndDelete(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	path := api.OpenAPI().Paths["/v1/social/feed/{eventId}/reaction"]
	if path == nil || path.Put == nil || path.Delete == nil || len(path.Put.Security) == 0 || len(path.Delete.Security) == 0 || path.Put.OperationID != "set-practice-reaction" || path.Delete.OperationID != "remove-practice-reaction" {
		t.Fatalf("reaction path=%+v", path)
	}
}

func TestPracticeReactionMapsFailuresWithoutLeakingDetails(t *testing.T) {
	service := &controlledService{err: fmt.Errorf("reaction repository detail: %w", ports.ErrUnavailable)}
	request := httptest.NewRequest(http.MethodPut, "/v1/social/feed/practice%3Aactivity-1/reaction", strings.NewReader(`{"reaction":"heart"}`))
	request.Header.Set("Authorization", "Bearer session")
	request.Header.Set("Idempotency-Key", "0123456789abcdef")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "repository detail") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPracticeFeedContractIsAuthenticatedAndBounded(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	operation := api.OpenAPI().Paths["/v1/social/feed"].Get
	if operation == nil || len(operation.Security) == 0 || operation.OperationID != "list-practice-feed" || operation.Summary != "List chronological feed events" {
		t.Fatalf("practice feed operation=%+v", operation)
	}

	for _, path := range []string{"/v1/social/feed?limit=0", "/v1/social/feed?limit=101"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler(&controlledService{}).ServeHTTP(response, request)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestPracticeFeedMapsApplicationFailuresWithoutLeakingDetails(t *testing.T) {
	service := &controlledService{err: fmt.Errorf("feed repository detail: %w", ports.ErrUnavailable)}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed", nil)
	request.Header.Set("Authorization", "Bearer session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "repository detail") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestActiveFollowingReturnsGroupedTimerProjectionAndPagination(t *testing.T) {
	startedAt := time.Date(2026, 7, 27, 15, 4, 5, 0, time.UTC)
	service := &controlledService{activeItems: []application.ActiveFollowingCandidate{{
		ParticipantID: "user-1", Username: "alice", DisplayName: "Alice", ProfilePictureURL: "https://images.example/alice.webp",
		Timers: []application.ActiveFollowingTimer{{ID: "timer-1", PathID: "path-1", PathName: "Piano", StartedAt: startedAt}},
	}}, cursor: "next-signed"}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed/active?cursor=previous-signed&limit=20", nil)
	request.Header.Set("Authorization", "Bearer application-session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.authorization != "Bearer application-session" || service.receivedCursor != "previous-signed" || service.limit != 20 {
		t.Fatalf("service=%+v", service)
	}
	var body struct {
		Data struct {
			Items []struct {
				Participant struct {
					UserID, Username, DisplayName, ProfilePictureURL string
				} `json:"participant"`
				Timers []struct {
					ID        string
					StartedAt time.Time `json:"startedAt"`
					Path      struct{ ID, Name string }
				} `json:"timers"`
			} `json:"items"`
		} `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Items) != 1 || body.Data.Items[0].Participant.UserID != "user-1" || len(body.Data.Items[0].Timers) != 1 || body.Data.Items[0].Timers[0].ID != "timer-1" || body.Data.Items[0].Timers[0].Path.ID != "path-1" || body.Data.Items[0].Timers[0].StartedAt != startedAt || body.Meta.NextCursor != "next-signed" {
		t.Fatalf("body=%s", response.Body.String())
	}
	decoded := mustJSON(t, response.Body.Bytes())
	for _, forbidden := range []string{"email", "profileVisibility", "occurrenceTimeZone", "participantId"} {
		if containsJSONKey(decoded, forbidden) {
			t.Fatalf("active feed exposes forbidden %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestActiveFollowingContractIsAuthenticatedAndBounded(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	operation := api.OpenAPI().Paths["/v1/social/feed/active"].Get
	if operation == nil || len(operation.Security) == 0 || operation.OperationID != "list-active-following" {
		t.Fatalf("active following operation=%+v", operation)
	}
	for _, path := range []string{"/v1/social/feed/active?limit=0", "/v1/social/feed/active?limit=101"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler(&controlledService{}).ServeHTTP(response, request)
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("path=%s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func TestFollowMutationRequiresIdempotencyAndReturnsAuthoritativeState(t *testing.T) {
	service := &controlledService{relationship: application.RelationshipResult{
		Target:    domain.PublicProfile{ID: "target", Username: "alice", DisplayName: "Alice", FollowerCount: 4, FollowingCount: 2, Relationship: domain.RelationshipRequested},
		RequestID: "request-1",
	}}
	request := httptest.NewRequest(http.MethodPost, "/v1/profiles/alice/follow", nil)
	request.Header.Set("Authorization", "Bearer session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing idempotency status=%d body=%s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/v1/profiles/alice/follow", nil)
	request.Header.Set("Authorization", "Bearer session")
	request.Header.Set("Idempotency-Key", "follow-request-key")
	response = httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Profile struct {
				Relationship string `json:"relationship"`
			} `json:"profile"`
			RequestID string `json:"requestId"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Profile.Relationship != "requested" || body.Data.RequestID != "request-1" || service.username != "alice" {
		t.Fatalf("body=%s service=%+v", response.Body.String(), service)
	}
}

func TestIncomingFollowRequestsReturnCompactSafeRequesterProjection(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	service := &controlledService{requests: []domain.FollowRequest{{
		ID: "request-1", RecipientUserID: "viewer", CreatedAt: createdAt,
		Requester: domain.PublicProfile{ID: "sender", Username: "alice", DisplayName: "Alice", Relationship: domain.RelationshipNone},
	}}, cursor: "next"}
	request := httptest.NewRequest(http.MethodGet, "/v1/follow-requests?limit=10", nil)
	request.Header.Set("Authorization", "Bearer session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	encoded := response.Body.String()
	if service.limit != 10 || !containsJSONKey(mustJSON(t, response.Body.Bytes()), "requester") || containsJSONKey(mustJSON(t, response.Body.Bytes()), "recipientUserId") || !strings.Contains(encoded, `"nextCursor":"next"`) {
		t.Fatalf("unsafe or incomplete response=%s service=%+v", encoded, service)
	}
}

func mustJSON(t *testing.T, encoded []byte) any {
	t.Helper()
	var value any
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestFollowRelationshipContractRegistersDedicatedAuthenticatedEndpoints(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	operations := []*huma.Operation{
		api.OpenAPI().Paths["/v1/profiles/{username}/follow"].Post,
		api.OpenAPI().Paths["/v1/profiles/{username}/follow"].Delete,
		api.OpenAPI().Paths["/v1/profiles/{username}/follow-request"].Delete,
		api.OpenAPI().Paths["/v1/follow-requests"].Get,
		api.OpenAPI().Paths["/v1/follow-requests/{requestId}/accept"].Post,
		api.OpenAPI().Paths["/v1/follow-requests/{requestId}/reject"].Post,
	}
	for index, operation := range operations {
		if operation == nil || len(operation.Security) == 0 {
			t.Fatalf("follow operation %d is absent or unauthenticated", index)
		}
	}
}

func TestPracticeCommentListReturnsOldestFirstProjectionAndCursor(t *testing.T) {
	createdAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	comment := domain.Comment{ID: "comment-1", EventID: "practice:activity-1", AuthorID: "author-1", Text: "Great work!\n\U0001f389", Version: 2, CreatedAt: createdAt, UpdatedAt: createdAt.Add(time.Minute)}
	service := &controlledService{commentItems: []application.PracticeCommentItem{{
		Comment: comment, HeartCount: 4, HeartedByViewer: true,
		Author: domain.PublicProfile{ID: "author-1", Username: "alice", DisplayName: "Alice", ProfilePictureURL: "https://images.example/alice.webp", Relationship: domain.RelationshipFollowing},
	}}, cursor: "next-comment-cursor"}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed/practice:activity-1/comments?cursor=prior&limit=20", nil)
	request.Header.Set("Authorization", "Bearer session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.commentCall != "list" || service.authorization != "Bearer session" || service.eventID != comment.EventID || service.receivedCursor != "prior" || service.limit != 20 {
		t.Fatalf("service=%+v", service)
	}
	var body struct {
		Data struct {
			Items []struct {
				Comment         dto.PracticeComment `json:"comment"`
				Author          dto.PublicProfile   `json:"author"`
				HeartCount      int64               `json:"heartCount"`
				HeartedByViewer bool                `json:"heartedByViewer"`
			} `json:"items"`
		} `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Items) != 1 || body.Data.Items[0].Comment.ID != comment.ID || body.Data.Items[0].Comment.Text != comment.Text || !body.Data.Items[0].Comment.Edited || body.Data.Items[0].Author.ID != comment.AuthorID || body.Data.Items[0].HeartCount != 4 || !body.Data.Items[0].HeartedByViewer || body.Meta.NextCursor != "next-comment-cursor" {
		t.Fatalf("body=%s", response.Body.String())
	}
	decoded := mustJSON(t, response.Body.Bytes())
	for _, forbidden := range []string{"email", "status", "profileVisibility", "provider"} {
		if containsJSONKey(decoded, forbidden) {
			t.Fatalf("comment response exposes %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestPracticeCommentHeartMutationsAndSafeRoster(t *testing.T) {
	for _, test := range []struct {
		method, call string
		hearted      bool
	}{{http.MethodPut, "heart", true}, {http.MethodDelete, "unheart", false}} {
		service := &controlledService{heartSummary: application.CommentHeartSummary{CommentID: "comment-1", HeartCount: 2, HeartedByViewer: test.hearted}}
		request := httptest.NewRequest(test.method, "/v1/social/feed/practice:activity-1/comments/comment-1/heart", nil)
		request.Header.Set("Authorization", "Bearer session")
		request.Header.Set("Idempotency-Key", "comment-heart-key")
		response := httptest.NewRecorder()
		handler(service).ServeHTTP(response, request)
		if response.Code != http.StatusOK || service.commentCall != test.call || service.commentKey != "comment-heart-key" || !strings.Contains(response.Body.String(), `"heartCount":2`) {
			t.Fatalf("method=%s status=%d body=%s service=%+v", test.method, response.Code, response.Body.String(), service)
		}
	}
	service := &controlledService{heartProfiles: []domain.PublicProfile{{ID: "user-1", Username: "alice", DisplayName: "Alice", Relationship: domain.RelationshipFollowing}}, cursor: "next-hearts"}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed/practice:activity-1/comments/comment-1/hearts?cursor=prior&limit=10", nil)
	request.Header.Set("Authorization", "Bearer session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.commentCall != "hearts" || service.receivedCursor != "prior" || service.limit != 10 || !strings.Contains(response.Body.String(), `"username":"alice"`) || containsJSONKey(mustJSON(t, response.Body.Bytes()), "email") {
		t.Fatalf("status=%d body=%s service=%+v", response.Code, response.Body.String(), service)
	}
}

func TestPracticeCommentCreateEditAndDeleteUseIdempotencyAndCAS(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, method, path, body, key, call string
		status                              int
		version                             int64
	}{
		{name: "create", method: http.MethodPost, path: "/v1/social/feed/practice:activity-1/comments", body: `{"text":"Caf\u0065\u0301!"}`, key: "comment-create-key", call: "create", status: http.StatusCreated},
		{name: "edit", method: http.MethodPatch, path: "/v1/social/feed/practice:activity-1/comments/comment-1", body: `{"text":"Edited","expectedVersion":3}`, key: "comment-edit-key-1", call: "edit", status: http.StatusOK, version: 3},
		{name: "delete", method: http.MethodDelete, path: "/v1/social/feed/practice:activity-1/comments/comment-1", key: "comment-delete-key", call: "delete", status: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &controlledService{comment: domain.Comment{ID: "comment-1", EventID: "practice:activity-1", AuthorID: "viewer", Text: "Edited", Version: 4, CreatedAt: now.Add(-time.Hour), UpdatedAt: now}, commentDelete: application.CommentDeleteResult{CommentID: "comment-1", Deleted: true}}
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer session")
			request.Header.Set("Idempotency-Key", test.key)
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler(service).ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if service.commentCall != test.call || service.eventID != "practice:activity-1" || service.commentKey != test.key || service.expectedVersion != test.version || (test.name != "create" && service.commentID != "comment-1") {
				t.Fatalf("service=%+v", service)
			}
			if test.name == "create" && service.commentText != "Cafe\u0301!" {
				t.Fatalf("create text=%q", service.commentText)
			}
			if test.name == "delete" && response.Body.Len() != 0 {
				t.Fatalf("delete body=%s", response.Body.String())
			}
		})
	}
}
