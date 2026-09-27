package routes

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfileActivityBindsAuthenticatedPaginationAndUsername(t *testing.T) {
	service := &controlledService{}
	request := httptest.NewRequest(http.MethodGet, "/v1/profiles/alex/activity?cursor=previous&limit=20", nil)
	request.Header.Set("Authorization", "Bearer application-session")
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request)
	if response.Code != http.StatusOK || service.authorization != "Bearer application-session" || service.username != "alex" || service.receivedCursor != "previous" || service.limit != 20 {
		t.Fatalf("profile activity binding status=%d service=%+v", response.Code, service)
	}
	response = httptest.NewRecorder()
	handler(service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/profiles/alex/activity?limit=101", nil))
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unbounded profile history: %d", response.Code)
	}
}
