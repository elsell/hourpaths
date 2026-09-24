package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

type leaveHTTPCall struct {
	authorization, key  string
	pathID              domain.ID
	confirmed, retained bool
}
type leaveHTTPService struct {
	controlledService
	call   *leaveHTTPCall
	result pathapp.LeavePathResult
}

func (s leaveHTTPService) Leave(_ context.Context, authorization, key string, pathID domain.ID, confirmed, retained bool) (pathapp.LeavePathResult, error) {
	*s.call = leaveHTTPCall{authorization, key, pathID, confirmed, retained}
	if s.result.PathID != "" {
		return s.result, nil
	}
	return pathapp.LeavePathResult{PathID: pathID, Left: true, ActivityRetained: true}, nil
}

func TestLeavePathContractRequiresExplicitRetentionAndReturnsReceipt(t *testing.T) {
	call := &leaveHTTPCall{}
	handler := pathHandler(leaveHTTPService{call: call})
	request := httptest.NewRequest(http.MethodDelete, "/v1/paths/path-1/membership", strings.NewReader(`{"confirmed":true,"retainActivity":true}`))
	request.Header.Set("Authorization", "Bearer application-session")
	request.Header.Set("Idempotency-Key", "leave-path-key-0001")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var body struct {
		Data struct {
			PathID   string `json:"pathId"`
			Left     bool   `json:"left"`
			Retained bool   `json:"activityRetained"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || *call != (leaveHTTPCall{"Bearer application-session", "leave-path-key-0001", "path-1", true, true}) || body.Data.PathID != "path-1" || !body.Data.Left || !body.Data.Retained {
		t.Fatalf("status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
}

func TestLeavePathContractAcceptsExplicitActivityDeletionAndReturnsReceipt(t *testing.T) {
	call := &leaveHTTPCall{}
	service := leaveHTTPService{call: call, result: pathapp.LeavePathResult{PathID: "path-1", Left: true, ActivityRetained: false}}
	handler := pathHandler(service)
	request := httptest.NewRequest(http.MethodDelete, "/v1/paths/path-1/membership", strings.NewReader(`{"confirmed":true,"retainActivity":false}`))
	request.Header.Set("Authorization", "Bearer application-session")
	request.Header.Set("Idempotency-Key", "leave-path-delete-0001")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || call.retained {
		t.Fatalf("status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"activityRetained":false`) {
		t.Fatalf("receipt=%s", response.Body.String())
	}
}

func TestLeavePathContractFailsClosedWithoutExplicitRetainedActivityChoice(t *testing.T) {
	for _, test := range []struct {
		body string
		want int
	}{{`{"confirmed":false,"retainActivity":true}`, http.StatusBadRequest}, {`{"confirmed":true}`, http.StatusUnprocessableEntity}} {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodDelete, "/v1/paths/path-1/membership", strings.NewReader(test.body))
		request.Header.Set("Authorization", "Bearer application-session")
		request.Header.Set("Idempotency-Key", "leave-path-key-0001")
		request.Header.Set("Content-Type", "application/json")
		pathHandler(controlledService{}).ServeHTTP(response, request)
		if response.Code != test.want {
			t.Fatalf("body=%s status=%d response=%s", test.body, response.Code, response.Body.String())
		}
	}
}
