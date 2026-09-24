package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type archiveStateCall struct {
	authorization, idempotencyKey string
	pathID                        domain.ID
	confirmed, expected, archived bool
}

type archiveStateService struct {
	controlledService
	call   *archiveStateCall
	result pathapp.SetArchiveStateResult
	err    error
}

func (s archiveStateService) SetArchiveState(_ context.Context, authorization, idempotencyKey string, pathID domain.ID, confirmed, expectedArchived, archived bool) (pathapp.SetArchiveStateResult, error) {
	if authorization != "Bearer valid-application-session" {
		return pathapp.SetArchiveStateResult{}, ports.ErrInvalidCredential
	}
	if s.call != nil {
		*s.call = archiveStateCall{authorization, idempotencyKey, pathID, confirmed, expectedArchived, archived}
	}
	return s.result, s.err
}

func performArchiveStateUpdate(handler http.Handler, id, body, authorization, idempotencyKey string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPut, "/v1/paths/"+id+"/archive-state", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	if idempotencyKey != "" {
		request.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestPathArchiveStateCallsDedicatedServiceAndReturnsArchivedInstant(t *testing.T) {
	call := &archiveStateCall{}
	archivedAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	response := performArchiveStateUpdate(pathHandler(archiveStateService{
		controlledService: controlledService{capabilities: pathapp.Capabilities{ManageLifecycle: true}},
		call:              call,
		result: pathapp.SetArchiveStateResult{Path: domain.Entity{
			ID: "path-1", OwnerUserID: "creator", ArchivedAt: archivedAt,
			Attributes: domain.Attributes{Name: "Read", Visibility: "private"},
		}},
	}), "path-1", `{"confirmed":true,"expectedArchived":false,"archived":true}`, "Bearer valid-application-session", "path-archive-key-0001")

	var body struct {
		Data struct {
			ID           string               `json:"id"`
			ArchivedAt   *time.Time           `json:"archivedAt"`
			Capabilities pathapp.Capabilities `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || body.Data.ID != "path-1" || body.Data.ArchivedAt == nil || !body.Data.ArchivedAt.Equal(archivedAt) ||
		body.Data.Capabilities != (pathapp.Capabilities{ManageLifecycle: true}) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if *call != (archiveStateCall{"Bearer valid-application-session", "path-archive-key-0001", "path-1", true, false, true}) {
		t.Fatalf("SetArchiveState call = %+v", call)
	}
}

func TestPathArchiveStateRequiresConfirmationSessionReplayKeyAndTransition(t *testing.T) {
	for _, test := range []struct {
		name, body, authorization, key string
		want                           int
	}{
		{"missing session", `{"confirmed":true,"expectedArchived":false,"archived":true}`, "", "path-archive-key-0002", http.StatusUnauthorized},
		{"missing key", `{"confirmed":true,"expectedArchived":false,"archived":true}`, "Bearer valid-application-session", "", http.StatusUnprocessableEntity},
		{"missing confirmation", `{"expectedArchived":false,"archived":true}`, "Bearer valid-application-session", "path-archive-key-0002", http.StatusUnprocessableEntity},
		{"declined confirmation", `{"confirmed":false,"expectedArchived":false,"archived":true}`, "Bearer valid-application-session", "path-archive-key-0002", http.StatusBadRequest},
		{"no transition", `{"confirmed":true,"expectedArchived":true,"archived":true}`, "Bearer valid-application-session", "path-archive-key-0002", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := performArchiveStateUpdate(pathHandler(archiveStateService{}), "path-1", test.body, test.authorization, test.key)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestPathArchiveStateConcealsDeniedAndMissingAndMapsConflicts(t *testing.T) {
	body := `{"confirmed":true,"expectedArchived":false,"archived":true}`
	denied := performArchiveStateUpdate(pathHandler(archiveStateService{err: platformapp.ErrForbidden}), "secret", body, "Bearer valid-application-session", "path-archive-key-0003")
	missing := performArchiveStateUpdate(pathHandler(archiveStateService{err: ports.ErrNotFound}), "secret", body, "Bearer valid-application-session", "path-archive-key-0003")
	if denied.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || denied.Body.String() != missing.Body.String() {
		t.Fatalf("denied=%d %s missing=%d %s", denied.Code, denied.Body.String(), missing.Code, missing.Body.String())
	}
	for _, test := range []struct {
		err  error
		code string
	}{
		{ports.ErrIdempotencyConflict, "idempotency_conflict"},
		{ports.ErrConflict, "conflict"},
	} {
		response := performArchiveStateUpdate(pathHandler(archiveStateService{err: test.err}), "path-1", body, "Bearer valid-application-session", "path-archive-key-0003")
		if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
			t.Fatalf("error=%v status=%d body=%s", test.err, response.Code, response.Body.String())
		}
	}
	failure := performArchiveStateUpdate(pathHandler(archiveStateService{err: errors.New("database secret")}), "path-1", body, "Bearer valid-application-session", "path-archive-key-0003")
	if failure.Code != http.StatusInternalServerError || strings.Contains(failure.Body.String(), "database secret") {
		t.Fatalf("failure status=%d body=%s", failure.Code, failure.Body.String())
	}
}
