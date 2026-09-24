package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestRenamePathContractUsesReviewedNameAndReturnsAuthoritativeProjection(t *testing.T) {
	call := &struct {
		authorization, idempotencyKey, expectedName, name string
		id                                                domain.ID
	}{}
	renamed := domain.Entity{
		ID: "path-1", OwnerUserID: "creator",
		Attributes: domain.Attributes{Name: "After", Visibility: "private"},
	}
	service := controlledService{
		renameCall: call, renameResult: pathapp.RenameResult{Path: renamed},
		capabilities: pathapp.Capabilities{RenamePath: true},
	}
	request := httptest.NewRequest(http.MethodPut, "/v1/paths/path-1/name", strings.NewReader(
		`{"expectedName":"Before","name":"After"}`,
	))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer valid-application-session")
	request.Header.Set("Idempotency-Key", "rename-path-key-0001")
	response := httptest.NewRecorder()
	pathHandler(service).ServeHTTP(response, request)

	var body struct {
		Data struct {
			ID, Name     string
			Capabilities struct {
				RenamePath bool `json:"renamePath"`
			} `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil ||
		response.Code != http.StatusOK || body.Data.ID != "path-1" ||
		body.Data.Name != "After" || !body.Data.Capabilities.RenamePath {
		t.Fatalf("rename response status=%d body=%s", response.Code, response.Body.String())
	}
	if call.authorization != "Bearer valid-application-session" ||
		call.idempotencyKey != "rename-path-key-0001" || call.id != "path-1" ||
		call.expectedName != "Before" || call.name != "After" {
		t.Fatalf("rename call = %+v", call)
	}
}

func TestRenamePathContractRequiresCompleteReplaySafeInputAndConcealsDeniedPath(t *testing.T) {
	for _, test := range []struct {
		name, body, key string
		serviceError    error
		want            int
	}{
		{name: "missing expected name", body: `{"name":"After"}`, key: "rename-path-key-0001", want: http.StatusUnprocessableEntity},
		{name: "missing name", body: `{"expectedName":"Before"}`, key: "rename-path-key-0001", want: http.StatusUnprocessableEntity},
		{name: "short replay key", body: `{"expectedName":"Before","name":"After"}`, key: "short", want: http.StatusUnprocessableEntity},
		{name: "denied", body: `{"expectedName":"Before","name":"After"}`, key: "rename-path-key-0001", serviceError: platformapp.ErrForbidden, want: http.StatusNotFound},
		{name: "conflict", body: `{"expectedName":"Before","name":"After"}`, key: "rename-path-key-0001", serviceError: ports.ErrConflict, want: http.StatusConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/v1/paths/path-1/name", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer valid-application-session")
			request.Header.Set("Idempotency-Key", test.key)
			response := httptest.NewRecorder()
			pathHandler(controlledService{renameError: test.serviceError}).ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}
