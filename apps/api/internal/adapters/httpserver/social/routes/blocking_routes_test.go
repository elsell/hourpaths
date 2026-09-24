package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
)

func TestBlockingRoutesUseSafeReviewMutationAndPagedListEnvelopes(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	target := domain.BlockTarget{UserID: "target", Username: "alice", DisplayName: "Alice"}
	service := &controlledService{
		blockReview:     application.BlockReview{Target: target, SharedPaths: []domain.SharedPath{{ID: "path-1", Name: "Piano"}}, Acknowledgement: application.BlockReviewAcknowledgement{Version: 1, Token: "signed-review-token", ExpiresAt: now.Add(time.Minute)}},
		blockResult:     application.BlockMutationResult{Target: target, Blocked: true},
		blockedAccounts: []domain.BlockedAccount{{Target: target, BlockedAt: now}}, cursor: "next-block",
	}
	for _, test := range []struct {
		method, path string
		key          bool
		want         string
	}{
		{http.MethodGet, "/v1/profiles/alice/block-review", false, `"sharedPaths":[{"id":"path-1","name":"Piano"}]`},
		{http.MethodPost, "/v1/profiles/alice/block", true, `"blocked":true`},
		{http.MethodGet, "/v1/blocked-accounts?limit=10", false, `"nextCursor":"next-block"`},
		{http.MethodDelete, "/v1/blocked-accounts/target", true, `"blocked":false`},
	} {
		if test.method == http.MethodDelete {
			service.blockResult.Blocked = false
		}
		body := strings.NewReader("")
		if test.method == http.MethodPost {
			body = strings.NewReader(`{"acknowledgement":{"version":1,"token":"signed-review-token","expiresAt":"2026-07-29T12:01:00Z"}}`)
		}
		request := httptest.NewRequest(test.method, test.path, body)
		if test.method == http.MethodPost {
			request.Header.Set("Content-Type", "application/json")
		}
		request.Header.Set("Authorization", "Bearer session")
		if test.key {
			request.Header.Set("Idempotency-Key", "blocking-request-key")
		}
		response := httptest.NewRecorder()
		handler(service).ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), test.want) {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
	}
	if service.blockAcknowledgement.Version != 1 || service.blockAcknowledgement.Token != "signed-review-token" {
		t.Fatalf("block acknowledgement=%+v", service.blockAcknowledgement)
	}
}

func TestBlockingContractRegistersAuthenticatedIdempotentMutations(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, &controlledService{}) }}})
	paths := api.OpenAPI().Paths
	operations := []*huma.Operation{paths["/v1/profiles/{username}/block-review"].Get, paths["/v1/profiles/{username}/block"].Post, paths["/v1/blocked-accounts"].Get, paths["/v1/blocked-accounts/{userId}"].Delete}
	for index, operation := range operations {
		if operation == nil || len(operation.Security) == 0 {
			t.Fatalf("blocking operation %d absent or unauthenticated", index)
		}
	}
	for _, operation := range []*huma.Operation{operations[1], operations[3]} {
		found := false
		for _, parameter := range operation.Parameters {
			if parameter.Name == "Idempotency-Key" && parameter.In == "header" && parameter.Required {
				found = true
			}
		}
		if !found {
			t.Fatalf("mutation %s lacks required idempotency header", operation.OperationID)
		}
	}
}

func TestBlockRouteRejectsSkippedReviewAcknowledgementBeforeService(t *testing.T) {
	service := &controlledService{}
	request := httptest.NewRequest(http.MethodPost, "/v1/profiles/alice/block", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer session")
	request.Header.Set("Idempotency-Key", "blocking-request-key")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code < 400 || service.blockKey != "" {
		t.Fatalf("status=%d service key=%q body=%s", response.Code, service.blockKey, response.Body.String())
	}
}
