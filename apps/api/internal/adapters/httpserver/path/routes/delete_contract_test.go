package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestDeletePathRequiresBoundConfirmationAndReturnsAuthoritativeReceipt(t *testing.T) {
	call := &struct {
		authorization, idempotencyKey, expectedName string
		id                                          domain.ID
		confirmed                                   bool
	}{}
	service := controlledService{deleteCall: call, deleteResult: pathapp.DeletePathResult{PathID: "path-1", Deleted: true}}
	request := httptest.NewRequest(http.MethodDelete, "/v1/paths/path-1", strings.NewReader(`{"confirmed":true,"expectedName":"Guitar"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer valid-application-session")
	request.Header.Set("Idempotency-Key", "delete-path-key-0001")
	response := httptest.NewRecorder()
	pathHandler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || call.authorization != "Bearer valid-application-session" || call.idempotencyKey != "delete-path-key-0001" || call.id != "path-1" || !call.confirmed || call.expectedName != "Guitar" {
		t.Fatalf("status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
	var body struct {
		Data struct {
			PathID  string `json:"pathId"`
			Deleted bool   `json:"deleted"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Data.PathID != "path-1" || !body.Data.Deleted {
		t.Fatalf("receipt=%+v err=%v body=%s", body, err, response.Body.String())
	}
}

func TestDeletePathRejectsUnconfirmedRequestBeforeService(t *testing.T) {
	call := &struct {
		authorization, idempotencyKey, expectedName string
		id                                          domain.ID
		confirmed                                   bool
	}{}
	request := httptest.NewRequest(http.MethodDelete, "/v1/paths/path-1", strings.NewReader(`{"confirmed":false,"expectedName":"Guitar"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer valid-application-session")
	request.Header.Set("Idempotency-Key", "delete-path-key-0001")
	response := httptest.NewRecorder()
	pathHandler(controlledService{deleteCall: call}).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest || call.authorization != "" {
		t.Fatalf("status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
}
