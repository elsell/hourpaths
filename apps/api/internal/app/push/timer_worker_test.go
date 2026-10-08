package pushapp

import (
	"context"
	"errors"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type timerPushAuthorization struct {
	allowed bool
	err     error
}

func (a timerPushAuthorization) Check(_ context.Context, kind, id, permission, user string) (bool, error) {
	if kind != "path" || id != "path-1" || permission != "view" || user != "recipient-1" {
		return false, errors.New("unexpected authorization target")
	}
	return a.allowed, a.err
}
func (timerPushAuthorization) WriteRelationship(context.Context, string, string, string, string, string) error {
	return errors.New("unexpected write")
}
func (timerPushAuthorization) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return errors.New("unexpected delete")
}
func TestTimerPushRequiresCurrentAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name    string
		auth    ports.Authorizer
		sent    bool
		failure bool
	}{
		{"allowed", timerPushAuthorization{allowed: true}, true, false},
		{"revoked", timerPushAuthorization{}, false, false},
		{"dependency unavailable", timerPushAuthorization{err: errors.New("unavailable")}, false, true},
		{"missing dependency", nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repository := &pushWorkerRepository{claims: []ports.PushDelivery{pushWorkerDelivery(1, "")}}
			provider := &pushWorkerProvider{ticket: ports.PushTicket{State: ports.PushDelivered}}
			projection := pushWorkerProjection(pathapp.NotificationTimerStarted, pathapp.NotificationInformational)
			projection.InvitationID = ""
			projection.OfferedRole = ""
			worker := testPushWorker(time.Now().UTC(), repository, provider, &pushWorkerNotifications{item: projection})
			worker.Authorizer = tc.auth
			_, err := worker.RunOnce(context.Background())
			if (err != nil) != tc.failure || (len(provider.messages) == 1) != tc.sent {
				t.Fatalf("error=%v messages=%+v", err, provider.messages)
			}
			if tc.sent && provider.messages[0].Body != "Alex started tracking on Lectura." {
				t.Fatalf("message=%+v", provider.messages[0])
			}
			if !tc.sent && !tc.failure && (len(repository.transitions) != 1 || repository.transitions[0].Outcome != ports.PushDeliverySuppressed) {
				t.Fatalf("not retired: %+v", repository.transitions)
			}
		})
	}
}
