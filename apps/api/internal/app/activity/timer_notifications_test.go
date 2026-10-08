package activity

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type notificationCandidateRepository struct {
	testRepository
	recipients []string
}

func (r notificationCandidateRepository) TimerNotificationCandidates(context.Context, string, string) ([]string, error) {
	return append([]string(nil), r.recipients...), nil
}

type timerRecipientAuthorizer struct {
	testAuthorizer
	permitted   map[string]bool
	unavailable string
}

func (a timerRecipientAuthorizer) Check(ctx context.Context, kind, id, permission, actor string) (bool, error) {
	if permission != "view" {
		return a.testAuthorizer.Check(ctx, kind, id, permission, actor)
	}
	if a.unavailable == actor {
		return false, ports.ErrUnavailable
	}
	return a.permitted[actor], nil
}

func TestTimerStartAuthorizesRecipientsBeforeAtomicNotificationAdmission(t *testing.T) {
	for _, unavailable := range []string{"", "denied-recipient"} {
		var starts []StartTimerCommand
		repository := notificationCandidateRepository{testRepository: testRepository{starts: &starts}, recipients: []string{"allowed-recipient", "denied-recipient"}}
		service := testService(repository)
		service.Authorizer = timerRecipientAuthorizer{testAuthorizer: testAuthorizer{allowed: true}, permitted: map[string]bool{"allowed-recipient": true}, unavailable: unavailable}
		_, err := service.StartTimer(context.Background(), "Bearer valid", "path-1", "authorized-timer-start")
		if unavailable != "" {
			if !errors.Is(err, ports.ErrUnavailable) || len(starts) != 0 {
				t.Fatalf("unavailable decision admitted start: %v %+v", err, starts)
			}
		} else if err != nil || len(starts) != 1 || !slices.Equal(starts[0].NotificationRecipients, []string{"allowed-recipient"}) {
			t.Fatalf("unauthorized recipient admitted: %v %+v", err, starts)
		}
	}
}
