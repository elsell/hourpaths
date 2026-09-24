package generated_test

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/generated"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type pathAuthenticator struct{}

var errPathAuthenticationDependency = errors.New("authentication dependency unavailable")

func (pathAuthenticator) Authenticate(_ context.Context, authorization string) (ports.Principal, error) {
	if authorization == "Bearer unavailable" {
		return ports.Principal{}, errPathAuthenticationDependency
	}
	if authorization != "Bearer valid" {
		return ports.Principal{}, ports.ErrInvalidCredential
	}
	return ports.Principal{UserID: "user", Scopes: []string{"api:user"}}, nil
}

func TestGeneratedDomainRoutes_path(t *testing.T) {
	handler, _ := httpserver.New(app.App{}, nil, httpserver.Options{
		DomainRegistrations: generated.Registrations(generated.Dependencies{Auth: pathAuthenticator{}}),
	})
	operations := []struct{ name, method, path, body string }{
		{"create", http.MethodPost, "/v1/paths", `{"name":"value","visibility":"private"}`},
		{"list", http.MethodGet, "/v1/paths", ""},
		{"get", http.MethodGet, "/v1/paths/id", ""},
		{"update", http.MethodPut, "/v1/paths/id", `{"name":"value","visibility":"value"}`},
		{"delete", http.MethodDelete, "/v1/paths/id", `{"confirmed":true,"expectedName":"value"}`},
	}
	credentials := []struct {
		name, authorization string
		status              int
	}{
		{"missing session", "", http.StatusUnauthorized},
		{"malformed session", "Bearer malformed", http.StatusUnauthorized},
		{"valid session without policy", "Bearer valid", http.StatusServiceUnavailable},
		{"authentication dependency unavailable", "Bearer unavailable", http.StatusInternalServerError},
	}
	for _, operation := range operations {
		for _, credential := range credentials {
			t.Run(operation.name+"/"+credential.name, func(t *testing.T) {
				request := httptest.NewRequest(operation.method, operation.path, strings.NewReader(operation.body))
				if operation.body != "" {
					request.Header.Set("Content-Type", "application/json")
				}
				if operation.method == http.MethodPost || operation.method == http.MethodDelete {
					request.Header.Set("Idempotency-Key", "0123456789abcdef")
				}
				if credential.authorization != "" {
					request.Header.Set("Authorization", credential.authorization)
				}
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				expectedStatus := credential.status
				if (operation.name == "create" || operation.name == "get" || operation.name == "list" || operation.name == "delete") && credential.name == "valid session without policy" {
					expectedStatus = http.StatusInternalServerError
				}
				if response.Code != expectedStatus {
					t.Fatalf("status=%d want=%d body=%s", response.Code, expectedStatus, response.Body.String())
				}
			})
		}
	}
}

func TestGeneratedPathRegistrationIncludesFailClosedInvitationRoutes(t *testing.T) {
	handler, api := httpserver.New(app.App{}, nil, httpserver.Options{
		DomainRegistrations: generated.Registrations(generated.Dependencies{Auth: pathAuthenticator{}}),
	})
	operations := []struct {
		name, method, openAPIPath, requestPath, operationID, body string
	}{
		{"review recipient", http.MethodGet, "/v1/paths/{pathId}/invitation-recipient", "/v1/paths/path-1/invitation-recipient?username=reader", "review-path-invitation-recipient", ""},
		{"send", http.MethodPost, "/v1/paths/{pathId}/invitations", "/v1/paths/path-1/invitations", "send-path-invitation", `{"username":"reader","expectedRecipientUserId":"recipient","offeredRole":"participant"}`},
		{"list", http.MethodGet, "/v1/path-invitations", "/v1/path-invitations", "list-pending-path-invitations", ""},
		{"accept", http.MethodPost, "/v1/path-invitations/{invitationId}/accept", "/v1/path-invitations/invitation-1/accept", "accept-path-invitation", ""},
	}
	for _, operation := range operations {
		t.Run(operation.name, func(t *testing.T) {
			item := api.OpenAPI().Paths[operation.openAPIPath]
			var registeredID string
			switch operation.method {
			case http.MethodGet:
				if item != nil && item.Get != nil {
					registeredID = item.Get.OperationID
				}
			case http.MethodPost:
				if item != nil && item.Post != nil {
					registeredID = item.Post.OperationID
				}
			}
			if registeredID != operation.operationID {
				t.Fatalf("%s %s operation = %q, want %q", operation.method, operation.openAPIPath, registeredID, operation.operationID)
			}

			request := httptest.NewRequest(operation.method, operation.requestPath, strings.NewReader(operation.body))
			request.Header.Set("Authorization", "Bearer valid")
			if operation.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			if operation.method == http.MethodPost {
				request.Header.Set("Idempotency-Key", "0123456789abcdef")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("missing typed dependencies status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}
