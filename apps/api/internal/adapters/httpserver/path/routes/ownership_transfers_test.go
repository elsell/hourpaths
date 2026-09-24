package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

type ownershipTransferHTTPCall struct {
	operation, authorization, idempotencyKey, recipientUserID string
	pathID                                                    domain.ID
	transferID                                                domain.OwnershipTransferID
	confirmed                                                 bool
	cursor                                                    string
	limit                                                     int
}

type controlledOwnershipTransferHTTPService struct {
	call       *ownershipTransferHTTPCall
	candidates []pathapp.OwnershipTransferCandidate
	decision   pathapp.OwnershipTransferDecision
	result     pathapp.OwnershipTransferResult
	review     pathapp.OwnershipTransferReview
	err        error
	nextCursor string
}

func (service controlledOwnershipTransferHTTPService) Review(_ context.Context, authorization string, pathID domain.ID, recipientUserID string) (pathapp.OwnershipTransferReview, error) {
	if service.call != nil {
		*service.call = ownershipTransferHTTPCall{operation: "review", authorization: authorization, pathID: pathID, recipientUserID: recipientUserID}
	}
	return service.review, service.err
}

func (service controlledOwnershipTransferHTTPService) ListCandidates(_ context.Context, authorization string, pathID domain.ID, cursor string, limit int) ([]pathapp.OwnershipTransferCandidate, string, error) {
	if service.call != nil {
		*service.call = ownershipTransferHTTPCall{operation: "list", authorization: authorization, pathID: pathID, cursor: cursor, limit: limit}
	}
	return service.candidates, service.nextCursor, service.err
}
func (service controlledOwnershipTransferHTTPService) GetPending(_ context.Context, authorization string, pathID domain.ID) (pathapp.OwnershipTransferDecision, error) {
	if service.call != nil {
		*service.call = ownershipTransferHTTPCall{operation: "get", authorization: authorization, pathID: pathID}
	}
	return service.decision, service.err
}
func (service controlledOwnershipTransferHTTPService) Initiate(_ context.Context, authorization string, pathID domain.ID, recipientUserID, idempotencyKey string, confirmed bool) (pathapp.OwnershipTransferResult, error) {
	if service.call != nil {
		*service.call = ownershipTransferHTTPCall{operation: "initiate", authorization: authorization, pathID: pathID, recipientUserID: recipientUserID, idempotencyKey: idempotencyKey, confirmed: confirmed}
	}
	return service.result, service.err
}
func (service controlledOwnershipTransferHTTPService) Accept(_ context.Context, authorization string, transferID domain.OwnershipTransferID, idempotencyKey string) (pathapp.OwnershipTransferResult, error) {
	if service.call != nil {
		*service.call = ownershipTransferHTTPCall{operation: "accept", authorization: authorization, transferID: transferID, idempotencyKey: idempotencyKey}
	}
	return service.result, service.err
}
func (service controlledOwnershipTransferHTTPService) Decline(_ context.Context, authorization string, transferID domain.OwnershipTransferID, idempotencyKey string) (pathapp.OwnershipTransferResult, error) {
	if service.call != nil {
		*service.call = ownershipTransferHTTPCall{operation: "decline", authorization: authorization, transferID: transferID, idempotencyKey: idempotencyKey}
	}
	return service.result, service.err
}
func (service controlledOwnershipTransferHTTPService) Cancel(_ context.Context, authorization string, transferID domain.OwnershipTransferID, idempotencyKey string) (pathapp.OwnershipTransferResult, error) {
	if service.call != nil {
		*service.call = ownershipTransferHTTPCall{operation: "cancel", authorization: authorization, transferID: transferID, idempotencyKey: idempotencyKey}
	}
	return service.result, service.err
}

func ownershipTransferHandler(service OwnershipTransferService) (http.Handler, huma.API) {
	return shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { RegisterOwnershipTransfers(api, service) }}})
}

func ownershipTransferRequest(method, target, body, key string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer application-session")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	return request
}

func ownershipTransferFixture(state string) pathapp.OwnershipTransferResult {
	createdAt := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	transfer, _ := domain.NewOwnershipTransfer("transfer-1", "path-1", "creator-1", "recipient-1", createdAt, createdAt.Add(7*24*time.Hour))
	switch state {
	case "accepted":
		transfer.AcceptedAt = createdAt.Add(time.Hour)
	case "declined":
		transfer.DeclinedAt = createdAt.Add(time.Hour)
	case "canceled":
		transfer.CanceledAt = createdAt.Add(time.Hour)
	}
	return pathapp.OwnershipTransferResult{Transfer: transfer}
}

func TestOwnershipTransferRoutesExposeUniqueActorScopedContracts(t *testing.T) {
	_, api := ownershipTransferHandler(controlledOwnershipTransferHTTPService{})
	want := map[string]struct{ method, operation string }{
		"/v1/paths/{pathId}/ownership-transfer-candidates":  {http.MethodGet, "list-path-ownership-transfer-candidates"},
		"/v1/paths/{pathId}/ownership-transfer/review":      {http.MethodPost, "review-path-ownership-transfer"},
		"/v1/paths/{pathId}/ownership-transfers":            {http.MethodPost, "initiate-path-ownership-transfer"},
		"/v1/paths/{pathId}/ownership-transfer":             {http.MethodGet, "get-pending-path-ownership-transfer"},
		"/v1/path-ownership-transfers/{transferId}/accept":  {http.MethodPost, "accept-path-ownership-transfer"},
		"/v1/path-ownership-transfers/{transferId}/decline": {http.MethodPost, "decline-path-ownership-transfer"},
		"/v1/path-ownership-transfers/{transferId}/cancel":  {http.MethodPost, "cancel-path-ownership-transfer"},
	}
	seen := map[string]bool{}
	for path, expected := range want {
		item := api.OpenAPI().Paths[path]
		if item == nil {
			t.Fatalf("missing OpenAPI path %s", path)
		}
		var operation *huma.Operation
		switch expected.method {
		case http.MethodGet:
			operation = item.Get
		case http.MethodPost:
			operation = item.Post
		}
		if operation == nil || operation.OperationID != expected.operation {
			t.Fatalf("%s operation = %+v", path, operation)
		}
		if seen[operation.OperationID] {
			t.Fatalf("duplicate operation id %q", operation.OperationID)
		}
		seen[operation.OperationID] = true
	}
}

func TestOwnershipTransferCandidateAndPendingReadsPreserveActorScope(t *testing.T) {
	call := &ownershipTransferHTTPCall{}
	fixture := ownershipTransferFixture("pending")
	handler, _ := ownershipTransferHandler(controlledOwnershipTransferHTTPService{
		call:       call,
		candidates: []pathapp.OwnershipTransferCandidate{{UserID: "recipient-1", Username: "Reader.One", DisplayName: "Reader One", Administrator: true}},
		nextCursor: "signed-next-page",
		decision:   pathapp.OwnershipTransferDecision{Transfer: fixture.Transfer},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, ownershipTransferRequest(http.MethodGet, "/v1/paths/path-1/ownership-transfer-candidates?cursor=signed-current-page&limit=100", "", ""))
	if response.Code != http.StatusOK || *call != (ownershipTransferHTTPCall{operation: "list", authorization: "Bearer application-session", pathID: "path-1", cursor: "signed-current-page", limit: 100}) || !strings.Contains(response.Body.String(), `"username":"Reader.One"`) || !strings.Contains(response.Body.String(), `"administrator":true`) || !strings.Contains(response.Body.String(), `"nextCursor":"signed-next-page"`) {
		t.Fatalf("candidate response=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, ownershipTransferRequest(http.MethodGet, "/v1/paths/path-1/ownership-transfer", "", ""))
	if response.Code != http.StatusOK || *call != (ownershipTransferHTTPCall{operation: "get", authorization: "Bearer application-session", pathID: "path-1"}) || !strings.Contains(response.Body.String(), `"state":"pending"`) {
		t.Fatalf("pending response=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
}

func TestOwnershipTransferInitiationRequiresExplicitConfirmationAndIdempotency(t *testing.T) {
	call := &ownershipTransferHTTPCall{}
	handler, _ := ownershipTransferHandler(controlledOwnershipTransferHTTPService{call: call, result: ownershipTransferFixture("pending")})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, ownershipTransferRequest(http.MethodPost, "/v1/paths/path-1/ownership-transfers", `{"reservationToken":"signed-review"}`, "initiate-transfer-key"))
	if response.Code != http.StatusCreated || *call != (ownershipTransferHTTPCall{operation: "initiate", authorization: "Bearer application-session", pathID: "path-1", recipientUserID: "signed-review", idempotencyKey: "initiate-transfer-key", confirmed: true}) {
		t.Fatalf("response=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
	call = &ownershipTransferHTTPCall{}
	handler, _ = ownershipTransferHandler(controlledOwnershipTransferHTTPService{call: call})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, ownershipTransferRequest(http.MethodPost, "/v1/paths/path-1/ownership-transfers", `{}`, "initiate-transfer-key"))
	if response.Code != http.StatusUnprocessableEntity || *call != (ownershipTransferHTTPCall{}) {
		t.Fatalf("unconfirmed response=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
	for _, key := range []string{"", "too-short"} {
		call = &ownershipTransferHTTPCall{}
		handler, _ = ownershipTransferHandler(controlledOwnershipTransferHTTPService{call: call})
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, ownershipTransferRequest(http.MethodPost, "/v1/paths/path-1/ownership-transfers", `{"reservationToken":"signed-review"}`, key))
		if response.Code != http.StatusUnprocessableEntity || *call != (ownershipTransferHTTPCall{}) {
			t.Fatalf("key=%q response=%d call=%+v body=%s", key, response.Code, call, response.Body.String())
		}
	}
	call = &ownershipTransferHTTPCall{}
	handler, _ = ownershipTransferHandler(controlledOwnershipTransferHTTPService{call: call})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, ownershipTransferRequest(http.MethodPost, "/v1/paths/path-1/ownership-transfers", `{"reservationToken":"`+strings.Repeat("x", 4097)+`"}`, "initiate-transfer-key"))
	if response.Code != http.StatusUnprocessableEntity || *call != (ownershipTransferHTTPCall{}) {
		t.Fatalf("oversized token response=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
}

func TestOwnershipTransferRecipientAndCreatorMutationsReturnTerminalLifecycle(t *testing.T) {
	for _, test := range []struct{ action, state string }{{"accept", "accepted"}, {"decline", "declined"}, {"cancel", "canceled"}} {
		t.Run(test.action, func(t *testing.T) {
			call := &ownershipTransferHTTPCall{}
			fixture := ownershipTransferFixture(test.state)
			if test.action == "accept" {
				fixture.Path = domain.Entity{ID: "path-1", OwnerUserID: "recipient-1", Attributes: domain.Attributes{Name: "Violin", Visibility: "private"}}
			}
			handler, _ := ownershipTransferHandler(controlledOwnershipTransferHTTPService{call: call, result: fixture})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, ownershipTransferRequest(http.MethodPost, "/v1/path-ownership-transfers/transfer-1/"+test.action, "", "mutation-transfer-key"))
			var body struct {
				Data struct {
					Transfer struct {
						State string `json:"state"`
					} `json:"transfer"`
					Path *struct {
						ID string `json:"id"`
					} `json:"path"`
				} `json:"data"`
			}
			err := json.Unmarshal(response.Body.Bytes(), &body)
			if response.Code != http.StatusOK || err != nil || body.Data.Transfer.State != test.state || call.operation != test.action || call.transferID != "transfer-1" || call.idempotencyKey != "mutation-transfer-key" {
				t.Fatalf("response=%d call=%+v body=%s err=%v", response.Code, call, response.Body.String(), err)
			}
			if test.action == "accept" && (body.Data.Path == nil || body.Data.Path.ID != "path-1") {
				t.Fatalf("accept omits resulting path: %s", response.Body.String())
			}
			if test.action != "accept" && body.Data.Path != nil {
				t.Fatalf("%s invents resulting path: %s", test.action, response.Body.String())
			}
		})
	}
}

func TestOwnershipTransferUnavailableIsOpaqueForEveryReadAndMutation(t *testing.T) {
	for _, test := range []struct{ method, target string }{
		{http.MethodGet, "/v1/paths/hidden/ownership-transfer-candidates"},
		{http.MethodGet, "/v1/paths/hidden/ownership-transfer"},
		{http.MethodPost, "/v1/paths/hidden/ownership-transfers"},
		{http.MethodPost, "/v1/path-ownership-transfers/hidden/accept"},
		{http.MethodPost, "/v1/path-ownership-transfers/hidden/decline"},
		{http.MethodPost, "/v1/path-ownership-transfers/hidden/cancel"},
	} {
		handler, _ := ownershipTransferHandler(controlledOwnershipTransferHTTPService{err: domain.ErrOwnershipTransferUnavailable})
		body := ""
		if test.target == "/v1/paths/hidden/ownership-transfers" {
			body = `{"reservationToken":"signed-review"}`
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, ownershipTransferRequest(test.method, test.target, body, "opaque-transfer-key"))
		if response.Code != http.StatusNotFound || strings.Contains(response.Body.String(), "ownership") || strings.Contains(response.Body.String(), "forbidden") {
			t.Fatalf("target=%s status=%d body=%s", test.target, response.Code, response.Body.String())
		}
	}
}
