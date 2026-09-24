package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
)

func TestActivityRoutesExposeCurrentIntervalProgressAfterEveryMutation(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	timer, _ := domain.StartTimer("timer-1", "path-1", "user-1", now.Add(-time.Minute), "Etc/UTC", now)
	entry, _, _ := timer.Stop("activity-1", now, now)
	edited, revision, err := entry.EditByOwner("user-1", domain.ActivityEdit{
		StartedAt: now.Add(-2 * time.Minute), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
	}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	window := application.IntervalWindow{StartedAt: now.Add(-time.Hour), EndedAt: now.Add(time.Hour)}
	progress := func(seconds int64) *application.IntervalProgress {
		return &application.IntervalProgress{AccumulatedSeconds: seconds, TargetSeconds: 60, Window: window}
	}
	service := &capturingDeleteService{
		controlledService: controlledService{
			start:   application.StartTimerResult{Timer: timer, IntervalProgress: progress(75)},
			current: application.CurrentTimerResult{Timer: &timer, IntervalProgress: progress(0)},
			stop:    application.StopTimerResult{Activity: entry, Saved: true, IntervalProgress: progress(90)},
			create:  application.CreateManualActivityResult{Activity: entry, Version: 1, IntervalProgress: progress(0)},
			update:  application.UpdateActivityResult{Activity: edited, Revision: revision, Version: 2, IntervalProgress: progress(75)},
		},
		result: application.DeleteActivityResult{IntervalProgress: progress(90)},
	}
	body := map[string]any{"localDate": "2026-07-22", "localStartTime": "11:59:00", "durationSeconds": 60}
	tests := []struct {
		name, method, path, key string
		body                    any
		wantStatus              int
		wantSeconds             int64
	}{
		{name: "current timer", method: http.MethodGet, path: "/v1/paths/path-1/timer", wantStatus: http.StatusOK, wantSeconds: 0},
		{name: "start timer", method: http.MethodPost, path: "/v1/paths/path-1/timer", key: "timer-start-key-01", wantStatus: http.StatusOK, wantSeconds: 75},
		{name: "stop timer", method: http.MethodDelete, path: "/v1/paths/path-1/timer/timer-1", key: "timer-stop-key-001", wantStatus: http.StatusOK, wantSeconds: 90},
		{name: "manual create", method: http.MethodPost, path: "/v1/paths/path-1/activities", key: "manual-create-key", body: body, wantStatus: http.StatusCreated, wantSeconds: 0},
		{name: "manual edit", method: http.MethodPut, path: "/v1/paths/path-1/activities/activity-1", key: "manual-update-key", body: body, wantStatus: http.StatusOK, wantSeconds: 75},
		{name: "activity delete", method: http.MethodDelete, path: "/v1/paths/path-1/activities/activity-1", key: "activity-delete-key", wantStatus: http.StatusOK, wantSeconds: 90},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			var requestValue *http.Request
			if test.body != nil {
				requestValue = activityRequest(test.method, test.path, test.key, test.body)
			} else {
				requestValue = request(test.method, test.path, "Bearer application-session", test.key)
			}
			handler(service).ServeHTTP(response, requestValue)
			if response.Code != test.wantStatus {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			assertIntervalProgressResponse(t, response.Body.Bytes(), test.wantSeconds, 60)
		})
	}
}

func TestActivityRoutesOmitIntervalProgressWhenGoalIsAbsent(t *testing.T) {
	now := time.Date(2026, 7, 22, 12, 0, 0, 0, time.UTC)
	timer, _ := domain.StartTimer("timer-1", "path-1", "user-1", now.Add(-time.Minute), "Etc/UTC", now)
	entry, _, _ := timer.Stop("activity-1", now, now)
	edited, revision, err := entry.EditByOwner("user-1", domain.ActivityEdit{
		StartedAt: now.Add(-2 * time.Minute), DurationSeconds: 60, OccurrenceTimeZone: "Etc/UTC",
	}, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	service := &capturingDeleteService{
		controlledService: controlledService{
			start:   application.StartTimerResult{Timer: timer},
			current: application.CurrentTimerResult{Timer: &timer},
			stop:    application.StopTimerResult{Activity: entry, Saved: true},
			create:  application.CreateManualActivityResult{Activity: entry, Version: 1},
			update:  application.UpdateActivityResult{Activity: edited, Revision: revision, Version: 2},
		},
		result: application.DeleteActivityResult{},
	}
	body := map[string]any{"localDate": "2026-07-22", "localStartTime": "11:59:00", "durationSeconds": 60}
	requests := []*http.Request{
		request(http.MethodGet, "/v1/paths/path-1/timer", "Bearer application-session", ""),
		request(http.MethodPost, "/v1/paths/path-1/timer", "Bearer application-session", "timer-start-key-01"),
		request(http.MethodDelete, "/v1/paths/path-1/timer/timer-1", "Bearer application-session", "timer-stop-key-001"),
		activityRequest(http.MethodPost, "/v1/paths/path-1/activities", "manual-create-key", body),
		activityRequest(http.MethodPut, "/v1/paths/path-1/activities/activity-1", "manual-update-key", body),
		request(http.MethodDelete, "/v1/paths/path-1/activities/activity-1", "Bearer application-session", "activity-delete-key"),
	}
	for _, requestValue := range requests {
		response := httptest.NewRecorder()
		handler(service).ServeHTTP(response, requestValue)
		if response.Code < http.StatusOK || response.Code >= http.StatusMultipleChoices {
			t.Fatalf("%s %s status=%d body=%s", requestValue.Method, requestValue.URL.Path, response.Code, response.Body.String())
		}
		var envelope struct {
			Data map[string]json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if _, exists := envelope.Data["intervalProgress"]; exists {
			t.Fatalf("%s %s exposed false interval progress: %s", requestValue.Method, requestValue.URL.Path, response.Body.String())
		}
	}
}

func TestIntervalProgressOpenAPIIsOptionalAndOmitsInternalWindow(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, controlledService{}) }}})
	schemas := api.OpenAPI().Components.Schemas.Map()
	progress := schemas["IntervalProgress"]
	if progress == nil || len(progress.Properties) != 2 || progress.Properties["accumulatedSeconds"] == nil || progress.Properties["targetSeconds"] == nil || progress.Properties["window"] != nil {
		t.Fatalf("public interval progress schema leaked or omitted fields: %+v", progress)
	}
	for _, field := range []string{"accumulatedSeconds", "targetSeconds"} {
		if !schemaRequires(progress, field) {
			t.Fatalf("IntervalProgress does not require %q", field)
		}
	}
	for _, schemaName := range []string{"TimerState", "StopResult", "ActivityMutationResult", "ActivityDeletionResult"} {
		schema := schemas[schemaName]
		if schema == nil || schema.Properties["intervalProgress"] == nil || schemaRequires(schema, "intervalProgress") {
			t.Fatalf("%s intervalProgress must exist and remain optional: %+v", schemaName, schema)
		}
	}
}

func assertIntervalProgressResponse(t *testing.T, encoded []byte, wantAccumulated, wantTarget int64) {
	t.Helper()
	var envelope struct {
		Data struct {
			IntervalProgress *struct {
				AccumulatedSeconds int64 `json:"accumulatedSeconds"`
				TargetSeconds      int64 `json:"targetSeconds"`
			} `json:"intervalProgress"`
		} `json:"data"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil || envelope.Data.IntervalProgress == nil || envelope.Data.IntervalProgress.AccumulatedSeconds != wantAccumulated || envelope.Data.IntervalProgress.TargetSeconds != wantTarget {
		t.Fatalf("interval progress=%+v error=%v body=%s", envelope.Data.IntervalProgress, err, encoded)
	}
}

func schemaRequires(schema *huma.Schema, field string) bool {
	for _, required := range schema.Required {
		if required == field {
			return true
		}
	}
	return false
}
