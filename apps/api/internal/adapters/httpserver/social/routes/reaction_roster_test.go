package routes

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestEmojiEngagementPreservesUnicodeAndAuthenticatedIntent(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		route := "/v1/social/feed/practice%3Aevent/reactions/" + url.PathEscape("🎸")
		if method == http.MethodGet {
			route = "/v1/social/feed/practice%3Aevent/reactions?reaction=" + url.QueryEscape("🎸") + "&cursor=next&limit=12"
		}
		service := &controlledService{}
		request := httptest.NewRequest(method, route, nil)
		request.Header.Set("Authorization", "Bearer viewer")
		request.Header.Set("Idempotency-Key", "reaction-intent-key")
		response := httptest.NewRecorder()
		handler(service).ServeHTTP(response, request)
		if response.Code != http.StatusOK || service.authorization != "Bearer viewer" || service.eventID != "practice:event" || string(service.reaction) != "🎸" {
			t.Fatalf("method=%s status=%d service=%+v body=%s", method, response.Code, service, response.Body.String())
		}
		if method == http.MethodGet && (service.receivedCursor != "next" || service.limit != 12) {
			t.Fatalf("pagination not preserved: %+v", service)
		}
	}
}

func TestEmojiEngagementFailuresRemainOpaque(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		for _, test := range []struct {
			failure error
			status  int
		}{{platformapp.ErrUnauthenticated, 401}, {ports.ErrInvalidCredential, 401}, {ports.ErrNotFound, 404}, {ports.ErrUnavailable, 503}} {
			route := "/v1/social/feed/practice%3Aevent/reactions/" + url.PathEscape("🎸")
			if method == http.MethodGet {
				route = "/v1/social/feed/practice%3Aevent/reactions?reaction=heart"
			}
			request := httptest.NewRequest(method, route, nil)
			request.Header.Set("Authorization", "Bearer rejected")
			request.Header.Set("Idempotency-Key", "reaction-intent-key")
			response := httptest.NewRecorder()
			handler(&controlledService{err: fmt.Errorf("private backing detail: %w", test.failure)}).ServeHTTP(response, request)
			if response.Code != test.status || strings.Contains(response.Body.String(), "private backing detail") {
				t.Fatalf("%s status=%d body=%s", method, response.Code, response.Body.String())
			}
		}
	}
}
