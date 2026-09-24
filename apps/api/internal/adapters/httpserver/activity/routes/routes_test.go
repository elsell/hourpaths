package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	application "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledService struct {
	start      application.StartTimerResult
	current    application.CurrentTimerResult
	stop       application.StopTimerResult
	create     application.CreateManualActivityResult
	update     application.UpdateActivityResult
	activity   domain.RecordedActivity
	version    int64
	revisions  []application.ActivityRevisionRecord
	activities []application.ActivityListRecord
	defaults   application.ManualActivityDefaults
	err        error
}

type capturingManualService struct {
	controlledService
	input application.ManualActivityInput
}

type capturingListService struct {
	controlledService
	activityCursor, revisionCursor string
	activityParticipantID          string
	activityLimit, revisionLimit   int
	nextCursor                     string
}

type capturingDeleteService struct {
	controlledService
	result                                            application.DeleteActivityResult
	authorization, pathID, activityID, idempotencyKey string
}

func (s *capturingDeleteService) DeleteActivity(_ context.Context, authorization, pathID, activityID, idempotencyKey string) (application.DeleteActivityResult, error) {
	s.authorization, s.pathID, s.activityID, s.idempotencyKey = authorization, pathID, activityID, idempotencyKey
	return s.result, s.err
}

func (s *capturingManualService) CreateManualActivity(_ context.Context, _, _, _ string, input application.ManualActivityInput) (application.CreateManualActivityResult, error) {
	s.input = input
	return s.create, s.err
}

func (s *capturingListService) ListActivities(_ context.Context, _, _, participantID, cursor string, limit int) ([]application.ActivityListRecord, string, error) {
	s.activityParticipantID = participantID
	s.activityCursor, s.activityLimit = cursor, limit
	return s.activities, s.nextCursor, s.err
}

func (s *capturingListService) ListActivityRevisions(_ context.Context, _, _, _, cursor string, limit int) ([]application.ActivityRevisionRecord, string, error) {
	s.revisionCursor, s.revisionLimit = cursor, limit
	return s.revisions, s.nextCursor, s.err
}

func (s controlledService) StartTimer(context.Context, string, string, string) (application.StartTimerResult, error) {
	return s.start, s.err
}
func (s controlledService) CurrentTimer(context.Context, string, string) (application.CurrentTimerResult, error) {
	return s.current, s.err
}
func (s controlledService) StopTimer(context.Context, string, string, string, string) (application.StopTimerResult, error) {
	return s.stop, s.err
}
func (s controlledService) CreateManualActivity(context.Context, string, string, string, application.ManualActivityInput) (application.CreateManualActivityResult, error) {
	return s.create, s.err
}
func (s controlledService) UpdateActivity(context.Context, string, string, string, string, application.ManualActivityInput) (application.UpdateActivityResult, error) {
	return s.update, s.err
}
func (s controlledService) DeleteActivity(context.Context, string, string, string, string) (application.DeleteActivityResult, error) {
	return application.DeleteActivityResult{}, s.err
}
func (s controlledService) GetActivity(context.Context, string, string, string) (domain.RecordedActivity, int64, error) {
	return s.activity, s.version, s.err
}
func (s controlledService) ListActivityRevisions(context.Context, string, string, string, string, int) ([]application.ActivityRevisionRecord, string, error) {
	return s.revisions, "", s.err
}
func (s controlledService) ListActivities(context.Context, string, string, string, string, int) ([]application.ActivityListRecord, string, error) {
	return s.activities, "", s.err
}
func (s controlledService) ManualActivityDefaults(context.Context, string, string) (application.ManualActivityDefaults, error) {
	return s.defaults, s.err
}

func handler(service Service) http.Handler {
	h, _ := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, service) }}})
	return h
}

func request(method, path, authorization, key string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	if authorization != "" {
		r.Header.Set("Authorization", authorization)
	}
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	return r
}

func activityRequest(method, path, key string, body any) *http.Request {
	encoded, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	r.Header.Set("Authorization", "Bearer application-session")
	r.Header.Set("Content-Type", "application/json")
	if key != "" {
		r.Header.Set("Idempotency-Key", key)
	}
	return r
}

func TestTimerRoutesReturnReloadSafeStartCurrentAndStopState(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	timer, _ := domain.StartTimer("timer-1", "path-1", "user-1", now.Add(-time.Minute), "America/New_York", now)
	entry, _, _ := timer.Stop("activity-1", now, now)
	service := controlledService{start: application.StartTimerResult{Timer: timer, AccumulatedSeconds: 120}, current: application.CurrentTimerResult{Timer: &timer, AccumulatedSeconds: 120}, stop: application.StopTimerResult{Activity: entry, Saved: true, AccumulatedSeconds: 180}}
	h := handler(service)
	tests := []struct {
		method, path, key      string
		wantRunning, wantSaved bool
		wantTotal              int64
	}{
		{http.MethodPost, "/v1/paths/path-1/timer", "timer-start-key-01", true, false, 120},
		{http.MethodGet, "/v1/paths/path-1/timer", "", true, false, 120},
		{http.MethodDelete, "/v1/paths/path-1/timer/timer-1", "timer-stop-key-001", false, true, 180},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request(test.method, test.path, "Bearer application-session", test.key))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", test.method, response.Code, response.Body.String())
		}
		var body struct {
			Data struct {
				Running, Saved, Subsecond bool
				AccumulatedSeconds        int64
				Timer, Activity           json.RawMessage
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Data.Running != test.wantRunning || body.Data.Saved != test.wantSaved || body.Data.AccumulatedSeconds != test.wantTotal {
			t.Fatalf("%s response=%s err=%v", test.method, response.Body.String(), err)
		}
		if test.wantSaved && (body.Data.Subsecond || len(body.Data.Activity) == 0) {
			t.Fatalf("stop outcome=%s", response.Body.String())
		}
	}
}

func TestManualActivityRoutesCreateEditReadAndInspectRevisions(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	original, err := domain.RecordManualActivity(domain.ManualActivity{ID: "activity-1", PathID: "path-1", ParticipantID: "user-1", StartedAt: now.Add(-time.Hour), DurationSeconds: 600, OccurrenceTimeZone: "America/New_York", Note: "before"}, now.Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	edited, revision, err := original.EditByOwner("user-1", domain.ActivityEdit{StartedAt: now.Add(-time.Hour), DurationSeconds: 1200, OccurrenceTimeZone: "America/New_York", Note: "after"}, now)
	if err != nil {
		t.Fatal(err)
	}
	service := controlledService{
		create:   application.CreateManualActivityResult{Activity: original, Version: 1, AccumulatedSeconds: 600},
		update:   application.UpdateActivityResult{Activity: edited, Revision: revision, Version: 2, AccumulatedSeconds: 1200},
		activity: edited, version: 2, activities: []application.ActivityListRecord{{Activity: edited, Version: 2}}, revisions: []application.ActivityRevisionRecord{{Revision: revision, Version: 1}},
	}
	h := handler(service)
	body := map[string]any{"localDate": "2026-07-21", "localStartTime": "13:00:00", "durationSeconds": 600, "note": "practice"}
	tests := []struct {
		method, path, key string
		body              any
		wantStatus        int
		wantVersion       int64
	}{
		{http.MethodPost, "/v1/paths/path-1/activities", "manual-create-key", body, http.StatusCreated, 1},
		{http.MethodPut, "/v1/paths/path-1/activities/activity-1", "manual-update-key", body, http.StatusOK, 2},
		{http.MethodGet, "/v1/paths/path-1/activities", "", nil, http.StatusOK, 2},
		{http.MethodGet, "/v1/paths/path-1/activities/activity-1", "", nil, http.StatusOK, 2},
		{http.MethodGet, "/v1/paths/path-1/activities/activity-1/revisions", "", nil, http.StatusOK, 1},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		var req *http.Request
		if test.body != nil {
			req = activityRequest(test.method, test.path, test.key, test.body)
		} else {
			req = request(test.method, test.path, "Bearer application-session", test.key)
		}
		h.ServeHTTP(response, req)
		if response.Code != test.wantStatus {
			t.Fatalf("%s %s status=%d body=%s", test.method, test.path, response.Code, response.Body.String())
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		var resource struct {
			Version int64 `json:"version"`
		}
		if len(envelope.Data) > 0 && envelope.Data[0] == '[' {
			var items []struct {
				Version int64 `json:"version"`
			}
			if err := json.Unmarshal(envelope.Data, &items); err != nil || len(items) != 1 {
				t.Fatalf("%s %s response=%s error=%v", test.method, test.path, response.Body.String(), err)
			}
			resource.Version = items[0].Version
		} else if err := json.Unmarshal(envelope.Data, &resource); err != nil {
			t.Fatal(err)
		}
		if resource.Version != test.wantVersion {
			t.Fatalf("%s %s response=%s", test.method, test.path, response.Body.String())
		}
	}
}

func TestDeleteActivityRouteRequiresIdempotencyAndReturnsAuthoritativeTotal(t *testing.T) {
	service := &capturingDeleteService{result: application.DeleteActivityResult{AccumulatedSeconds: 42, SessionCount: 3, UnreadNotificationCount: 4, RemovedFeedEventIDs: []string{"achievement:a", "practice:activity-1"}}}
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, request(http.MethodDelete, "/v1/paths/path-1/activities/activity-1", "Bearer application-session", "activity-delete-key"))
	if response.Code != http.StatusOK {
		t.Fatalf("DELETE status=%d body=%s", response.Code, response.Body.String())
	}
	if service.authorization != "Bearer application-session" || service.pathID != "path-1" || service.activityID != "activity-1" || service.idempotencyKey != "activity-delete-key" {
		t.Fatalf("delete input = %+v", service)
	}
	var body struct {
		Data struct {
			AccumulatedSeconds      int64    `json:"accumulatedSeconds"`
			SessionCount            int64    `json:"sessionCount"`
			UnreadNotificationCount int64    `json:"unreadNotificationCount"`
			RemovedFeedEventIDs     []string `json:"removedFeedEventIds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Data.AccumulatedSeconds != 42 || body.Data.SessionCount != 3 || body.Data.UnreadNotificationCount != 4 || !slices.Equal(body.Data.RemovedFeedEventIDs, []string{"achievement:a", "practice:activity-1"}) {
		t.Fatalf("DELETE response=%s err=%v", response.Body.String(), err)
	}

	missingKey := httptest.NewRecorder()
	handler(service).ServeHTTP(missingKey, request(http.MethodDelete, "/v1/paths/path-1/activities/activity-1", "Bearer application-session", ""))
	if missingKey.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing key status=%d body=%s", missingKey.Code, missingKey.Body.String())
	}
}

func TestDeleteActivityRouteConcealsDeniedAndMissingIdentically(t *testing.T) {
	responses := make([]*httptest.ResponseRecorder, 0, 2)
	for _, inaccessible := range []error{platformapp.ErrForbidden, ports.ErrNotFound} {
		response := httptest.NewRecorder()
		handler(&capturingDeleteService{controlledService: controlledService{err: inaccessible}}).ServeHTTP(response, request(http.MethodDelete, "/v1/paths/path-1/activities/activity-1", "Bearer application-session", "activity-delete-key"))
		responses = append(responses, response)
	}
	if responses[0].Code != http.StatusNotFound || responses[1].Code != http.StatusNotFound || !bytes.Equal(responses[0].Body.Bytes(), responses[1].Body.Bytes()) {
		t.Fatalf("inaccessible deletion differs: denied=%d %q missing=%d %q", responses[0].Code, responses[0].Body.String(), responses[1].Code, responses[1].Body.String())
	}
}

func TestActivityHistoryRoutesForwardPaginationAndReturnNextCursor(t *testing.T) {
	service := &capturingListService{controlledService: controlledService{}, nextCursor: "signed-next-page"}
	h := handler(service)
	for _, test := range []struct {
		path   string
		assert func()
	}{
		{path: "/v1/paths/path-1/activities?participantId=user-2&cursor=signed-current-page&limit=100", assert: func() {
			if service.activityParticipantID != "user-2" || service.activityCursor != "signed-current-page" || service.activityLimit != 100 {
				t.Fatalf("activity pagination = %q, %d", service.activityCursor, service.activityLimit)
			}
		}},
		{path: "/v1/paths/path-1/activities/activity-1/revisions?cursor=signed-current-page", assert: func() {
			if service.revisionCursor != "signed-current-page" || service.revisionLimit != 25 {
				t.Fatalf("revision pagination = %q, %d", service.revisionCursor, service.revisionLimit)
			}
		}},
	} {
		response := httptest.NewRecorder()
		h.ServeHTTP(response, request(http.MethodGet, test.path, "Bearer application-session", ""))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status=%d body=%s", test.path, response.Code, response.Body.String())
		}
		var envelope struct {
			Data json.RawMessage `json:"data"`
			Meta struct {
				NextCursor string `json:"nextCursor"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil || string(envelope.Data) != "[]" || envelope.Meta.NextCursor != "signed-next-page" {
			t.Fatalf("GET %s meta=%+v error=%v body=%s", test.path, envelope.Meta, err, response.Body.String())
		}
		test.assert()
	}
}

func TestManualActivityDefaultsRouteCarriesAuthoritativeInstantAndTimeZone(t *testing.T) {
	instant := time.Date(2026, time.November, 1, 6, 30, 0, 0, time.UTC)
	response := httptest.NewRecorder()
	handler(controlledService{defaults: application.ManualActivityDefaults{
		LocalDate: "2026-11-01", LocalStartTime: "01:30:00", TimeZone: "America/New_York", CurrentInstant: instant,
	}}).ServeHTTP(response, request(http.MethodGet, "/v1/paths/path-1/activities/manual-defaults", "Bearer application-session", ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			CurrentInstant time.Time `json:"currentInstant"`
			TimeZone       string    `json:"timeZone"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Data.CurrentInstant != instant || body.Data.TimeZone != "America/New_York" {
		t.Fatalf("response=%s err=%v", response.Body.String(), err)
	}
}

func TestManualActivityRouteDefersNoteLengthUntilAfterNFCNormalization(t *testing.T) {
	now := time.Date(2026, 7, 21, 18, 0, 0, 0, time.UTC)
	entry, err := domain.RecordManualActivity(domain.ManualActivity{ID: "activity-1", PathID: "path-1", ParticipantID: "user-1", StartedAt: now.Add(-time.Hour), DurationSeconds: 600, OccurrenceTimeZone: "America/New_York"}, now)
	if err != nil {
		t.Fatal(err)
	}
	service := &capturingManualService{controlledService: controlledService{create: application.CreateManualActivityResult{Activity: entry, Version: 1, AccumulatedSeconds: 600}}}
	decomposed := strings.Repeat("e\u0301", 2000)
	response := httptest.NewRecorder()
	handler(service).ServeHTTP(response, activityRequest(http.MethodPost, "/v1/paths/path-1/activities", "manual-create-key", map[string]any{
		"localDate": "2026-07-21", "localStartTime": "13:00:00", "durationSeconds": 600, "note": decomposed,
	}))
	if response.Code != http.StatusCreated || service.input.Note != decomposed {
		t.Fatalf("status=%d noteLength=%d body=%s", response.Code, len([]rune(service.input.Note)), response.Body.String())
	}
}

func TestStopTimerRouteReturnsSubsecondResultWithoutActivity(t *testing.T) {
	response := httptest.NewRecorder()
	handler(controlledService{stop: application.StopTimerResult{Saved: false, AccumulatedSeconds: 47}}).
		ServeHTTP(response, request(http.MethodDelete, "/v1/paths/path-1/timer/timer-1", "Bearer application-session", "timer-stop-key-001"))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Running            bool            `json:"running"`
			Saved              bool            `json:"saved"`
			Subsecond          bool            `json:"subsecond"`
			AccumulatedSeconds int64           `json:"accumulatedSeconds"`
			Activity           json.RawMessage `json:"activity"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Running || body.Data.Saved || !body.Data.Subsecond || body.Data.AccumulatedSeconds != 47 || len(body.Data.Activity) != 0 {
		t.Fatalf("subsecond stop response=%s", response.Body.String())
	}
}

func TestDelayedStartReplayReturnsStoppedCurrentState(t *testing.T) {
	response := httptest.NewRecorder()
	handler(controlledService{start: application.StartTimerResult{AccumulatedSeconds: 180, Replayed: true}}).
		ServeHTTP(response, request(http.MethodPost, "/v1/paths/path-1/timer", "Bearer application-session", "timer-start-key-01"))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Running            bool            `json:"running"`
			Timer              json.RawMessage `json:"timer"`
			AccumulatedSeconds int64           `json:"accumulatedSeconds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Data.Running || len(body.Data.Timer) != 0 || body.Data.AccumulatedSeconds != 180 {
		t.Fatalf("response=%s err=%v", response.Body.String(), err)
	}
}

func TestDelayedStopReplayReturnsNewerCurrentTimer(t *testing.T) {
	now := time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC)
	stoppedTimer, _ := domain.StartTimer("timer-1", "path-1", "user-1", now.Add(-time.Minute), "America/New_York", now)
	entry, _, _ := stoppedTimer.Stop("activity-1", now, now)
	currentTimer, _ := domain.StartTimer("timer-2", "path-1", "user-1", now.Add(time.Second), "America/New_York", now.Add(time.Second))
	response := httptest.NewRecorder()
	handler(controlledService{stop: application.StopTimerResult{Activity: entry, CurrentTimer: &currentTimer, AccumulatedSeconds: 60, Saved: true, Replayed: true}}).
		ServeHTTP(response, request(http.MethodDelete, "/v1/paths/path-1/timer/timer-1", "Bearer application-session", "timer-stop-key-001"))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Running bool `json:"running"`
			Timer   struct {
				ID string `json:"id"`
			} `json:"timer"`
			Activity struct {
				ID string `json:"id"`
			} `json:"activity"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || !body.Data.Running || body.Data.Timer.ID != "timer-2" || body.Data.Activity.ID != "activity-1" {
		t.Fatalf("response=%s err=%v", response.Body.String(), err)
	}
}

func TestTimerRoutesEnforceHeadersAndMapFailuresSafely(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		path := "/v1/paths/path-1/timer"
		if method == http.MethodDelete {
			path += "/timer-1"
		}
		response := httptest.NewRecorder()
		handler(controlledService{}).ServeHTTP(response, request(method, path, "Bearer valid", ""))
		if response.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%s missing key status=%d body=%s", method, response.Code, response.Body.String())
		}
	}
	for _, failure := range []struct {
		err  error
		want int
	}{{ports.ErrInvalidCredential, http.StatusUnauthorized}, {platformapp.ErrForbidden, http.StatusNotFound}, {errors.New("database secret"), http.StatusInternalServerError}} {
		response := httptest.NewRecorder()
		handler(controlledService{err: failure.err}).ServeHTTP(response, request(http.MethodGet, "/v1/paths/path-1/timer", "Bearer invalid", ""))
		if response.Code != failure.want {
			t.Fatalf("error=%v status=%d body=%s", failure.err, response.Code, response.Body.String())
		}
		if failure.want == http.StatusInternalServerError && response.Body.String() == "database secret" {
			t.Fatal("dependency detail leaked")
		}
	}
}

func TestTimerRoutesConcealDeniedAndMissingResourcesIdentically(t *testing.T) {
	tests := []struct {
		name, method, path, key string
	}{
		{"start", http.MethodPost, "/v1/paths/path-1/timer", "timer-start-key-01"},
		{"current", http.MethodGet, "/v1/paths/path-1/timer", ""},
		{"stop", http.MethodDelete, "/v1/paths/path-1/timer/timer-1", "timer-stop-key-001"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responses := make([]*httptest.ResponseRecorder, 0, 2)
			for _, inaccessible := range []error{platformapp.ErrForbidden, ports.ErrNotFound} {
				response := httptest.NewRecorder()
				handler(controlledService{err: inaccessible}).ServeHTTP(response, request(test.method, test.path, "Bearer application-session", test.key))
				responses = append(responses, response)
			}
			if responses[0].Code != http.StatusNotFound || responses[1].Code != http.StatusNotFound {
				t.Fatalf("denied status=%d missing status=%d", responses[0].Code, responses[1].Code)
			}
			if !bytes.Equal(responses[0].Body.Bytes(), responses[1].Body.Bytes()) {
				t.Fatalf("inaccessible responses differ: denied=%q missing=%q", responses[0].Body.String(), responses[1].Body.String())
			}
			for _, header := range []string{"Cache-Control", "Content-Type", "Link", "Referrer-Policy", "X-Content-Type-Options"} {
				if denied, missing := responses[0].Header().Values(header), responses[1].Header().Values(header); !slices.Equal(denied, missing) {
					t.Fatalf("inaccessible %s headers differ: denied=%v missing=%v", header, denied, missing)
				}
			}
		})
	}
}
