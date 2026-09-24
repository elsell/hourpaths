package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type goalUpdateCall struct {
	authorization, idempotencyKey string
	pathID                        domain.ID
	confirmed                     bool
	expected                      domain.GoalConfiguration
	intervalGoal                  domain.IntervalGoal
	overallTarget                 domain.OverallTarget
}

type goalUpdateService struct {
	controlledService
	call   *goalUpdateCall
	result pathapp.UpdateGoalsResult
	err    error
}

func (s goalUpdateService) UpdateGoals(
	_ context.Context,
	authorization, idempotencyKey string,
	pathID domain.ID,
	confirmed bool,
	expected domain.GoalConfiguration,
	intervalGoal domain.IntervalGoal,
	overallTarget domain.OverallTarget,
) (pathapp.UpdateGoalsResult, error) {
	if authorization != "Bearer valid-application-session" {
		return pathapp.UpdateGoalsResult{}, ports.ErrInvalidCredential
	}
	if s.call != nil {
		*s.call = goalUpdateCall{authorization, idempotencyKey, pathID, confirmed, expected, intervalGoal, overallTarget}
	}
	return s.result, s.err
}

func performGoalUpdate(handler http.Handler, id, body, authorization, idempotencyKey string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPut, "/v1/paths/"+id+"/goals", strings.NewReader(body))
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

func TestPathGoalUpdateCallsDedicatedServiceAndReturnsAuthoritativeProjection(t *testing.T) {
	call := &goalUpdateCall{}
	updated := domain.Entity{
		ID:          "path-1",
		OwnerUserID: "creator",
		Attributes: domain.Attributes{
			Name:       "Read",
			Visibility: "private",
			IntervalGoal: domain.IntervalGoal{
				Present: true, TargetSeconds: 30, Recurrence: domain.RecurrenceDaily,
				Alignment: domain.GoalAlignment{Hour: 6},
			},
		},
	}
	progress := &pathapp.GoalIntervalProgress{AccumulatedSeconds: 45, TargetSeconds: 30}
	response := performGoalUpdate(pathHandler(goalUpdateService{
		controlledService: controlledService{capabilities: pathapp.Capabilities{
			TrackTime: true, InviteMembers: true, ManageGoals: true,
		}},
		call: call,
		result: pathapp.UpdateGoalsResult{
			Path: updated, AccumulatedSeconds: 45, IntervalProgress: progress,
		},
	}), "path-1", `{"confirmed":true,"expectedGoals":{"intervalGoal":{"targetSeconds":60,"recurrence":"weekly","alignment":{"isoWeekday":1}},"overallTarget":{"targetSeconds":3600}},"intervalGoal":{"targetSeconds":30,"recurrence":"daily","alignment":{"hour":6}}}`, "Bearer valid-application-session", "path-goals-key-0001")

	var body struct {
		Data struct {
			Path struct {
				ID, Name, Visibility string
				Capabilities         pathapp.Capabilities
				IntervalGoal         *struct {
					TargetSeconds int64
					Recurrence    string
					Alignment     struct{ Hour *int }
				}
			}
			AccumulatedSeconds int64
			IntervalProgress   *struct{ AccumulatedSeconds, TargetSeconds int64 }
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || body.Data.Path.ID != "path-1" ||
		body.Data.Path.IntervalGoal == nil || body.Data.Path.IntervalGoal.Alignment.Hour == nil ||
		*body.Data.Path.IntervalGoal.Alignment.Hour != 6 ||
		body.Data.Path.Capabilities != (pathapp.Capabilities{TrackTime: true, InviteMembers: true, ManageGoals: true}) ||
		body.Data.AccumulatedSeconds != 45 || body.Data.IntervalProgress == nil ||
		body.Data.IntervalProgress.AccumulatedSeconds != 45 || body.Data.IntervalProgress.TargetSeconds != 30 {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	wantGoal := domain.IntervalGoal{
		Present: true, TargetSeconds: 30, Recurrence: domain.RecurrenceDaily,
		Alignment: domain.GoalAlignment{Hour: 6},
	}
	wantExpected := domain.GoalConfiguration{
		IntervalGoal: domain.IntervalGoal{
			Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceWeekly,
			Alignment: domain.GoalAlignment{ISOWeekday: 1},
		},
		OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3_600},
	}
	if call.authorization != "Bearer valid-application-session" || call.idempotencyKey != "path-goals-key-0001" ||
		call.pathID != "path-1" || !call.confirmed || call.expected != wantExpected || call.intervalGoal != wantGoal ||
		call.overallTarget != (domain.OverallTarget{}) {
		t.Fatalf("UpdateGoals call = %+v", call)
	}
}

func TestPathGoalUpdateOmissionRemovesGoalsAndRejectsPathMetadata(t *testing.T) {
	call := &goalUpdateCall{}
	removed := domain.Entity{
		ID: "path-1", OwnerUserID: "creator",
		Attributes: domain.Attributes{Name: "Read", Visibility: "private"},
	}
	response := performGoalUpdate(pathHandler(goalUpdateService{
		call:   call,
		result: pathapp.UpdateGoalsResult{Path: removed, AccumulatedSeconds: 45},
	}), "path-1", `{"confirmed":true,"expectedGoals":{"overallTarget":{"targetSeconds":45}}}`, "Bearer valid-application-session", "path-goals-key-0002")
	if response.Code != http.StatusOK || call.intervalGoal != (domain.IntervalGoal{}) ||
		call.expected != (domain.GoalConfiguration{OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 45}}) ||
		call.overallTarget != (domain.OverallTarget{}) ||
		strings.Contains(response.Body.String(), "intervalProgress") {
		t.Fatalf("removal status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}

	for _, metadata := range []string{
		`{"confirmed":true,"expectedGoals":{},"name":"Renamed"}`,
		`{"confirmed":true,"expectedGoals":{},"visibility":"public"}`,
	} {
		rejected := performGoalUpdate(pathHandler(goalUpdateService{}), "path-1", metadata, "Bearer valid-application-session", "path-goals-key-0002")
		if rejected.Code != http.StatusUnprocessableEntity {
			t.Fatalf("metadata body %s status=%d response=%s", metadata, rejected.Code, rejected.Body.String())
		}
	}
}

func TestPathGoalUpdateRequiresConfirmationSessionAndReplayKey(t *testing.T) {
	for _, test := range []struct {
		name, body, authorization, key string
		want                           int
	}{
		{"missing session", `{"confirmed":true,"expectedGoals":{}}`, "", "path-goals-key-0003", http.StatusUnauthorized},
		{"missing key", `{"confirmed":true,"expectedGoals":{}}`, "Bearer valid-application-session", "", http.StatusUnprocessableEntity},
		{"missing confirmation", `{"expectedGoals":{}}`, "Bearer valid-application-session", "path-goals-key-0003", http.StatusUnprocessableEntity},
		{"missing reviewed goals", `{"confirmed":true}`, "Bearer valid-application-session", "path-goals-key-0003", http.StatusUnprocessableEntity},
		{"declined confirmation", `{"confirmed":false,"expectedGoals":{}}`, "Bearer valid-application-session", "path-goals-key-0003", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := performGoalUpdate(pathHandler(goalUpdateService{}), "path-1", test.body, test.authorization, test.key)
			if response.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestPathGoalUpdateConcealsDeniedAndMissingAndMapsReplayConflict(t *testing.T) {
	denied := performGoalUpdate(pathHandler(goalUpdateService{err: platformapp.ErrForbidden}), "secret", `{"confirmed":true,"expectedGoals":{}}`, "Bearer valid-application-session", "path-goals-key-0004")
	missing := performGoalUpdate(pathHandler(goalUpdateService{err: ports.ErrNotFound}), "secret", `{"confirmed":true,"expectedGoals":{}}`, "Bearer valid-application-session", "path-goals-key-0004")
	if denied.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || denied.Body.String() != missing.Body.String() {
		t.Fatalf("denied=%d %s missing=%d %s", denied.Code, denied.Body.String(), missing.Code, missing.Body.String())
	}

	conflict := performGoalUpdate(pathHandler(goalUpdateService{err: ports.ErrIdempotencyConflict}), "path-1", `{"confirmed":true,"expectedGoals":{}}`, "Bearer valid-application-session", "path-goals-key-0004")
	if conflict.Code != http.StatusConflict || !strings.Contains(conflict.Body.String(), `"code":"idempotency_conflict"`) {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}

	stale := performGoalUpdate(pathHandler(goalUpdateService{err: ports.ErrConflict}), "path-1", `{"confirmed":true,"expectedGoals":{}}`, "Bearer valid-application-session", "path-goals-key-0005")
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), `"code":"conflict"`) {
		t.Fatalf("stale status=%d body=%s", stale.Code, stale.Body.String())
	}

	failure := performGoalUpdate(pathHandler(goalUpdateService{err: errors.New("database secret")}), "path-1", `{"confirmed":true,"expectedGoals":{}}`, "Bearer valid-application-session", "path-goals-key-0004")
	if failure.Code != http.StatusInternalServerError || strings.Contains(failure.Body.String(), "database secret") {
		t.Fatalf("failure status=%d body=%s", failure.Code, failure.Body.String())
	}
}
