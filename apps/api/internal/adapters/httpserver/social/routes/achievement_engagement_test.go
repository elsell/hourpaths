package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestAchievementEngagementUsesExistingReactionCommentAndHeartContracts(t *testing.T) {
	const eventID = "achievement:goal-1"
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name, method, path, body, call string
		service                        *controlledService
		want                           int
	}{
		{
			name: "reaction", method: http.MethodPut, path: "/v1/social/feed/achievement%3Agoal-1/reaction", body: `{"reaction":"celebrate"}`, want: http.StatusOK,
			service: &controlledService{reactionSummary: domain.ReactionSummary{Counts: domain.ReactionCounts{Celebrate: 1}, ViewerReaction: domain.ReactionCelebrate}},
		},
		{
			name: "comment", method: http.MethodPost, path: "/v1/social/feed/achievement%3Agoal-1/comments", body: `{"text":"Congratulations"}`, call: "create", want: http.StatusCreated,
			service: &controlledService{comment: domain.Comment{ID: "comment-1", EventID: eventID, AuthorID: "viewer", Text: "Congratulations", Version: 1, CreatedAt: now, UpdatedAt: now}},
		},
		{
			name: "heart", method: http.MethodPut, path: "/v1/social/feed/achievement%3Agoal-1/comments/comment-1/heart", call: "heart", want: http.StatusOK,
			service: &controlledService{heartSummary: application.CommentHeartSummary{CommentID: "comment-1", HeartCount: 1, HeartedByViewer: true}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer session")
			request.Header.Set("Idempotency-Key", "achievement-engagement-key")
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler(test.service).ServeHTTP(response, request)
			if response.Code != test.want || test.service.eventID != eventID || (test.call != "" && test.service.commentCall != test.call) {
				t.Fatalf("status=%d body=%s service=%+v", response.Code, response.Body.String(), test.service)
			}
		})
	}
}

func TestAchievementEngagementAuthorizationFailuresRemainOpaque(t *testing.T) {
	for _, request := range []*http.Request{
		httptest.NewRequest(http.MethodPut, "/v1/social/feed/achievement%3Ahidden/reaction", strings.NewReader(`{"reaction":"heart"}`)),
		httptest.NewRequest(http.MethodPost, "/v1/social/feed/achievement%3Ahidden/comments", strings.NewReader(`{"text":"Hidden"}`)),
		httptest.NewRequest(http.MethodPut, "/v1/social/feed/achievement%3Ahidden/comments/comment-1/heart", nil),
	} {
		request.Header.Set("Authorization", "Bearer session")
		request.Header.Set("Idempotency-Key", "achievement-hidden-key")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler(&controlledService{err: fmt.Errorf("private achievement detail: %w", ports.ErrNotFound)}).ServeHTTP(response, request)
		if response.Code != http.StatusNotFound || !strings.Contains(response.Body.String(), `"code":"not_found"`) || strings.Contains(response.Body.String(), "achievement detail") {
			t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
		}
	}
}
