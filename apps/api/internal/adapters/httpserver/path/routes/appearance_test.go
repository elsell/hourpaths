package routes

import (
	"context"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func (s controlledService) ReadAppearance(_ context.Context, authorization string, _ domain.ID) (domain.Appearance, error) {
	if authorization != "Bearer valid-application-session" {
		return domain.Appearance{}, ports.ErrInvalidCredential
	}
	return domain.Appearance{}, s.dependencyError
}
func (s controlledService) SaveAppearance(_ context.Context, authorization, _ string, _ domain.ID, value domain.Appearance) (domain.Appearance, error) {
	if authorization != "Bearer valid-application-session" {
		return domain.Appearance{}, ports.ErrInvalidCredential
	}
	value.Revision++
	return value, s.dependencyError
}
func TestPersonalAppearanceHTTPContract(t *testing.T) {
	request := httptest.NewRequest(http.MethodPut, "/v1/me/path-appearances/path-1", strings.NewReader(`{"color":"mint","emoji":"🎹","expectedRevision":0}`))
	request.Header.Set("Authorization", "Bearer valid-application-session")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "appearance-key-0001")
	response := httptest.NewRecorder()
	pathHandler(controlledService{}).ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"revision":1`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
