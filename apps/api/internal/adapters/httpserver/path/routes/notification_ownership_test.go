package routes

import (
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNotificationOwnershipCheckDuringPolicyReviewExposesNoContent(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
	}{
		{"own", nil, 200}, {"foreign", ports.ErrNotFound, 404}, {"missing", ports.ErrNotFound, 404},
		{"expired", ports.ErrInvalidCredential, 401}, {"dependency", ports.ErrUnavailable, 503},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			service := controlledInvitationHTTPService{notifications: []pathapp.InvitationNotificationProjection{{ID: "notice", PathName: "private-path-name", PathID: "private-path-id", Actor: pathapp.InvitationPublicIdentity{Username: "private-user"}}}, err: item.err}
			handler, _ := shared.New(app.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { RegisterInvitations(api, service) }}})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, invitationRequest(http.MethodGet, "/v1/me/notifications/notice/ownership", "", ""))
			if response.Code != item.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
			if item.err == nil {
				ordinary := httptest.NewRecorder()
				handler.ServeHTTP(ordinary, invitationRequest(http.MethodGet, "/v1/notifications/notice", "", ""))
				if ordinary.Code != http.StatusServiceUnavailable {
					t.Fatalf("ordinary read bypassed admission: %d", ordinary.Code)
				}
				var body map[string]json.RawMessage
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				var data map[string]any
				if err := json.Unmarshal(body["data"], &data); err != nil {
					t.Fatal(err)
				}
				if len(data) != 1 || data["owned"] != true {
					t.Fatalf("ownership leaked content: %s", response.Body)
				}
			}
		})
	}
}
