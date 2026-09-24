package routes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledService struct {
	dependencyError error
	createEntity    domain.Entity
	createError     error
	createCall      *struct {
		authorization, idempotencyKey string
		attributes                    domain.Attributes
	}
	listItems       []domain.Entity
	listProjections []pathapp.Projection
	listCursor      string
	capabilities    pathapp.Capabilities
	listCall        *struct {
		authorization, cursor string
		limit                 int
	}
	renameCall *struct {
		authorization, idempotencyKey, expectedName, name string
		id                                                domain.ID
	}
	renameResult pathapp.RenameResult
	renameError  error
	deleteCall   *struct {
		authorization, idempotencyKey, expectedName string
		id                                          domain.ID
		confirmed                                   bool
	}
	deleteResult pathapp.DeletePathResult
	deleteError  error
}

func (s controlledService) Create(_ context.Context, authorization, idempotencyKey string, attributes domain.Attributes) (domain.Entity, error) {
	if authorization != "Bearer valid-application-session" {
		return domain.Entity{}, ports.ErrInvalidCredential
	}
	if s.createCall != nil {
		s.createCall.authorization, s.createCall.idempotencyKey, s.createCall.attributes = authorization, idempotencyKey, attributes
	}
	if s.createError != nil {
		return domain.Entity{}, s.createError
	}
	return s.createEntity, nil
}
func (s controlledService) List(_ context.Context, authorization, cursor string, limit int) ([]domain.Entity, string, error) {
	if authorization != "Bearer valid-application-session" {
		return nil, "", ports.ErrInvalidCredential
	}
	if s.dependencyError != nil {
		return nil, "", s.dependencyError
	}
	if s.listCall != nil {
		s.listCall.authorization, s.listCall.cursor, s.listCall.limit = authorization, cursor, limit
	}
	return s.listItems, s.listCursor, nil
}
func (s controlledService) ListArchived(_ context.Context, authorization, cursor string, limit int) ([]domain.Entity, string, error) {
	return s.List(context.Background(), authorization, cursor, limit)
}
func (s controlledService) ListProjected(ctx context.Context, authorization, cursor string, limit int) ([]pathapp.Projection, string, error) {
	if s.listProjections != nil {
		if authorization != "Bearer valid-application-session" {
			return nil, "", ports.ErrInvalidCredential
		}
		return s.listProjections, s.listCursor, s.dependencyError
	}
	entities, nextCursor, err := s.List(ctx, authorization, cursor, limit)
	if err != nil {
		return nil, "", err
	}
	projections := make([]pathapp.Projection, 0, len(entities))
	for _, entity := range entities {
		projections = append(projections, pathapp.Projection{Path: entity, Capabilities: s.capabilities})
	}
	return projections, nextCursor, nil
}
func (s controlledService) ListArchivedProjected(ctx context.Context, authorization, cursor string, limit int) ([]pathapp.Projection, string, error) {
	return s.ListProjected(ctx, authorization, cursor, limit)
}
func (s controlledService) ReadHomePreferences(_ context.Context, authorization string) (domain.HomePreferences, error) {
	if authorization != "Bearer valid-application-session" {
		return domain.HomePreferences{}, ports.ErrInvalidCredential
	}
	return domain.HomePreferences{OrderMethod: domain.HomeOrderRecent}, s.dependencyError
}
func (s controlledService) UpdateHomePreferences(_ context.Context, authorization, _ string, expectedRevision int64, orderMethod domain.HomeOrderMethod, pinned, manual []domain.ID) (domain.HomePreferences, error) {
	if authorization != "Bearer valid-application-session" {
		return domain.HomePreferences{}, ports.ErrInvalidCredential
	}
	return domain.HomePreferences{OrderMethod: orderMethod, Revision: expectedRevision + 1, PinnedPathIDs: pinned, ManualPathIDs: manual, UpdatedAt: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)}, s.dependencyError
}
func (s controlledService) getEntity(id domain.ID) domain.Entity {
	return domain.Entity{ID: id, OwnerUserID: "creator", Attributes: domain.Attributes{Name: "Guitar", Visibility: "private"}}
}
func (s controlledService) Get(_ context.Context, authorization string, id domain.ID) (domain.Entity, error) {
	if authorization != "Bearer valid-application-session" {
		return domain.Entity{}, ports.ErrInvalidCredential
	}
	if s.dependencyError != nil {
		return domain.Entity{}, s.dependencyError
	}
	switch id {
	case "denied":
		return domain.Entity{}, platformapp.ErrForbidden
	case "missing":
		return domain.Entity{}, ports.ErrNotFound
	default:
		return s.getEntity(id), nil
	}
}
func (s controlledService) GetProjected(ctx context.Context, authorization string, id domain.ID) (pathapp.Projection, error) {
	entity, err := s.Get(ctx, authorization, id)
	if err != nil {
		return pathapp.Projection{}, err
	}
	recent := time.Date(2026, 8, 4, 11, 0, 0, 0, time.UTC)
	return pathapp.Projection{Path: entity, Capabilities: s.capabilities, Home: pathapp.HomeOrganization{PathID: id, Classification: pathapp.HomeShared, RecentActivityAt: &recent}}, nil
}
func (s controlledService) Project(_ context.Context, authorization string, entity domain.Entity) (pathapp.Projection, error) {
	if authorization != "Bearer valid-application-session" {
		return pathapp.Projection{}, ports.ErrInvalidCredential
	}
	if s.dependencyError != nil {
		return pathapp.Projection{}, s.dependencyError
	}
	return pathapp.Projection{Path: entity, Capabilities: s.capabilities}, nil
}
func (controlledService) Update(context.Context, string, domain.ID, domain.Attributes) (domain.Entity, error) {
	return domain.Entity{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (s controlledService) Rename(_ context.Context, authorization, idempotencyKey string, id domain.ID, expectedName, name string) (pathapp.RenameResult, error) {
	if s.renameCall != nil {
		s.renameCall.authorization = authorization
		s.renameCall.idempotencyKey = idempotencyKey
		s.renameCall.id = id
		s.renameCall.expectedName = expectedName
		s.renameCall.name = name
	}
	return s.renameResult, s.renameError
}
func (controlledService) UpdateGoals(context.Context, string, string, domain.ID, bool, domain.GoalConfiguration, domain.IntervalGoal, domain.OverallTarget) (pathapp.UpdateGoalsResult, error) {
	return pathapp.UpdateGoalsResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) SetArchiveState(context.Context, string, string, domain.ID, bool, bool, bool) (pathapp.SetArchiveStateResult, error) {
	return pathapp.SetArchiveStateResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) SetVisibility(context.Context, string, string, domain.ID, bool, string, string) (pathapp.SetVisibilityResult, error) {
	return pathapp.SetVisibilityResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (s controlledService) Delete(_ context.Context, authorization, idempotencyKey string, id domain.ID, confirmed bool, expectedName string) (pathapp.DeletePathResult, error) {
	if s.deleteCall != nil {
		s.deleteCall.authorization, s.deleteCall.idempotencyKey, s.deleteCall.id, s.deleteCall.confirmed, s.deleteCall.expectedName = authorization, idempotencyKey, id, confirmed, expectedName
	}
	return s.deleteResult, s.deleteError
}
func (controlledService) Leave(context.Context, string, string, domain.ID, bool, bool) (pathapp.LeavePathResult, error) {
	return pathapp.LeavePathResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) ReviewMemberRemoval(context.Context, string, domain.ID, string) (pathapp.MemberRemovalReview, error) {
	return pathapp.MemberRemovalReview{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) RemoveMember(context.Context, string, string, domain.ID, string, bool, domain.MembershipRole) (pathapp.RemoveMemberResult, error) {
	return pathapp.RemoveMemberResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) ChangeMemberRole(context.Context, string, string, domain.ID, string, bool, domain.MembershipRole, domain.MembershipRole) (pathapp.ChangeMemberRoleResult, error) {
	return pathapp.ChangeMemberRoleResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) ListMembers(context.Context, string, domain.ID, string, int) ([]pathapp.Member, string, error) {
	return nil, "", ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) Send(context.Context, string, string, domain.ID, string, string, domain.MembershipRole) (pathapp.SendInvitationResult, error) {
	return pathapp.SendInvitationResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) ReviewRecipient(context.Context, string, domain.ID, string) (pathapp.InvitationRecipientReview, error) {
	return pathapp.InvitationRecipientReview{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) ListPending(context.Context, string, string, int) ([]pathapp.PendingInvitation, string, error) {
	return nil, "", ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) ListManagedPending(context.Context, string, domain.ID, string, int) ([]pathapp.ManagedInvitation, string, error) {
	return nil, "", ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) CancelInvitation(context.Context, string, string, domain.ID, domain.InvitationID) (pathapp.CancelInvitationResult, error) {
	return pathapp.CancelInvitationResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) ListNotifications(context.Context, string, string, int) ([]pathapp.InvitationNotificationProjection, string, int64, error) {
	return nil, "", 0, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) GetNotification(context.Context, string, string) (pathapp.InvitationNotificationProjection, error) {
	return pathapp.InvitationNotificationProjection{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) MarkNotificationRead(context.Context, string, string) (pathapp.NotificationMutationResult, error) {
	return pathapp.NotificationMutationResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) DeleteNotification(context.Context, string, string) (pathapp.NotificationMutationResult, error) {
	return pathapp.NotificationMutationResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) MarkAllNotificationsRead(context.Context, string) (pathapp.NotificationMutationResult, error) {
	return pathapp.NotificationMutationResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) Accept(context.Context, string, string, domain.InvitationID) (pathapp.AcceptInvitationResult, error) {
	return pathapp.AcceptInvitationResult{}, ports.ErrAuthorizationPolicyNotConfigured
}
func (controlledService) Reject(context.Context, string, string, domain.InvitationID) (pathapp.RejectInvitationResult, error) {
	return pathapp.RejectInvitationResult{}, ports.ErrAuthorizationPolicyNotConfigured
}

func pathHandler(service Service) http.Handler {
	handler, _ := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, service) }}})
	return handler
}

func performGet(handler http.Handler, id, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/v1/paths/"+id, nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func performList(handler http.Handler, authorization string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/v1/paths?cursor=next&limit=25", nil)
	if authorization != "" {
		request.Header.Set("Authorization", authorization)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func performCreate(handler http.Handler, body, authorization, idempotencyKey string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, "/v1/paths", strings.NewReader(body))
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

func TestPathCreateHTTPAcceptsNameOnlyAndReturnsCreatedPath(t *testing.T) {
	call := &struct {
		authorization, idempotencyKey string
		attributes                    domain.Attributes
	}{}
	created := domain.Entity{ID: "created-path", OwnerUserID: "member", Attributes: domain.Attributes{Name: "Guitar", Visibility: "followers"}}
	response := performCreate(pathHandler(controlledService{createEntity: created, createCall: call}), `{"name":"Guitar"}`, "Bearer valid-application-session", "path-create-key-0001")
	var body struct {
		Data struct{ ID, Name, Visibility string } `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusCreated || body.Data.ID != "created-path" || body.Data.Name != "Guitar" || body.Data.Visibility != "followers" {
		t.Fatalf("create response status=%d body=%s", response.Code, response.Body.String())
	}
	if call.authorization != "Bearer valid-application-session" || call.idempotencyKey != "path-create-key-0001" || call.attributes != (domain.Attributes{Name: "Guitar"}) {
		t.Fatalf("create call = %+v", call)
	}
}

func TestPathCreateHTTPPassesExplicitVisibilityAndMapsFailuresSafely(t *testing.T) {
	call := &struct {
		authorization, idempotencyKey string
		attributes                    domain.Attributes
	}{}
	created := domain.Entity{ID: "private-path", OwnerUserID: "member", Attributes: domain.Attributes{Name: "Read", Visibility: "private"}}
	response := performCreate(pathHandler(controlledService{createEntity: created, createCall: call}), `{"name":"Read","visibility":"private"}`, "Bearer valid-application-session", "path-create-key-0002")
	if response.Code != http.StatusCreated || call.attributes.Visibility != "private" {
		t.Fatalf("explicit visibility status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}

	for _, test := range []struct {
		name, body, authorization, key string
		serviceError                   error
		want                           int
	}{
		{"missing session", `{"name":"Read"}`, "", "path-create-key-0002", nil, http.StatusUnauthorized},
		{"malformed session", `{"name":"Read"}`, "Bearer malformed", "path-create-key-0002", nil, http.StatusUnauthorized},
		{"missing idempotency", `{"name":"Read"}`, "Bearer valid-application-session", "", nil, http.StatusUnprocessableEntity},
		{"missing name", `{}`, "Bearer valid-application-session", "path-create-key-0002", nil, http.StatusUnprocessableEntity},
		{"invalid visibility", `{"name":"Read","visibility":"friends"}`, "Bearer valid-application-session", "path-create-key-0002", nil, http.StatusUnprocessableEntity},
		{"service validation", `{"name":"Read"}`, "Bearer valid-application-session", "path-create-key-0002", ports.ErrInvalidArgument, http.StatusBadRequest},
		{"dependency failure", `{"name":"Read"}`, "Bearer valid-application-session", "path-create-key-0002", errors.New("database unavailable: secret detail"), http.StatusInternalServerError},
	} {
		t.Run(test.name, func(t *testing.T) {
			failed := performCreate(pathHandler(controlledService{createError: test.serviceError}), test.body, test.authorization, test.key)
			if failed.Code != test.want {
				t.Fatalf("status=%d want=%d body=%s", failed.Code, test.want, failed.Body.String())
			}
			payload, err := io.ReadAll(failed.Body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(payload), "secret detail") {
				t.Fatalf("dependency detail leaked: %s", payload)
			}
		})
	}
}

func TestPathCreateHTTPMapsOptionalGoalsAndReturnsExactJSON(t *testing.T) {
	tests := []struct {
		name     string
		request  string
		want     domain.Attributes
		response string
	}{
		{name: "neither", request: `{"name":"Read"}`, want: domain.Attributes{Name: "Read"}, response: `{"data":{"id":"created","name":"Read","visibility":"followers"}}`},
		{name: "overall only", request: `{"name":"Read","overallTarget":{"targetSeconds":3600}}`, want: domain.Attributes{Name: "Read", OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 3_600}}, response: `{"data":{"id":"created","name":"Read","visibility":"followers","overallTarget":{"targetSeconds":3600}}}`},
		{name: "hourly omitted alignment", request: `{"name":"Read","intervalGoal":{"targetSeconds":60,"recurrence":"hourly"}}`, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly}}, response: `{"data":{"id":"created","name":"Read","visibility":"followers","intervalGoal":{"targetSeconds":60,"recurrence":"hourly","alignment":{"minute":0}}}}`},
		{name: "hourly", request: `{"name":"Read","intervalGoal":{"targetSeconds":60,"recurrence":"hourly","alignment":{"minute":30}}}`, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 60, Recurrence: domain.RecurrenceHourly, Alignment: domain.GoalAlignment{Minute: 30}}}, response: `{"data":{"id":"created","name":"Read","visibility":"followers","intervalGoal":{"targetSeconds":60,"recurrence":"hourly","alignment":{"minute":30}}}}`},
		{name: "daily", request: `{"name":"Read","intervalGoal":{"targetSeconds":600,"recurrence":"daily","alignment":{"hour":5}}}`, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 600, Recurrence: domain.RecurrenceDaily, Alignment: domain.GoalAlignment{Hour: 5}}}, response: `{"data":{"id":"created","name":"Read","visibility":"followers","intervalGoal":{"targetSeconds":600,"recurrence":"daily","alignment":{"hour":5}}}}`},
		{name: "weekly", request: `{"name":"Read","intervalGoal":{"targetSeconds":3600,"recurrence":"weekly","alignment":{"isoWeekday":1}}}`, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 3_600, Recurrence: domain.RecurrenceWeekly, Alignment: domain.GoalAlignment{ISOWeekday: 1}}}, response: `{"data":{"id":"created","name":"Read","visibility":"followers","intervalGoal":{"targetSeconds":3600,"recurrence":"weekly","alignment":{"isoWeekday":1}}}}`},
		{name: "monthly", request: `{"name":"Read","intervalGoal":{"targetSeconds":7200,"recurrence":"monthly","alignment":{"day":31}}}`, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 7_200, Recurrence: domain.RecurrenceMonthly, Alignment: domain.GoalAlignment{Day: 31}}}, response: `{"data":{"id":"created","name":"Read","visibility":"followers","intervalGoal":{"targetSeconds":7200,"recurrence":"monthly","alignment":{"day":31}}}}`},
		{name: "yearly and overall", request: `{"name":"Read","intervalGoal":{"targetSeconds":36000,"recurrence":"yearly","alignment":{"month":2,"day":29}},"overallTarget":{"targetSeconds":360000}}`, want: domain.Attributes{Name: "Read", IntervalGoal: domain.IntervalGoal{Present: true, TargetSeconds: 36_000, Recurrence: domain.RecurrenceYearly, Alignment: domain.GoalAlignment{Month: 2, Day: 29}}, OverallTarget: domain.OverallTarget{Present: true, TargetSeconds: 360_000}}, response: `{"data":{"id":"created","name":"Read","visibility":"followers","intervalGoal":{"targetSeconds":36000,"recurrence":"yearly","alignment":{"month":2,"day":29}},"overallTarget":{"targetSeconds":360000}}}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			call := &struct {
				authorization, idempotencyKey string
				attributes                    domain.Attributes
			}{}
			created := domain.Entity{ID: "created", OwnerUserID: "member", Attributes: test.want}
			created.Visibility = "followers"
			capabilities := pathapp.Capabilities{TrackTime: true, InviteMembers: true, ManageMembers: true, ManageGoals: true, ManageLifecycle: true, TransferOwnership: true}
			response := performCreate(pathHandler(controlledService{createEntity: created, createCall: call, capabilities: capabilities}), test.request, "Bearer valid-application-session", "path-create-goal-0001")
			projectedResponse := strings.TrimSuffix(test.response, "}}") + `,"capabilities":{"trackTime":true,"renamePath":false,"inviteMembers":true,"manageMembers":true,"manageGoals":true,"manageLifecycle":true,"manageVisibility":false,"transferOwnership":true,"leavePath":false}}}`
			exactResponse := strings.Replace(projectedResponse, `{"data":`, `{"$schema":"https://example.com/schemas/PathProjectionOutputBody.json","data":`, 1)
			if response.Code != http.StatusCreated || call.attributes != test.want || strings.TrimSpace(response.Body.String()) != exactResponse {
				t.Fatalf("status=%d call=%+v body=%s; want call=%+v body=%s", response.Code, call, response.Body.String(), test.want, exactResponse)
			}
		})
	}
}

func TestPathCreateHTTPRejectsMalformedGoalPresenceBeforeService(t *testing.T) {
	for _, body := range []string{
		`{"name":"Read","intervalGoal":{"targetSeconds":1,"recurrence":"hourly","alignment":{}}}`,
		`{"name":"Read","intervalGoal":{"targetSeconds":1,"recurrence":"hourly","alignment":{"hour":1}}}`,
		`{"name":"Read","intervalGoal":{"targetSeconds":1,"recurrence":"weekly","alignment":{"isoWeekday":1,"day":1}}}`,
		`{"name":"Read","intervalGoal":{"targetSeconds":1,"recurrence":"yearly","alignment":{"month":2}}}`,
	} {
		call := &struct {
			authorization, idempotencyKey string
			attributes                    domain.Attributes
		}{}
		response := performCreate(pathHandler(controlledService{createCall: call}), body, "Bearer valid-application-session", "path-create-goal-0001")
		if response.Code != http.StatusBadRequest || call.authorization != "" {
			t.Fatalf("malformed goal status=%d call=%+v body=%s", response.Code, call, response.Body.String())
		}
	}
}

func TestPathGoalOpenAPIRequiresCanonicalResponseAlignmentOnly(t *testing.T) {
	_, api := shared.New(platformapp.App{}, nil, shared.Options{DomainRegistrations: []func(huma.API){func(api huma.API) { Register(api, controlledService{}) }}})
	schemas := api.OpenAPI().Components.Schemas.Map()
	requestGoal := schemas["IntervalGoalCreate"]
	responseGoal := schemas["IntervalGoalItem"]
	if requestGoal == nil || responseGoal == nil {
		t.Fatalf("goal schemas missing: %v", schemas)
	}
	requires := func(schema *huma.Schema, field string) bool {
		for _, required := range schema.Required {
			if required == field {
				return true
			}
		}
		return false
	}
	if requires(requestGoal, "alignment") {
		t.Fatal("create alignment must remain optional so omission requests profile defaults")
	}
	if !requires(responseGoal, "alignment") {
		t.Fatal("canonical interval-goal responses must require alignment")
	}
}

func TestPathListHTTPReturnsNonNullEmptyCollection(t *testing.T) {
	call := &struct {
		authorization, cursor string
		limit                 int
	}{}
	response := performList(pathHandler(controlledService{listCall: call}), "Bearer valid-application-session")
	var body struct {
		Data []domain.Entity `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK || body.Data == nil || len(body.Data) != 0 || body.Meta.NextCursor != "" {
		t.Fatalf("empty list status=%d body=%s", response.Code, response.Body.String())
	}
	if call.authorization != "Bearer valid-application-session" || call.cursor != "next" || call.limit != 25 {
		t.Fatalf("list call = %+v", call)
	}
	defaultCall := &struct {
		authorization, cursor string
		limit                 int
	}{}
	defaultRequest := httptest.NewRequest(http.MethodGet, "/v1/paths", nil)
	defaultRequest.Header.Set("Authorization", "Bearer valid-application-session")
	defaultResponse := httptest.NewRecorder()
	pathHandler(controlledService{listCall: defaultCall}).ServeHTTP(defaultResponse, defaultRequest)
	if defaultResponse.Code != http.StatusOK || defaultCall.limit != 25 {
		t.Fatalf("default list status=%d call=%+v body=%s", defaultResponse.Code, defaultCall, defaultResponse.Body.String())
	}

	for _, authorization := range []string{"", "Bearer malformed", "Bearer oidc.header.payload"} {
		denied := performList(pathHandler(controlledService{}), authorization)
		if denied.Code != http.StatusUnauthorized {
			t.Fatalf("credential %q status=%d body=%s", authorization, denied.Code, denied.Body.String())
		}
	}

	failure := performList(pathHandler(controlledService{dependencyError: errors.New("database unavailable")}), "Bearer valid-application-session")
	if failure.Code != http.StatusInternalServerError {
		t.Fatalf("dependency failure status=%d body=%s", failure.Code, failure.Body.String())
	}
}

func TestPathDetailHTTPAuthorizationBoundary(t *testing.T) {
	handler := pathHandler(controlledService{})
	for _, authorization := range []string{"", "Bearer malformed", "Bearer oidc.header.payload"} {
		response := performGet(handler, "private-path", authorization)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("credential %q status=%d body=%s", authorization, response.Code, response.Body.String())
		}
	}
	response := performGet(handler, "private-path", "Bearer valid-application-session")
	var body struct {
		Data struct{ ID, Name, Visibility string } `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK || body.Data.ID != "private-path" || body.Data.Name != "Guitar" || body.Data.Visibility != "private" {
		t.Fatalf("legitimate response status=%d body=%s", response.Code, response.Body.String())
	}

	denied := performGet(handler, "denied", "Bearer valid-application-session")
	missing := performGet(handler, "missing", "Bearer valid-application-session")
	if denied.Code != http.StatusNotFound || missing.Code != http.StatusNotFound || denied.Body.String() != missing.Body.String() {
		t.Fatalf("denied and missing responses differ: denied=%d %q missing=%d %q", denied.Code, denied.Body.String(), missing.Code, missing.Body.String())
	}
}

func TestPathDetailHTTPDependencyFailureIsNotAuthenticationOrAbsence(t *testing.T) {
	dependencyError := errors.New("SpiceDB unavailable")
	response := performGet(pathHandler(controlledService{dependencyError: dependencyError}), "private-path", "Bearer valid-application-session")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("dependency failure status=%d body=%s", response.Code, response.Body.String())
	}
}
