package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
)

func TestPracticeCommentMutationsRequireIdempotencyAndPositiveExpectedVersion(t *testing.T) {
	for _, test := range []struct{ name, method, path, body string }{
		{name: "create missing key", method: http.MethodPost, path: "/v1/social/feed/practice:activity/comments", body: `{"text":"Comment"}`},
		{name: "edit missing key", method: http.MethodPatch, path: "/v1/social/feed/practice:activity/comments/comment", body: `{"text":"Edit","expectedVersion":1}`},
		{name: "delete missing key", method: http.MethodDelete, path: "/v1/social/feed/practice:activity/comments/comment"},
		{name: "edit zero version", method: http.MethodPatch, path: "/v1/social/feed/practice:activity/comments/comment", body: `{"text":"Edit","expectedVersion":0}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &controlledService{}
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			request.Header.Set("Authorization", "Bearer session")
			if test.name == "edit zero version" {
				request.Header.Set("Idempotency-Key", "comment-edit-key-2")
			}
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			response := httptest.NewRecorder()
			handler(service).ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity || service.commentCall != "" {
				t.Fatalf("status=%d body=%s service=%+v", response.Code, response.Body.String(), service)
			}
		})
	}
}

func TestPracticeCommentContractRegistersDedicatedAuthenticatedRoutes(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	paths := api.OpenAPI().Paths
	operations := []*huma.Operation{
		paths["/v1/social/feed/{eventId}/comments"].Get,
		paths["/v1/social/feed/{eventId}/comments"].Post,
		paths["/v1/social/feed/{eventId}/comments/{commentId}"].Patch,
		paths["/v1/social/feed/{eventId}/comments/{commentId}"].Delete,
		paths["/v1/social/feed/{eventId}/comments/{commentId}/history"].Get,
		paths["/v1/social/feed/{eventId}/comments/{commentId}/heart"].Put,
		paths["/v1/social/feed/{eventId}/comments/{commentId}/heart"].Delete,
		paths["/v1/social/feed/{eventId}/comments/{commentId}/hearts"].Get,
	}
	for index, operation := range operations {
		if operation == nil || len(operation.Security) == 0 {
			t.Fatalf("comment operation %d is absent or unauthenticated", index)
		}
	}
	if got := paths["/v1/social/feed/{eventId}/comments"].Post.DefaultStatus; got != http.StatusCreated {
		t.Fatalf("create comment OpenAPI status=%d want=%d", got, http.StatusCreated)
	}
}
