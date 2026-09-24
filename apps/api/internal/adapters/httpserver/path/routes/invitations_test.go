package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type invitationHTTPCall struct {
	authorization, idempotencyKey, cursor, username, expectedRecipientUserID string
	pathID                                                                   domain.ID
	invitationID                                                             domain.InvitationID
	role                                                                     domain.MembershipRole
	acknowledgement                                                          *pathapp.InvitationWarningAcknowledgement
	limit                                                                    int
}

type controlledInvitationHTTPService struct {
	call          *invitationHTTPCall
	recipient     pathapp.InvitationRecipientReview
	send          pathapp.SendInvitationResult
	pending       []pathapp.PendingInvitation
	managed       []pathapp.ManagedInvitation
	notifications []pathapp.InvitationNotificationProjection
	mutation      pathapp.NotificationMutationResult
	nextCursor    string
	unreadCount   int64
	accept        pathapp.AcceptInvitationResult
	reject        pathapp.RejectInvitationResult
	cancel        pathapp.CancelInvitationResult
	err           error
}

func (service controlledInvitationHTTPService) ListManagedPending(_ context.Context, authorization string, pathID domain.ID, cursor string, limit int) ([]pathapp.ManagedInvitation, string, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, pathID: pathID, cursor: cursor, limit: limit}
	}
	return service.managed, service.nextCursor, service.err
}

func (service controlledInvitationHTTPService) CancelInvitation(_ context.Context, authorization, key string, pathID domain.ID, invitationID domain.InvitationID) (pathapp.CancelInvitationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, idempotencyKey: key, pathID: pathID, invitationID: invitationID}
	}
	return service.cancel, service.err
}

func (service controlledInvitationHTTPService) MarkNotificationRead(_ context.Context, authorization, notificationID string) (pathapp.NotificationMutationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, invitationID: domain.InvitationID(notificationID)}
	}
	return service.mutation, service.err
}

func (service controlledInvitationHTTPService) DeleteNotification(_ context.Context, authorization, notificationID string) (pathapp.NotificationMutationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, invitationID: domain.InvitationID(notificationID)}
	}
	return service.mutation, service.err
}

func (service controlledInvitationHTTPService) MarkAllNotificationsRead(_ context.Context, authorization string) (pathapp.NotificationMutationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization}
	}
	return service.mutation, service.err
}

func (service controlledInvitationHTTPService) GetNotification(
	_ context.Context,
	_, notificationID string,
) (pathapp.InvitationNotificationProjection, error) {
	for _, notification := range service.notifications {
		if notification.ID == notificationID {
			return notification, service.err
		}
	}
	return pathapp.InvitationNotificationProjection{}, ports.ErrNotFound
}

func TestGetNotificationRouteReturnsOneOpaqueResolvedProjection(t *testing.T) {
	now := time.Date(2026, time.July, 24, 1, 15, 0, 0, time.UTC)
	handler, api := invitationHandler(controlledInvitationHTTPService{
		notifications: []pathapp.InvitationNotificationProjection{{
			ID: "notification-1", Kind: pathapp.NotificationPathInvitationReceived,
			Presentation: pathapp.NotificationActionable, CreatedAt: now,
			Actor: pathapp.InvitationPublicIdentity{
				UserID: "creator", Username: "Practice.Owner", DisplayName: "Practice Owner",
			},
			PathID: "path-1", PathName: "Violin", InvitationID: "invitation-1",
			OfferedRole: domain.RoleSupporter,
		}},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(
		http.MethodGet, "/v1/notifications/notification-1", "", "",
	))
	var body struct {
		Data struct {
			ID, Type, Presentation, PathID, InvitationID string
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil ||
		response.Code != http.StatusOK ||
		body.Data.ID != "notification-1" ||
		body.Data.Type != "path_invitation_received" ||
		body.Data.Presentation != "actionable" ||
		body.Data.PathID != "path-1" ||
		body.Data.InvitationID != "invitation-1" {
		t.Fatalf("response=%d body=%s error=%v", response.Code, response.Body.String(), err)
	}
	path := api.OpenAPI().Paths["/v1/notifications/{notificationId}"]
	if path == nil || path.Get == nil || path.Get.OperationID != "get-notification" {
		t.Fatalf("single notification operation = %+v", path)
	}
}

func TestGetNotificationRouteMapsPreEligibilityNotFoundToOpaque404(t *testing.T) {
	handler, _ := invitationHandler(controlledInvitationHTTPService{
		notifications: []pathapp.InvitationNotificationProjection{{ID: "pre-eligibility-notification"}},
		err:           ports.ErrNotFound,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(
		http.MethodGet, "/v1/notifications/pre-eligibility-notification", "", "",
	))
	if response.Code != http.StatusNotFound ||
		!strings.Contains(response.Body.String(), `"code":"not_found"`) ||
		strings.Contains(response.Body.String(), "pre-eligibility-notification") {
		t.Fatalf("pre-eligibility response=%d body=%s", response.Code, response.Body.String())
	}
}

func (service controlledInvitationHTTPService) ReviewRecipient(_ context.Context, authorization string, pathID domain.ID, username string) (pathapp.InvitationRecipientReview, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, pathID: pathID, username: username}
	}
	return service.recipient, service.err
}

func (service controlledInvitationHTTPService) Send(_ context.Context, authorization, key string, pathID domain.ID, username, expectedRecipientUserID string, role domain.MembershipRole) (pathapp.SendInvitationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, idempotencyKey: key, pathID: pathID, username: username, expectedRecipientUserID: expectedRecipientUserID, role: role}
	}
	return service.send, service.err
}

func (service controlledInvitationHTTPService) ListPending(_ context.Context, authorization, cursor string, limit int) ([]pathapp.PendingInvitation, string, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, cursor: cursor, limit: limit}
	}
	return service.pending, service.nextCursor, service.err
}

func (service controlledInvitationHTTPService) ListNotifications(_ context.Context, authorization, cursor string, limit int) ([]pathapp.InvitationNotificationProjection, string, int64, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, cursor: cursor, limit: limit}
	}
	return service.notifications, service.nextCursor, service.unreadCount, service.err
}

func (service controlledInvitationHTTPService) Accept(_ context.Context, authorization, key string, invitationID domain.InvitationID) (pathapp.AcceptInvitationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, idempotencyKey: key, invitationID: invitationID}
	}
	return service.accept, service.err
}

func (service controlledInvitationHTTPService) Reject(_ context.Context, authorization, key string, invitationID domain.InvitationID) (pathapp.RejectInvitationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{authorization: authorization, idempotencyKey: key, invitationID: invitationID}
	}
	return service.reject, service.err
}

func (service controlledInvitationHTTPService) AcceptConfirmed(
	_ context.Context,
	authorization, key string,
	invitationID domain.InvitationID,
	acknowledgement pathapp.InvitationWarningAcknowledgement,
) (pathapp.AcceptInvitationResult, error) {
	if service.call != nil {
		*service.call = invitationHTTPCall{
			authorization: authorization, idempotencyKey: key, invitationID: invitationID,
			acknowledgement: &acknowledgement,
		}
	}
	return service.accept, service.err
}

func invitationHandler(service InvitationService) (http.Handler, huma.API) {
	return shared.New(platformapp.App{}, nil, shared.Options{
		DomainRegistrations: []func(huma.API){func(api huma.API) { RegisterInvitations(api, service) }},
	})
}

func invitationRequest(method, target, body, key string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer application-session")
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		request.Header.Set("Idempotency-Key", key)
	}
	return request
}

func TestPathInvitationRoutesSendListAndAcceptThroughDistinctContract(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	pending := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
		RecipientUserID: "recipient", OfferedRole: domain.RoleSupporter, CreatedAt: now,
	}
	accepted := pending
	accepted.AcceptedAt = now.Add(time.Minute)

	sendCall := &invitationHTTPCall{}
	handler, _ := invitationHandler(controlledInvitationHTTPService{
		call: sendCall, send: pathapp.SendInvitationResult{Invitation: pending},
	})
	sendResponse := httptest.NewRecorder()
	handler.ServeHTTP(sendResponse, invitationRequest(
		http.MethodPost, "/v1/paths/path-1/invitations",
		`{"username":"reader","expectedRecipientUserId":"recipient","offeredRole":"supporter"}`, "send-invite-key-01",
	))
	if sendResponse.Code != http.StatusCreated ||
		*sendCall != (invitationHTTPCall{
			authorization: "Bearer application-session", idempotencyKey: "send-invite-key-01",
			pathID: "path-1", username: "reader", expectedRecipientUserID: "recipient", role: domain.RoleSupporter,
		}) {
		t.Fatalf("send status=%d call=%+v body=%s", sendResponse.Code, sendCall, sendResponse.Body.String())
	}

	listCall := &invitationHTTPCall{}
	handler, _ = invitationHandler(controlledInvitationHTTPService{
		call: listCall, pending: []pathapp.PendingInvitation{{
			Invitation: pending, PathName: "Morning Reading",
			Inviter: pathapp.InvitationPublicIdentity{
				UserID: "creator", Username: "Book.Owner", DisplayName: "Book Owner",
			},
			Warning: &pathapp.InvitationWarningContext{
				PathVisibility: "followers", HasRetainedActivity: true,
			},
		}}, nextCursor: "signed-next-page",
	})
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, invitationRequest(
		http.MethodGet, "/v1/path-invitations?cursor=signed-current-page&limit=50", "", "",
	))
	var listBody struct {
		Data []struct {
			Invitation struct {
				ID, PathID, InviterUserID, RecipientUserID, OfferedRole string
				CreatedAt                                               time.Time
				AcceptedAt                                              *time.Time
			} `json:"invitation"`
			PathName string `json:"pathName"`
			Warning  *struct {
				PathVisibility      string `json:"pathVisibility"`
				HasRetainedActivity bool   `json:"hasRetainedActivity"`
			} `json:"warning"`
			Inviter struct {
				UserID, Username, DisplayName string
			} `json:"inviter"`
		} `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(listResponse.Body.Bytes(), &listBody); err != nil ||
		listResponse.Code != http.StatusOK || len(listBody.Data) != 1 ||
		listBody.Data[0].Invitation.ID != "invitation-1" || listBody.Data[0].Invitation.OfferedRole != "supporter" ||
		listBody.Data[0].Invitation.AcceptedAt != nil || listBody.Data[0].PathName != "Morning Reading" ||
		listBody.Data[0].Warning == nil ||
		listBody.Data[0].Warning.PathVisibility != "followers" ||
		!listBody.Data[0].Warning.HasRetainedActivity ||
		listBody.Data[0].Inviter.UserID != "creator" || listBody.Data[0].Inviter.Username != "Book.Owner" ||
		listBody.Data[0].Inviter.DisplayName != "Book Owner" ||
		listBody.Meta.NextCursor != "signed-next-page" ||
		*listCall != (invitationHTTPCall{
			authorization: "Bearer application-session", cursor: "signed-current-page", limit: 50,
		}) {
		t.Fatalf("list status=%d call=%+v body=%s error=%v", listResponse.Code, listCall, listResponse.Body.String(), err)
	}

	acceptCall := &invitationHTTPCall{}
	handler, _ = invitationHandler(controlledInvitationHTTPService{
		call: acceptCall, accept: pathapp.AcceptInvitationResult{Invitation: accepted, Replayed: true},
	})
	acceptResponse := httptest.NewRecorder()
	handler.ServeHTTP(acceptResponse, invitationRequest(
		http.MethodPost, "/v1/path-invitations/invitation-1/accept", "", "accept-invite-key1",
	))
	if acceptResponse.Code != http.StatusOK ||
		*acceptCall != (invitationHTTPCall{
			authorization: "Bearer application-session", idempotencyKey: "accept-invite-key1",
			invitationID: "invitation-1",
		}) {
		t.Fatalf("accept status=%d call=%+v body=%s", acceptResponse.Code, acceptCall, acceptResponse.Body.String())
	}
	var acceptBody struct {
		Data struct {
			AcceptedAt *time.Time `json:"acceptedAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(acceptResponse.Body.Bytes(), &acceptBody); err != nil ||
		acceptBody.Data.AcceptedAt == nil || !acceptBody.Data.AcceptedAt.Equal(accepted.AcceptedAt) {
		t.Fatalf("accept body=%s error=%v", acceptResponse.Body.String(), err)
	}

	rejected := pending
	rejected.RejectedAt = now.Add(2 * time.Minute)
	rejectCall := &invitationHTTPCall{}
	handler, _ = invitationHandler(controlledInvitationHTTPService{
		call: rejectCall, reject: pathapp.RejectInvitationResult{Invitation: rejected, UnreadCount: 3},
	})
	rejectResponse := httptest.NewRecorder()
	handler.ServeHTTP(rejectResponse, invitationRequest(
		http.MethodPost, "/v1/path-invitations/invitation-1/reject", "", "reject-invite-key",
	))
	var rejectBody struct {
		Data struct {
			InvitationID string    `json:"invitationId"`
			RejectedAt   time.Time `json:"rejectedAt"`
			UnreadCount  int64     `json:"unreadCount"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rejectResponse.Body.Bytes(), &rejectBody); err != nil ||
		rejectResponse.Code != http.StatusOK || rejectBody.Data.InvitationID != "invitation-1" ||
		!rejectBody.Data.RejectedAt.Equal(rejected.RejectedAt) || rejectBody.Data.UnreadCount != 3 ||
		*rejectCall != (invitationHTTPCall{authorization: "Bearer application-session", idempotencyKey: "reject-invite-key", invitationID: "invitation-1"}) {
		t.Fatalf("reject status=%d call=%+v body=%s error=%v", rejectResponse.Code, rejectCall, rejectResponse.Body.String(), err)
	}
}

func TestPathInvitationAcceptRoutesOptionalVisibilityAcknowledgement(t *testing.T) {
	call := &invitationHTTPCall{}
	handler, _ := invitationHandler(controlledInvitationHTTPService{call: call})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(
		http.MethodPost, "/v1/path-invitations/invitation-1/accept",
		`{"visibilityWarningAcknowledgement":{"pathVisibility":"public"}}`,
		"accept-invite-key1",
	))
	if response.Code != http.StatusOK ||
		call.acknowledgement == nil ||
		call.acknowledgement.PathVisibility != "public" {
		t.Fatalf("status=%d call=%+v body=%s", response.Code, call, response.Body.String())
	}
}

func TestPathInvitationAcceptRejectsInvalidVisibilityAcknowledgementBeforeService(t *testing.T) {
	for _, body := range []string{
		`{"visibilityWarningAcknowledgement":{"pathVisibility":"private"}}`,
		`{"visibilityWarningAcknowledgement":{"pathVisibility":"friends"}}`,
		`{"visibilityWarningAcknowledgement":{}}`,
	} {
		t.Run(body, func(t *testing.T) {
			call := &invitationHTTPCall{}
			handler, _ := invitationHandler(controlledInvitationHTTPService{call: call})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, invitationRequest(
				http.MethodPost, "/v1/path-invitations/invitation-1/accept",
				body, "accept-invite-key1",
			))
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if *call != (invitationHTTPCall{}) {
				t.Fatalf("invalid acknowledgement reached service: %+v", call)
			}
		})
	}
}

func TestPathInvitationNotificationRouteReturnsRecipientProjection(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	call := &invitationHTTPCall{}
	handler, _ := invitationHandler(controlledInvitationHTTPService{
		call: call,
		notifications: []pathapp.InvitationNotificationProjection{{
			ID: "notification-1", Kind: pathapp.NotificationPathInvitationAccepted,
			Presentation: pathapp.NotificationInformational, Read: true, CreatedAt: now,
			Actor: pathapp.InvitationPublicIdentity{
				UserID: "recipient", Username: "Reader.One", DisplayName: "Reader One",
			},
			PathID: "path-1", PathName: "Morning Reading", InvitationID: "invitation-1",
			OfferedRole: domain.RoleParticipant,
		}},
		nextCursor:  "signed-next-page",
		unreadCount: 7,
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(
		http.MethodGet, "/v1/notifications?cursor=signed-current-page&limit=50", "", "",
	))
	var body struct {
		Data []struct {
			ID, Type, Presentation, PathID, PathName, InvitationID, OfferedRole string
			Read, Actionable                                                    bool
			CreatedAt                                                           time.Time
			Actor                                                               struct {
				UserID, Username, DisplayName string
			}
		} `json:"data"`
		Meta struct {
			NextCursor  string `json:"nextCursor"`
			UnreadCount int64  `json:"unreadCount"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil ||
		response.Code != http.StatusOK || len(body.Data) != 1 ||
		body.Data[0].ID != "notification-1" ||
		body.Data[0].Type != "path_invitation_accepted" ||
		body.Data[0].Presentation != "informational" ||
		!body.Data[0].Read || body.Data[0].PathID != "path-1" ||
		body.Data[0].PathName != "Morning Reading" ||
		body.Data[0].InvitationID != "invitation-1" ||
		body.Data[0].OfferedRole != "participant" ||
		body.Data[0].Actor.UserID != "recipient" ||
		body.Data[0].Actor.Username != "Reader.One" ||
		body.Data[0].Actor.DisplayName != "Reader One" ||
		body.Meta.NextCursor != "signed-next-page" || body.Meta.UnreadCount != 7 ||
		*call != (invitationHTTPCall{
			authorization: "Bearer application-session", cursor: "signed-current-page", limit: 50,
		}) {
		t.Fatalf("notification list status=%d call=%+v body=%s error=%v", response.Code, call, response.Body.String(), err)
	}
	if strings.Contains(response.Body.String(), "recipientUserId") ||
		strings.Contains(response.Body.String(), "email") ||
		strings.Contains(response.Body.String(), "profileVisibility") {
		t.Fatalf("notification projection leaks private identity: %s", response.Body.String())
	}
}

func TestPracticeReactionNotificationRouteReturnsExactPublicContext(t *testing.T) {
	now := time.Date(2026, time.July, 28, 12, 0, 0, 0, time.UTC)
	handler, _ := invitationHandler(controlledInvitationHTTPService{
		call: &invitationHTTPCall{},
		notifications: []pathapp.InvitationNotificationProjection{{
			ID: "reaction-notice", Kind: pathapp.NotificationPracticeReaction,
			Presentation: pathapp.NotificationInformational, CreatedAt: now,
			Actor:             pathapp.InvitationPublicIdentity{UserID: "reactor", Username: "reader", DisplayName: "Reader"},
			PathID:            "path-1",
			PathName:          "Piano",
			SocialFeedEventID: "practice:activity-1",
			Reaction:          socialdomain.ReactionFire,
		}},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(http.MethodGet, "/v1/notifications", "", ""))
	var body struct {
		Data []struct {
			Type, PathID, PathName, SocialFeedEventID, Reaction string
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != http.StatusOK || len(body.Data) != 1 ||
		body.Data[0].Type != "practice_reaction" || body.Data[0].PathID != "path-1" || body.Data[0].PathName != "Piano" ||
		body.Data[0].SocialFeedEventID != "practice:activity-1" || body.Data[0].Reaction != "fire" {
		t.Fatalf("status=%d body=%s error=%v", response.Code, response.Body.String(), err)
	}
	for _, forbidden := range []string{"invitationId", "ownershipTransferId", "followRequestId", "offeredRole"} {
		if strings.Contains(response.Body.String(), `"`+forbidden+`"`) {
			t.Fatalf("reaction notification leaked %s: %s", forbidden, response.Body.String())
		}
	}
}

func TestPathDeletionNotificationRouteReturnsSnapshotWithoutDeletedPathTarget(t *testing.T) {
	now := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	handler, _ := invitationHandler(controlledInvitationHTTPService{
		call: &invitationHTTPCall{},
		notifications: []pathapp.InvitationNotificationProjection{{
			ID: "deletion-notice", Kind: pathapp.NotificationPathDeleted,
			Presentation: pathapp.NotificationInformational, CreatedAt: now,
			Actor: pathapp.InvitationPublicIdentity{
				UserID: "creator", Username: "Creator.One", DisplayName: "Creator One",
			},
			PathName: "Former Practice",
		}},
	})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, invitationRequest(http.MethodGet, "/v1/notifications", "", ""))
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || len(body.Data) != 1 {
		t.Fatalf("body=%s error=%v", response.Body.String(), err)
	}
	notice := body.Data[0]
	if notice["type"] != "path_deleted" || notice["presentation"] != "informational" ||
		notice["pathName"] != "Former Practice" {
		t.Fatalf("deletion notice = %#v", notice)
	}
	for _, forbidden := range []string{"pathId", "invitationId", "ownershipTransferId"} {
		if _, exists := notice[forbidden]; exists {
			t.Fatalf("deletion notice exposed %s: %#v", forbidden, notice)
		}
	}
}

func TestNotificationMutationRoutesReturnResultingUnreadCount(t *testing.T) {
	for _, test := range []struct {
		name, method, target string
		wantCall             invitationHTTPCall
	}{
		{
			name: "mark read", method: http.MethodPatch, target: "/v1/notifications/notification-1/read",
			wantCall: invitationHTTPCall{authorization: "Bearer application-session", invitationID: "notification-1"},
		},
		{
			name: "delete", method: http.MethodDelete, target: "/v1/notifications/notification-1",
			wantCall: invitationHTTPCall{authorization: "Bearer application-session", invitationID: "notification-1"},
		},
		{
			name: "mark all", method: http.MethodPost, target: "/v1/notifications/read-all",
			wantCall: invitationHTTPCall{authorization: "Bearer application-session"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			call := &invitationHTTPCall{}
			handler, _ := invitationHandler(controlledInvitationHTTPService{
				call: call, mutation: pathapp.NotificationMutationResult{UnreadCount: 3},
			})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, invitationRequest(test.method, test.target, "", ""))
			var body struct {
				Data struct {
					UnreadCount int64 `json:"unreadCount"`
				} `json:"data"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil ||
				response.Code != http.StatusOK || body.Data.UnreadCount != 3 ||
				*call != test.wantCall {
				t.Fatalf("status=%d call=%+v body=%s error=%v", response.Code, call, response.Body.String(), err)
			}
		})
	}
}

func TestPathInvitationContractValidatesRoleAndIdempotencyBeforeService(t *testing.T) {
	for _, test := range []struct {
		name, method, target, body, key string
	}{
		{"missing send key", http.MethodPost, "/v1/paths/path-1/invitations", `{"username":"reader","expectedRecipientUserId":"recipient","offeredRole":"participant"}`, ""},
		{"short send key", http.MethodPost, "/v1/paths/path-1/invitations", `{"username":"reader","expectedRecipientUserId":"recipient","offeredRole":"participant"}`, "short"},
		{"unsupported role", http.MethodPost, "/v1/paths/path-1/invitations", `{"username":"reader","expectedRecipientUserId":"recipient","offeredRole":"administrator"}`, "send-invite-key-01"},
		{"missing username", http.MethodPost, "/v1/paths/path-1/invitations", `{"expectedRecipientUserId":"recipient","offeredRole":"participant"}`, "send-invite-key-01"},
		{"missing reviewed recipient", http.MethodPost, "/v1/paths/path-1/invitations", `{"username":"reader","offeredRole":"participant"}`, "send-invite-key-01"},
		{"missing accept key", http.MethodPost, "/v1/path-invitations/invitation-1/accept", "", ""},
		{"missing reject key", http.MethodPost, "/v1/path-invitations/invitation-1/reject", "", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			call := &invitationHTTPCall{}
			handler, _ := invitationHandler(controlledInvitationHTTPService{call: call})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, invitationRequest(test.method, test.target, test.body, test.key))
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
			if *call != (invitationHTTPCall{}) {
				t.Fatalf("invalid request reached service: %+v", call)
			}
		})
	}
}

func TestPathInvitationOpaqueUnavailableAndWarningErrorsHaveStableCodes(t *testing.T) {
	opaqueResponses := make([]*httptest.ResponseRecorder, 0, 3)
	for _, inaccessible := range []error{
		domain.ErrInvitationUnavailable,
		platformapp.ErrForbidden,
		ports.ErrNotFound,
	} {
		handler, _ := invitationHandler(controlledInvitationHTTPService{err: inaccessible})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, invitationRequest(
			http.MethodPost, "/v1/path-invitations/secret/accept", "", "accept-invite-key1",
		))
		opaqueResponses = append(opaqueResponses, response)
	}
	for _, response := range opaqueResponses {
		if response.Code != http.StatusNotFound ||
			!bytes.Equal(response.Body.Bytes(), opaqueResponses[0].Body.Bytes()) {
			t.Fatalf("opaque responses differ: %#v", opaqueResponses)
		}
	}
	handler, _ := invitationHandler(controlledInvitationHTTPService{err: ports.ErrNotFound})
	reassigned := httptest.NewRecorder()
	handler.ServeHTTP(reassigned, invitationRequest(
		http.MethodPost, "/v1/paths/path-1/invitations",
		`{"username":"reader","expectedRecipientUserId":"reviewed-user","offeredRole":"participant"}`,
		"send-invite-key-01",
	))
	if reassigned.Code != http.StatusNotFound ||
		!strings.Contains(reassigned.Body.String(), `"code":"not_found"`) ||
		strings.Contains(reassigned.Body.String(), "reviewed-user") ||
		strings.Contains(reassigned.Body.String(), "reader") {
		t.Fatalf("reassigned recipient was not opaque: status=%d body=%s", reassigned.Code, reassigned.Body.String())
	}

	handler, _ = invitationHandler(controlledInvitationHTTPService{
		err: &pathapp.InvitationWarningRequiredError{Context: pathapp.InvitationWarningContext{
			PathVisibility: "public", HasRetainedActivity: true,
		}},
	})
	warning := httptest.NewRecorder()
	handler.ServeHTTP(warning, invitationRequest(
		http.MethodPost, "/v1/path-invitations/invitation-1/accept", "", "accept-invite-key1",
	))
	if warning.Code != http.StatusConflict ||
		!strings.Contains(warning.Body.String(), `"code":"invitation_warning_required"`) ||
		strings.Contains(warning.Body.String(), "public") ||
		strings.Contains(warning.Body.String(), "retained") {
		t.Fatalf("warning status=%d body=%s", warning.Code, warning.Body.String())
	}

	handler, _ = invitationHandler(controlledInvitationHTTPService{err: errors.New("database secret")})
	failure := httptest.NewRecorder()
	handler.ServeHTTP(failure, invitationRequest(
		http.MethodPost, "/v1/path-invitations/invitation-1/accept", "", "accept-invite-key1",
	))
	if failure.Code != http.StatusInternalServerError || strings.Contains(failure.Body.String(), "database secret") {
		t.Fatalf("failure status=%d body=%s", failure.Code, failure.Body.String())
	}
}

func TestPathInvitationRejectOpaqueUnavailableErrorsAreByteIdentical(t *testing.T) {
	var baseline []byte
	for _, inaccessible := range []error{
		domain.ErrInvitationUnavailable,
		platformapp.ErrForbidden,
		ports.ErrNotFound,
	} {
		handler, _ := invitationHandler(controlledInvitationHTTPService{err: inaccessible})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, invitationRequest(
			http.MethodPost, "/v1/path-invitations/secret/reject", "", "reject-invite-key",
		))
		if response.Code != http.StatusNotFound {
			t.Fatalf("reject opaque status=%d body=%s", response.Code, response.Body.String())
		}
		if baseline == nil {
			baseline = append([]byte(nil), response.Body.Bytes()...)
		} else if !bytes.Equal(response.Body.Bytes(), baseline) {
			t.Fatalf("reject opaque bodies differ: %q != %q", response.Body.Bytes(), baseline)
		}
	}
}

func TestPathInvitationOpenAPIUsesDistinctPathsStrictRolesAndReplayHeaders(t *testing.T) {
	_, api := invitationHandler(controlledInvitationHTTPService{})
	openapi := api.OpenAPI()
	send := openapi.Paths["/v1/paths/{pathId}/invitations"].Post
	list := openapi.Paths["/v1/path-invitations"].Get
	accept := openapi.Paths["/v1/path-invitations/{invitationId}/accept"].Post
	reject := openapi.Paths["/v1/path-invitations/{invitationId}/reject"].Post
	notifications := openapi.Paths["/v1/notifications"].Get
	if send == nil || send.OperationID != "send-path-invitation" ||
		list == nil || list.OperationID != "list-pending-path-invitations" ||
		accept == nil || accept.OperationID != "accept-path-invitation" ||
		reject == nil || reject.OperationID != "reject-path-invitation" ||
		notifications == nil || notifications.OperationID != "list-notifications" {
		t.Fatalf("invitation operations missing: send=%+v list=%+v accept=%+v reject=%+v notifications=%+v", send, list, accept, reject, notifications)
	}
	for name, operation := range map[string]*huma.Operation{"send": send, "accept": accept, "reject": reject} {
		found := false
		for _, parameter := range operation.Parameters {
			if parameter.Name == "Idempotency-Key" && parameter.Required &&
				parameter.Schema.MinLength != nil && *parameter.Schema.MinLength == 16 &&
				parameter.Schema.MaxLength != nil && *parameter.Schema.MaxLength == 128 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s operation lacks strict Idempotency-Key", name)
		}
	}
	role := openapi.Components.Schemas.Map()["PathInvitationCreate"].Properties["offeredRole"]
	if len(role.Enum) != 2 || role.Enum[0] != "participant" || role.Enum[1] != "supporter" {
		t.Fatalf("offered-role enum = %#v", role.Enum)
	}
	createSchema := openapi.Components.Schemas.Map()["PathInvitationCreate"]
	expectedRecipient := createSchema.Properties["expectedRecipientUserId"]
	requiredExpectedRecipient := false
	for _, required := range createSchema.Required {
		requiredExpectedRecipient = requiredExpectedRecipient || required == "expectedRecipientUserId"
	}
	if expectedRecipient == nil || expectedRecipient.MinLength == nil ||
		*expectedRecipient.MinLength != 1 || !requiredExpectedRecipient {
		t.Fatalf("reviewed recipient binding is not required: %+v", createSchema)
	}
	listSchema := openapi.Components.Schemas.Map()["PathInvitationListOutputBody"]
	if listSchema == nil || listSchema.Properties["data"].Type != huma.TypeArray ||
		listSchema.Properties["data"].Nullable {
		t.Fatalf("pending invitation list data must be a non-null array: %+v", listSchema)
	}
	if listSchema.Properties["data"].Items == nil ||
		listSchema.Properties["data"].Items.Ref != "#/components/schemas/PendingPathInvitation" {
		t.Fatalf("pending list must use its distinct context projection: %+v", listSchema.Properties["data"])
	}
	notificationListSchema := openapi.Components.Schemas.Map()["NotificationListOutputBody"]
	notificationSchema := openapi.Components.Schemas.Map()["PathInvitationNotification"]
	notificationMetaSchema := openapi.Components.Schemas.Map()["NotificationListMeta"]
	if notificationListSchema == nil ||
		notificationListSchema.Properties["data"].Type != huma.TypeArray ||
		notificationListSchema.Properties["data"].Nullable ||
		notificationListSchema.Properties["data"].Items.Ref != "#/components/schemas/PathInvitationNotification" ||
		notificationSchema == nil ||
		notificationSchema.Properties["actor"].Ref != "#/components/schemas/PathInvitationPublicIdentity" ||
		notificationListSchema.Properties["meta"].Ref != "#/components/schemas/NotificationListMeta" ||
		notificationMetaSchema == nil ||
		notificationMetaSchema.Properties["unreadCount"] == nil ||
		notificationMetaSchema.Properties["unreadCount"].Minimum == nil ||
		*notificationMetaSchema.Properties["unreadCount"].Minimum != 0 ||
		notificationSchema.Properties["recipientUserId"] != nil ||
		notificationSchema.Properties["email"] != nil ||
		notificationSchema.Properties["profileVisibility"] != nil {
		t.Fatalf("notification list contract leaks or omits projection: list=%+v item=%+v", notificationListSchema, notificationSchema)
	}
	deletionType := false
	for _, value := range notificationSchema.Properties["type"].Enum {
		deletionType = deletionType || value == "path_deleted"
	}
	pathIDRequired := false
	for _, field := range notificationSchema.Required {
		pathIDRequired = pathIDRequired || field == "pathId"
	}
	if !deletionType || pathIDRequired {
		t.Fatalf("notification contract must include standalone path deletion notices: type=%#v required=%#v",
			notificationSchema.Properties["type"].Enum, notificationSchema.Required)
	}
	unreadRequired := false
	for _, field := range notificationMetaSchema.Required {
		unreadRequired = unreadRequired || field == "unreadCount"
	}
	if !unreadRequired {
		t.Fatalf("notification list unread count is not required: %+v", notificationMetaSchema)
	}
	pendingSchema := openapi.Components.Schemas.Map()["PendingPathInvitation"]
	warningSchema := openapi.Components.Schemas.Map()["PathInvitationVisibilityWarning"]
	acceptInputSchema := openapi.Components.Schemas.Map()["PathInvitationAccept"]
	identitySchema := openapi.Components.Schemas.Map()["PathInvitationPublicIdentity"]
	if pendingSchema == nil || pendingSchema.Properties["invitation"].Ref != "#/components/schemas/PathInvitation" ||
		pendingSchema.Properties["pathName"] == nil ||
		pendingSchema.Properties["inviter"].Ref != "#/components/schemas/PathInvitationPublicIdentity" ||
		pendingSchema.Properties["warning"].Ref != "#/components/schemas/PathInvitationVisibilityWarning" ||
		warningSchema == nil ||
		len(warningSchema.Properties["pathVisibility"].Enum) != 2 ||
		warningSchema.Properties["pathVisibility"].Enum[0] != "followers" ||
		warningSchema.Properties["pathVisibility"].Enum[1] != "public" ||
		acceptInputSchema == nil ||
		acceptInputSchema.Properties["visibilityWarningAcknowledgement"].Ref != "#/components/schemas/PathInvitationVisibilityWarningAcknowledgement" ||
		identitySchema == nil || len(identitySchema.Properties) != 3 ||
		identitySchema.Properties["userId"] == nil ||
		identitySchema.Properties["username"] == nil ||
		identitySchema.Properties["displayName"] == nil ||
		identitySchema.Properties["email"] != nil ||
		identitySchema.Properties["profileVisibility"] != nil {
		t.Fatalf("pending invitation context leaks or omits identity: pending=%+v identity=%+v", pendingSchema, identitySchema)
	}
	codeSchema := openapi.Components.Schemas.Map()["APIError"].Properties["code"]
	foundWarningCode := false
	for _, value := range codeSchema.Enum {
		if value == "invitation_warning_required" {
			foundWarningCode = true
		}
	}
	if !foundWarningCode {
		t.Fatalf("API error contract omits invitation_warning_required: %#v", codeSchema.Enum)
	}
}
