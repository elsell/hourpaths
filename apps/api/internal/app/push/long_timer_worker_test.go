package pushapp

import (
	"context"
	"errors"
	"testing"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestLongTimerPushRequiresOwnTimerAndCurrentAccess(t *testing.T) {
	for _, tc := range []struct {
		name, actor, locale, body string
		auth                      ports.Authorizer
		failure                   bool
	}{
		{"English", "recipient-1", "en", "Your timer on Lectura is still running. Is it time to stop?", timerPushAuthorization{allowed: true}, false},
		{"Spanish", "recipient-1", "es", "Tu temporizador en Lectura sigue en marcha. ¿Es hora de detenerlo?", timerPushAuthorization{allowed: true}, false},
		{"foreign timer", "actor-1", "en", "", timerPushAuthorization{allowed: true}, false},
		{"revoked access", "recipient-1", "en", "", timerPushAuthorization{}, false},
		{"authorization unavailable", "recipient-1", "en", "", timerPushAuthorization{err: errors.New("unavailable")}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			delivery := pushWorkerDelivery(1, "")
			delivery.Locale = tc.locale
			repository := &pushWorkerRepository{claims: []ports.PushDelivery{delivery}}
			provider := &pushWorkerProvider{ticket: ports.PushTicket{State: ports.PushDelivered}}
			projection := pushWorkerProjection(pathapp.NotificationLongTimerRunning, pathapp.NotificationInformational)
			projection.InvitationID = ""
			projection.OfferedRole = ""
			projection.Actor.UserID = tc.actor
			worker := testPushWorker(time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC), repository, provider, &pushWorkerNotifications{item: projection})
			worker.Authorizer = tc.auth
			_, err := worker.RunOnce(context.Background())
			if (err != nil) != tc.failure {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.body != "" {
				if len(provider.messages) != 1 || provider.messages[0].Body != tc.body {
					t.Fatalf("messages: %+v", provider.messages)
				}
			} else {
				if len(provider.messages) != 0 {
					t.Fatalf("unauthorized delivery: %+v", provider.messages)
				}
				if !tc.failure && (len(repository.transitions) != 1 || repository.transitions[0].Outcome != ports.PushDeliverySuppressed) {
					t.Fatalf("not suppressed: %+v", repository.transitions)
				}
			}
		})
	}
}
