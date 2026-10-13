package routes

import (
	"context"
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"testing"
)

type ownHistoryRouteService struct {
	controlledService
	authorization, window, cursor string
	limit                         int
}

func (s *ownHistoryRouteService) ListOwnHistory(_ context.Context, authorization, window, cursor string, limit int) ([]application.ActivityListRecord, string, error) {
	s.authorization, s.window, s.cursor, s.limit = authorization, window, cursor, limit
	return []application.ActivityListRecord{}, "next-accessible-page", s.err
}
func TestOwnHistoryRoutePreservesEmptyAuthorizedPageCursor(t *testing.T) {
	service := &ownHistoryRouteService{}
	handler, _ := shared.New(platformapp.App{}, nil, shared.Options{AccountAdmission: acceptedPolicyFixture{}, PolicyAdmission: acceptedPolicyFixture{}, DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, service) }}})
	req := httptest.NewRequest(http.MethodGet, "/v1/me/activity-history?window=retained&cursor=page&limit=7", nil)
	req.Header.Set("Authorization", "Bearer own-session")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || service.authorization != "Bearer own-session" || service.window != "retained" || service.cursor != "page" || service.limit != 7 {
		t.Fatalf("response=%d %s service=%+v", recorder.Code, recorder.Body, service)
	}
	var response struct {
		Data []ownHistoryRecord
		Meta shared.PaginationMeta
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Data == nil || len(response.Data) != 0 || response.Meta.NextCursor != "next-accessible-page" {
		t.Fatalf("response=%+v", response)
	}
	service.err = ports.ErrInvalidCredential
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("authentication failure=%d %s", recorder.Code, recorder.Body)
	}
}
