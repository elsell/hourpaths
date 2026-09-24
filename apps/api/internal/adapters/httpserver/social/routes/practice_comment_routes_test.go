package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/dto"
	"github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPracticeCommentHistoryReturnsImmutableVersionsAndCursor(t *testing.T) {
	createdAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	service := &controlledService{commentVersions: []social.CommentVersion{{CommentID: "comment-1", Text: "Original", Version: 1, CreatedAt: createdAt}}, cursor: "next-history"}
	request := httptest.NewRequest(http.MethodGet, "/v1/social/feed/practice:activity-1/comments/comment-1/history?cursor=prior&limit=10", nil)
	request.Header.Set("Authorization", "Bearer session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.commentCall != "history" || service.commentID != "comment-1" || service.receivedCursor != "prior" || service.limit != 10 {
		t.Fatalf("status=%d body=%s service=%+v", response.Code, response.Body.String(), service)
	}
	var body struct {
		Data struct {
			Versions []dto.CommentVersion `json:"versions"`
		} `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data.Versions) != 1 || body.Data.Versions[0].Version != 1 || body.Data.Versions[0].Text != "Original" || body.Meta.NextCursor != "next-history" {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestPracticeCommentRoutesConcealDeniedResourcesAndMapConflicts(t *testing.T) {
	for _, test := range []struct {
		name, method, path, body, code string
		err                            error
		want                           int
	}{
		{name: "hidden", method: http.MethodGet, path: "/v1/social/feed/practice:hidden/comments", err: ports.ErrNotFound, want: http.StatusNotFound, code: "not_found"},
		{name: "stale edit", method: http.MethodPatch, path: "/v1/social/feed/practice:activity/comments/comment", body: `{"text":"Edit","expectedVersion":1}`, err: ports.ErrConflict, want: http.StatusConflict, code: "conflict"},
		{name: "idempotency conflict", method: http.MethodPost, path: "/v1/social/feed/practice:activity/comments", body: `{"text":"Comment"}`, err: ports.ErrIdempotencyConflict, want: http.StatusConflict, code: "idempotency_conflict"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer session")
			if test.method != http.MethodGet {
				request.Header.Set("Idempotency-Key", "comment-error-key")
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler(&controlledService{err: test.err}).ServeHTTP(response, request)
			if response.Code != test.want || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) || strings.Contains(response.Body.String(), "practice:hidden") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
