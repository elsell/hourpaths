package routes

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestPracticeReactionNotificationOpenAPIIsCuratedAndPathAddressable(t *testing.T) {
	_, openapi := invitationHandler(controlledInvitationHTTPService{})
	schema := openapi.OpenAPI().Components.Schemas.Map()["PathInvitationNotification"]
	if schema == nil || schema.Properties["pathId"] == nil || schema.Properties["pathName"] == nil || schema.Properties["socialFeedEventId"] == nil || schema.Properties["reaction"] == nil {
		t.Fatalf("practice reaction notification context is incomplete: %+v", schema)
	}
	hasType := false
	for _, value := range schema.Properties["type"].Enum {
		hasType = hasType || value == "practice_reaction"
	}
	if !hasType || !reflect.DeepEqual(schema.Properties["reaction"].Enum, []any{"heart", "applause", "fire", "strong", "celebrate"}) {
		t.Fatalf("reaction notification enums type=%#v reaction=%#v", schema.Properties["type"].Enum, schema.Properties["reaction"].Enum)
	}
}

func TestManagedPathInvitationRoutesExposeSafeListAndCancellation(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	invitation := domain.Invitation{ID: "invitation-1", PathID: "path-1", InviterUserID: "sender", RecipientUserID: "recipient", OfferedRole: domain.RoleSupporter, CreatedAt: now}
	managed := pathapp.ManagedInvitation{Invitation: invitation, Inviter: pathapp.InvitationPublicIdentity{UserID: "sender", Username: "sender", DisplayName: "Sender"}, Recipient: pathapp.InvitationPublicIdentity{UserID: "recipient", Username: "reader", DisplayName: "Reader"}}
	listCall := &invitationHTTPCall{}
	handler, api := invitationHandler(controlledInvitationHTTPService{call: listCall, managed: []pathapp.ManagedInvitation{managed}, nextCursor: "next"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(http.MethodGet, "/v1/paths/path-1/invitations?limit=10&cursor=current", "", ""))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"recipient":{"userId":"recipient","username":"reader","displayName":"Reader"}`) || strings.Contains(response.Body.String(), "email") || listCall.pathID != "path-1" || listCall.cursor != "current" || listCall.limit != 10 {
		t.Fatalf("managed list status=%d body=%s call=%+v", response.Code, response.Body.String(), listCall)
	}
	canceled := invitation
	canceled.CanceledAt = now.Add(time.Minute)
	cancelCall := &invitationHTTPCall{}
	handler, _ = invitationHandler(controlledInvitationHTTPService{call: cancelCall, cancel: pathapp.CancelInvitationResult{Invitation: canceled}})
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(http.MethodDelete, "/v1/paths/path-1/invitations/invitation-1", "", "cancel-key-000001"))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"invitationId":"invitation-1"`) || cancelCall.pathID != "path-1" || cancelCall.invitationID != "invitation-1" || cancelCall.idempotencyKey != "cancel-key-000001" {
		t.Fatalf("cancel status=%d body=%s call=%+v", response.Code, response.Body.String(), cancelCall)
	}
	listOperation := api.OpenAPI().Paths["/v1/paths/{pathId}/invitations"].Get
	cancelOperation := api.OpenAPI().Paths["/v1/paths/{pathId}/invitations/{invitationId}"].Delete
	if listOperation == nil || listOperation.OperationID != "list-managed-path-invitations" || cancelOperation == nil || cancelOperation.OperationID != "cancel-path-invitation" {
		t.Fatalf("operations list=%+v cancel=%+v", listOperation, cancelOperation)
	}
}
