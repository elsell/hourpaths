package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type pushRouteAuthenticator struct{}

func (pushRouteAuthenticator) Authenticate(context.Context, string) (ports.Principal, error) {
	return ports.Principal{UserID: "user-1", Scopes: []string{"api:user"}}, nil
}

type pushRouteRepository struct {
	installation ports.PushInstallation
	deleted      string
}

func (repository *pushRouteRepository) UpsertPushInstallation(
	_ context.Context,
	installation ports.PushInstallation,
	_ audit.Event,
) error {
	repository.installation = installation
	return nil
}

func (repository *pushRouteRepository) DeletePushInstallation(
	_ context.Context,
	ownerUserID, installationID string,
	_ time.Time,
	_ audit.Event,
) error {
	repository.deleted = ownerUserID + "|" + installationID
	return nil
}

type pushRouteClock struct{ now time.Time }

func (clock pushRouteClock) Now() time.Time { return clock.now }

type pushRouteLimiter struct{}

func (pushRouteLimiter) Allow(string, time.Time) bool { return true }

func TestPushInstallationRoutesRegisterAndDeleteWithoutCredentialDisclosure(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 45, 0, 0, time.UTC)
	repository := &pushRouteRepository{}
	handler, api := New(app.App{
		Auth: pushRouteAuthenticator{}, PushInstallations: repository,
		AuditRateLimiter: pushRouteLimiter{},
		Clock:            pushRouteClock{now: now},
	}, nil, Options{DisableDocs: true})

	body := []byte(`{"provider":"expo","platform":"android","locale":"en","token":"ExpoPushToken[secret-device-token]"}`)
	request := httptest.NewRequest(
		http.MethodPut, "/v1/push-installations/installation-1", bytes.NewReader(body),
	)
	request.Header.Set("Authorization", "Bearer application")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("PUT response = %d %q", response.Code, response.Body.String())
	}
	if repository.installation.OwnerUserID != "user-1" ||
		repository.installation.ID != "installation-1" ||
		repository.installation.Token != "ExpoPushToken[secret-device-token]" {
		t.Fatalf("installation = %+v", repository.installation)
	}

	deleteRequest := httptest.NewRequest(
		http.MethodDelete, "/v1/push-installations/installation-1", nil,
	)
	deleteRequest.Header.Set("Authorization", "Bearer application")
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || deleteResponse.Body.Len() != 0 {
		t.Fatalf("DELETE response = %d %q", deleteResponse.Code, deleteResponse.Body.String())
	}
	if repository.deleted != "user-1|installation-1" {
		t.Fatalf("deleted = %q", repository.deleted)
	}

	path := api.OpenAPI().Paths["/v1/push-installations/{installationId}"]
	if path == nil || path.Put == nil || path.Delete == nil ||
		path.Put.OperationID != "upsert-push-installation" ||
		path.Delete.OperationID != "delete-push-installation" {
		t.Fatalf("push installation OpenAPI path = %+v", path)
	}
	encoded, err := json.Marshal(api.OpenAPI())
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(`"token":"ExpoPushToken[secret-device-token]"`)) {
		t.Fatal("OpenAPI disclosed a concrete push token")
	}
}

func TestPushInstallationRouteRejectsUnsupportedProviderBeforePersistence(t *testing.T) {
	repository := &pushRouteRepository{}
	handler, _ := New(app.App{
		Auth: pushRouteAuthenticator{}, PushInstallations: repository,
		AuditRateLimiter: pushRouteLimiter{},
		Clock:            pushRouteClock{now: time.Date(2026, 7, 24, 0, 46, 0, 0, time.UTC)},
	}, nil, Options{DisableDocs: true})
	request := httptest.NewRequest(
		http.MethodPut,
		"/v1/push-installations/installation-1",
		bytes.NewBufferString(`{"provider":"apns","platform":"ios","locale":"en","token":"ExpoPushToken[secret]"}`),
	)
	request.Header.Set("Authorization", "Bearer application")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("response = %d %q, want 422", response.Code, response.Body.String())
	}
	if repository.installation.ID != "" {
		t.Fatalf("invalid registration persisted: %+v", repository.installation)
	}
}
