package routes

import (
	"context"
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

type visibilityService struct {
	controlledService
	call   *[]string
	result pathapp.SetVisibilityResult
	err    error
}

func (s visibilityService) SetVisibility(_ context.Context, authorization, key string, id domain.ID, confirmed bool, expected, next string) (pathapp.SetVisibilityResult, error) {
	if s.call != nil {
		*s.call = []string{authorization, key, string(id), expected, next}
	}
	return s.result, s.err
}
func visibilityRequest(service Service, body, key string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPut, "/v1/paths/path-1/visibility", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer valid-application-session")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	w := httptest.NewRecorder()
	pathHandler(service).ServeHTTP(w, r)
	return w
}
func TestSetPathVisibilityReturnsAuthoritativeProjection(t *testing.T) {
	var call []string
	w := visibilityRequest(visibilityService{controlledService: controlledService{capabilities: pathapp.Capabilities{ManageVisibility: true}}, call: &call, result: pathapp.SetVisibilityResult{Path: domain.Entity{ID: "path-1", OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Read", Visibility: "followers"}}}}, `{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}`, "visibility-key-0001")
	var body struct {
		Data struct {
			Visibility   string `json:"visibility"`
			Capabilities struct {
				ManageVisibility bool `json:"manageVisibility"`
			} `json:"capabilities"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != http.StatusOK || body.Data.Visibility != "followers" || !body.Data.Capabilities.ManageVisibility {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if len(call) != 5 || call[1] != "visibility-key-0001" || call[3] != "private" || call[4] != "followers" {
		t.Fatalf("call=%v", call)
	}
}
func TestSetPathVisibilityValidatesAndConceals(t *testing.T) {
	for _, tc := range []struct {
		name, body, key string
		err             error
		want            int
	}{
		{"missing confirmation", `{"expectedVisibility":"private","visibility":"followers"}`, "visibility-key-0001", nil, 422},
		{"same", `{"confirmed":true,"expectedVisibility":"private","visibility":"private"}`, "visibility-key-0001", nil, 400},
		{"unknown", `{"confirmed":true,"expectedVisibility":"private","visibility":"friends"}`, "visibility-key-0001", nil, 422},
		{"missing key", `{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}`, "", nil, 422},
		{"denied", `{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}`, "visibility-key-0001", platformapp.ErrForbidden, 404},
		{"conflict", `{"confirmed":true,"expectedVisibility":"private","visibility":"followers"}`, "visibility-key-0001", ports.ErrConflict, 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := visibilityRequest(visibilityService{err: tc.err}, tc.body, tc.key)
			if w.Code != tc.want {
				t.Fatalf("status=%d want=%d body=%s", w.Code, tc.want, w.Body.String())
			}
		})
	}
}
