package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPathInvitationRecipientReviewReturnsOnlyAlwaysPublicIdentity(t *testing.T) {
	call := &invitationHTTPCall{}
	handler, _ := invitationHandler(controlledInvitationHTTPService{
		call: call,
		recipient: pathapp.InvitationRecipientReview{
			UserID: "recipient-id", Username: "Reader.One", DisplayName: "Reader One",
		},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(
		http.MethodGet,
		"/v1/paths/path-1/invitation-recipient?username=reader.one",
		"", "",
	))
	if response.Code != http.StatusOK ||
		*call != (invitationHTTPCall{
			authorization: "Bearer application-session", pathID: "path-1", username: "reader.one",
		}) {
		t.Fatalf("review status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Data) != 3 || body.Data["userId"] != "recipient-id" ||
		body.Data["username"] != "Reader.One" || body.Data["displayName"] != "Reader One" {
		t.Fatalf("recipient review body exposed unexpected data: %s", response.Body.String())
	}
	for _, forbidden := range []string{"email", "profileVisibility", "provider"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("recipient review exposed %q: %s", forbidden, response.Body.String())
		}
	}
}

func TestPathInvitationRecipientReviewValidatesUsernameBeforeService(t *testing.T) {
	for _, target := range []string{
		"/v1/paths/path-1/invitation-recipient",
		"/v1/paths/path-1/invitation-recipient?username=ab",
		"/v1/paths/path-1/invitation-recipient?username=reader%40example.com",
	} {
		call := &invitationHTTPCall{}
		handler, _ := invitationHandler(controlledInvitationHTTPService{call: call})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, invitationRequest(http.MethodGet, target, "", ""))
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s status=%d body=%s", target, response.Code, response.Body.String())
		}
		if *call != (invitationHTTPCall{}) {
			t.Fatalf("%s reached service: %+v", target, call)
		}
	}
}

func TestPathInvitationRecipientReviewConcealsAuthorizationAndMissingIdentity(t *testing.T) {
	for _, inaccessible := range []error{platformapp.ErrForbidden, ports.ErrNotFound} {
		handler, _ := invitationHandler(controlledInvitationHTTPService{err: inaccessible})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, invitationRequest(
			http.MethodGet,
			"/v1/paths/secret/invitation-recipient?username=reader",
			"", "",
		))
		if response.Code != http.StatusNotFound ||
			!strings.Contains(response.Body.String(), `"code":"not_found"`) ||
			strings.Contains(response.Body.String(), "reader") {
			t.Fatalf("concealed review status=%d body=%s", response.Code, response.Body.String())
		}
	}
}

func TestPathInvitationRecipientReviewOpenAPIIsAuthenticatedAndEmailFree(t *testing.T) {
	_, api := invitationHandler(controlledInvitationHTTPService{})
	operation := api.OpenAPI().Paths["/v1/paths/{pathId}/invitation-recipient"].Get
	if operation == nil || operation.OperationID != "review-path-invitation-recipient" ||
		len(operation.Security) != 1 {
		t.Fatalf("recipient review operation = %+v", operation)
	}
	foundUsername := false
	for _, parameter := range operation.Parameters {
		if parameter.Name == "username" && parameter.Required &&
			parameter.Schema.MinLength != nil && *parameter.Schema.MinLength == 3 &&
			parameter.Schema.MaxLength != nil && *parameter.Schema.MaxLength == 64 {
			foundUsername = true
		}
	}
	if !foundUsername {
		t.Fatal("recipient review lacks strict required username query")
	}
	schema := api.OpenAPI().Components.Schemas.Map()["PathInvitationRecipient"]
	if schema == nil || len(schema.Properties) != 3 ||
		schema.Properties["userId"] == nil || schema.Properties["username"] == nil ||
		schema.Properties["displayName"] == nil || schema.Properties["email"] != nil {
		t.Fatalf("recipient schema exposes unexpected fields: %+v", schema)
	}
}
