package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

type removalHTTPCall struct {
	authorization, key, target string
	pathID                     domain.ID
	confirmed                  bool
	role                       domain.MembershipRole
}
type removalHTTPService struct {
	controlledService
	review  pathapp.MemberRemovalReview
	command *removalHTTPCall
}

func (s removalHTTPService) ListMembers(context.Context, string, domain.ID, string, int) ([]pathapp.Member, string, error) {
	return []pathapp.Member{{UserID: "target", Username: "reader", DisplayName: "Reader", Role: "participant", SessionCount: 2, TotalTrackedSeconds: 3600, IntervalProgress: &pathapp.GoalProgress{AccumulatedSeconds: 1200, TargetSeconds: 1800}, OverallProgress: &pathapp.GoalProgress{AccumulatedSeconds: 3600, TargetSeconds: 7200}, BlockedByViewer: true, CanRemove: true, CanChangeRole: true, CanGrantAdministrator: true}}, "next", nil
}

func (s removalHTTPService) ReviewMemberRemoval(context.Context, string, domain.ID, string) (pathapp.MemberRemovalReview, error) {
	return s.review, nil
}
func (s removalHTTPService) RemoveMember(_ context.Context, authorization, key string, pathID domain.ID, target string, confirmed bool, role domain.MembershipRole) (pathapp.RemoveMemberResult, error) {
	*s.command = removalHTTPCall{authorization, key, target, pathID, confirmed, role}
	return pathapp.RemoveMemberResult{PathID: pathID, UserID: target, Removed: true, ActivityDeleted: role == domain.RoleParticipant}, nil
}
func (s removalHTTPService) ChangeMemberRole(_ context.Context, authorization, key string, pathID domain.ID, target string, confirmed bool, expectedRole, role domain.MembershipRole) (pathapp.ChangeMemberRoleResult, error) {
	*s.command = removalHTTPCall{authorization, key, target, pathID, confirmed, expectedRole}
	return pathapp.ChangeMemberRoleResult{PathID: pathID, UserID: target, Role: role, ActivityDeleted: expectedRole == domain.RoleParticipant && role == domain.RoleSupporter}, nil
}

func TestMemberRemovalHTTPReviewAndConfirmedMutation(t *testing.T) {
	command := &removalHTTPCall{}
	service := removalHTTPService{review: pathapp.MemberRemovalReview{UserID: "target", Username: "reader", DisplayName: "Reader", Role: domain.RoleParticipant, SessionCount: 2, TotalTrackedSeconds: 3600, RunningTimer: true}, command: command}
	handler := pathHandler(service)
	listRequest := httptest.NewRequest(http.MethodGet, "/v1/paths/path-1/members?limit=25", nil)
	listRequest.Header.Set("Authorization", "Bearer app")
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"username":"reader"`) || !strings.Contains(listResponse.Body.String(), `"sessionCount":2`) || !strings.Contains(listResponse.Body.String(), `"totalTrackedSeconds":3600`) || !strings.Contains(listResponse.Body.String(), `"intervalProgress":{"accumulatedSeconds":1200,"targetSeconds":1800}`) || !strings.Contains(listResponse.Body.String(), `"overallProgress":{"accumulatedSeconds":3600,"targetSeconds":7200}`) || !strings.Contains(listResponse.Body.String(), `"blockedByViewer":true`) || !strings.Contains(listResponse.Body.String(), `"canRemove":true`) || !strings.Contains(listResponse.Body.String(), `"canChangeRole":true`) || !strings.Contains(listResponse.Body.String(), `"canGrantAdministrator":true`) || !strings.Contains(listResponse.Body.String(), `"nextCursor":"next"`) {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	reviewRequest := httptest.NewRequest(http.MethodGet, "/v1/paths/path-1/members/target/removal-review", nil)
	reviewRequest.Header.Set("Authorization", "Bearer app")
	reviewResponse := httptest.NewRecorder()
	handler.ServeHTTP(reviewResponse, reviewRequest)
	if reviewResponse.Code != http.StatusOK || !strings.Contains(reviewResponse.Body.String(), `"totalTrackedSeconds":3600`) || !strings.Contains(reviewResponse.Body.String(), `"runningTimer":true`) {
		t.Fatalf("review status=%d body=%s", reviewResponse.Code, reviewResponse.Body.String())
	}
	request := httptest.NewRequest(http.MethodDelete, "/v1/paths/path-1/members/target", strings.NewReader(`{"confirmed":true,"expectedRole":"participant"}`))
	request.Header.Set("Authorization", "Bearer app")
	request.Header.Set("Idempotency-Key", "remove-member-key-0001")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var body struct {
		Data struct{ Removed, ActivityDeleted bool } `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || *command != (removalHTTPCall{"Bearer app", "remove-member-key-0001", "target", "path-1", true, domain.RoleParticipant}) || !body.Data.Removed || !body.Data.ActivityDeleted {
		t.Fatalf("status=%d command=%+v body=%s", response.Code, command, response.Body.String())
	}
	roleRequest := httptest.NewRequest(http.MethodPatch, "/v1/paths/path-1/members/target", strings.NewReader(`{"confirmed":true,"expectedRole":"participant","role":"supporter"}`))
	roleRequest.Header.Set("Authorization", "Bearer app")
	roleRequest.Header.Set("Idempotency-Key", "change-member-role-0001")
	roleRequest.Header.Set("Content-Type", "application/json")
	roleResponse := httptest.NewRecorder()
	handler.ServeHTTP(roleResponse, roleRequest)
	if roleResponse.Code != http.StatusOK || !strings.Contains(roleResponse.Body.String(), `"role":"supporter"`) || !strings.Contains(roleResponse.Body.String(), `"activityDeleted":true`) {
		t.Fatalf("role status=%d body=%s", roleResponse.Code, roleResponse.Body.String())
	}
}

func TestAdministratorRoleMutationHTTPContract(t *testing.T) {
	command := &removalHTTPCall{}
	handler := pathHandler(removalHTTPService{command: command})
	request := httptest.NewRequest(http.MethodPatch, "/v1/paths/path-1/members/target", strings.NewReader(`{"confirmed":true,"expectedRole":"participant","role":"administrator"}`))
	request.Header.Set("Authorization", "Bearer app")
	request.Header.Set("Idempotency-Key", "administrator-role-0001")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"role":"administrator"`) || !strings.Contains(response.Body.String(), `"activityDeleted":false`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
