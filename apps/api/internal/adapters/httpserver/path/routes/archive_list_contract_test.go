package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

type archiveListService struct {
	controlledService
	called bool
}

func (s *archiveListService) ListArchivedProjected(_ context.Context, authorization, cursor string, limit int) ([]pathapp.Projection, string, error) {
	s.called = true
	archivedAt := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	return []pathapp.Projection{{Path: domain.Entity{ID: "archived", ArchivedAt: archivedAt, Attributes: domain.Attributes{Name: "Read", Visibility: "private"}}}}, "archived-next", nil
}

func TestPathListArchivedUsesSeparateCollectionProjection(t *testing.T) {
	service := &archiveListService{}
	request := httptest.NewRequest(http.MethodGet, "/v1/paths?archived=true&limit=25", nil)
	request.Header.Set("Authorization", "Bearer valid-application-session")
	response := httptest.NewRecorder()
	pathHandler(service).ServeHTTP(response, request)
	var body struct {
		Data []struct {
			ID         string     `json:"id"`
			ArchivedAt *time.Time `json:"archivedAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || !service.called || len(body.Data) != 1 || body.Data[0].ID != "archived" || body.Data[0].ArchivedAt == nil {
		t.Fatalf("status=%d called=%t body=%s", response.Code, service.called, response.Body.String())
	}
}
