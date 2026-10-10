package routes

import (
	"context"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type reminderHTTPIdentity struct{}

func (reminderHTTPIdentity) Authenticate(_ context.Context, header string) (ports.Principal, error) {
	if header == "Bearer owner" || header == "Bearer foreign" {
		return ports.Principal{UserID: strings.TrimPrefix(header, "Bearer "), Scopes: []string{"api:user"}}, nil
	}
	return ports.Principal{}, ports.ErrInvalidCredential
}

type reminderHTTPStore struct {
	pathapp.InvitationRepository
	item pathapp.InvitationNotificationProjection
}

func (r reminderHTTPStore) GetNotification(_ context.Context, owner, id string) (pathapp.InvitationNotificationProjection, error) {
	if owner != "owner" || id != r.item.ID {
		return pathapp.InvitationNotificationProjection{}, ports.ErrNotFound
	}
	return r.item, nil
}

type reminderHTTPAccess struct {
	ports.Authorizer
	deny, unavailable bool
}

func (a reminderHTTPAccess) Check(_ context.Context, kind, id, permission, user string) (bool, error) {
	if a.unavailable {
		return false, ports.ErrUnavailable
	}
	return !a.deny && kind == "path" && permission == "view" && user == "owner" && (id == "reading" || id == "music"), nil
}

type reminderHTTPAudit struct {
	ports.Audits
	events []audit.Event
}

func (a *reminderHTTPAudit) AppendAuditEvent(_ context.Context, event audit.Event) error {
	a.events = append(a.events, event)
	return nil
}

type reminderHTTPClock struct{ at time.Time }

func (c reminderHTTPClock) Now() time.Time { return c.at }

type reminderHTTPLimiter struct{}

func (reminderHTTPLimiter) Allow(string, time.Time) bool { return true }

func TestGoalReminderHTTPResolutionUsesCapabilityAndCurrentOwnerAccess(t *testing.T) {
	at := time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	item := pathapp.InvitationNotificationProjection{ID: "notice", Kind: pathapp.NotificationGoalPracticeReminder, Presentation: pathapp.NotificationInformational, CreatedAt: at, Actor: pathapp.InvitationPublicIdentity{UserID: "owner", Username: "reader", DisplayName: "Reader"}, Reminder: &pathapp.GoalReminderBundle{Paths: []pathapp.GoalReminderPath{{ID: "reading", Name: "Private Reading"}, {ID: "music", Name: "Private Music"}}}}
	for _, c := range []struct {
		name, header, query string
		access              reminderHTTPAccess
		status              int
	}{
		{"legitimate", "Bearer owner", "?goalReminders=true", reminderHTTPAccess{}, 200},
		{"old-client", "Bearer owner", "", reminderHTTPAccess{}, 404},
		{"foreign", "Bearer foreign", "?goalReminders=true", reminderHTTPAccess{}, 404},
		{"missing-auth", "", "?goalReminders=true", reminderHTTPAccess{}, 401},
		{"rejected-session", "Bearer expired", "?goalReminders=true", reminderHTTPAccess{}, 401},
		{"denied", "Bearer owner", "?goalReminders=true", reminderHTTPAccess{deny: true}, 404},
		{"dependency-failed", "Bearer owner", "?goalReminders=true", reminderHTTPAccess{unavailable: true}, 503},
	} {
		t.Run(c.name, func(t *testing.T) {
			journal := &reminderHTTPAudit{}
			service := pathapp.NewInvitationService(pathapp.InvitationDependencies{Auth: reminderHTTPIdentity{}, Invitations: reminderHTTPStore{item: item}, Authorizer: c.access, Audits: journal, Clock: reminderHTTPClock{at}, AuditRateLimiter: reminderHTTPLimiter{}})
			handler, _ := invitationHandler(service)
			request := httptest.NewRequest(http.MethodGet, "/v1/notifications/notice"+c.query, nil)
			request.Header.Set("Authorization", c.header)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != c.status {
				t.Fatalf("status %d body %s", response.Code, response.Body)
			}
			for _, name := range []string{"Private Reading", "Private Music"} {
				if strings.Contains(response.Body.String(), name) != (c.status == 200) {
					t.Fatalf("incorrect subject exposure: %s", response.Body)
				}
			}
			if c.status == 200 && !strings.Contains(response.Body.String(), `"reminder":{"paths":[`) {
				t.Fatalf("missing bundle: %s", response.Body)
			}
		})
	}
}
