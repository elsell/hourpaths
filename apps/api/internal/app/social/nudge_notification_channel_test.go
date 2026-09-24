package social

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledNudgeNotificationChannels struct {
	preference  NudgeNotificationChannelPreference
	found       bool
	err         error
	getCalls    int
	updateCalls int
	actor       string
	command     *NudgeNotificationChannelCommand
}

func (repository *controlledNudgeNotificationChannels) GetNudgeNotificationChannel(_ context.Context, actor string) (NudgeNotificationChannelPreference, bool, error) {
	repository.getCalls++
	repository.actor = actor
	return repository.preference, repository.found, repository.err
}

func (repository *controlledNudgeNotificationChannels) UpdateNudgeNotificationChannel(_ context.Context, command NudgeNotificationChannelCommand) (NudgeNotificationChannelMutationResult, error) {
	repository.updateCalls++
	repository.command = &command
	return NudgeNotificationChannelMutationResult{Preference: repository.preference}, repository.err
}

func nudgeNotificationChannelService(repository *controlledNudgeNotificationChannels) (*Service, *controlledAudits) {
	audits := &controlledAudits{}
	service := testService(&controlledProfiles{}, audits)
	service.NudgeNotificationChannels = repository
	service.Clock = fixedClock{now: time.Date(2026, 8, 5, 16, 0, 0, 0, time.UTC)}
	return service, audits
}

func TestNudgeNotificationChannelReadDefaultsEnabledWithoutWriting(t *testing.T) {
	repository := &controlledNudgeNotificationChannels{}
	service, audits := nudgeNotificationChannelService(repository)

	preference, err := service.GetNudgeNotificationChannel(context.Background(), "Bearer session")
	if err != nil || preference != (NudgeNotificationChannelPreference{Enabled: true}) {
		t.Fatalf("preference=%+v err=%v", preference, err)
	}
	if repository.actor != "viewer" || repository.getCalls != 1 || repository.updateCalls != 0 {
		t.Fatalf("repository=%+v", repository)
	}
	if len(audits.events) != 1 || audits.events[0].Action != audit.ResourceViewed || audits.events[0].TargetType != "notification_channel" || audits.events[0].TargetID != NudgeNotificationChannel {
		t.Fatalf("audits=%+v", audits.events)
	}
}

func TestNudgeNotificationChannelReadReturnsStoredPreference(t *testing.T) {
	want := NudgeNotificationChannelPreference{Enabled: false, Revision: 3}
	repository := &controlledNudgeNotificationChannels{preference: want, found: true}
	service, _ := nudgeNotificationChannelService(repository)

	got, err := service.GetNudgeNotificationChannel(context.Background(), "Bearer session")
	if err != nil || got != want || repository.updateCalls != 0 {
		t.Fatalf("preference=%+v repository=%+v err=%v", got, repository, err)
	}
}

func TestUpdateNudgeNotificationChannelBuildsAtomicRevisionedCommand(t *testing.T) {
	repository := &controlledNudgeNotificationChannels{preference: NudgeNotificationChannelPreference{Enabled: false, Revision: 5}}
	service, audits := nudgeNotificationChannelService(repository)

	got, err := service.UpdateNudgeNotificationChannel(context.Background(), "Bearer session", "nudge-channel-key", 4, false)
	if err != nil || got != repository.preference {
		t.Fatalf("preference=%+v err=%v", got, err)
	}
	command := repository.command
	if command == nil || command.ActorUserID != "viewer" || command.ExpectedRevision != 4 || command.Enabled || command.OccurredAt != service.Clock.Now() || command.Idempotency.PrincipalID != "viewer" || command.Idempotency.Operation != UpdateNudgeNotificationChannelOperation || command.Idempotency.Key != "nudge-channel-key" || len(command.Idempotency.RequestHash) != 32 {
		t.Fatalf("command=%+v", command)
	}
	if command.Audit.Action != audit.ResourceUpdated || command.Audit.TargetType != "notification_channel" || command.Audit.TargetID != NudgeNotificationChannel || command.Audit.OccurredAt != service.Clock.Now() {
		t.Fatalf("audit=%+v", command.Audit)
	}
	if len(audits.events) != 0 {
		t.Fatalf("update audit must be committed atomically by repository: %+v", audits.events)
	}
}

func TestNudgeNotificationChannelRejectsInvalidRequestsAndDependencies(t *testing.T) {
	for _, test := range []struct {
		name     string
		key      string
		revision int64
	}{
		{name: "short key", key: "short", revision: 0},
		{name: "negative revision", key: "nudge-channel-key", revision: -1},
	} {
		t.Run(test.name, func(t *testing.T) {
			repository := &controlledNudgeNotificationChannels{}
			service, _ := nudgeNotificationChannelService(repository)
			if _, err := service.UpdateNudgeNotificationChannel(context.Background(), "Bearer session", test.key, test.revision, true); !errors.Is(err, ports.ErrInvalidArgument) || repository.updateCalls != 0 {
				t.Fatalf("err=%v repository=%+v", err, repository)
			}
		})
	}

	repository := &controlledNudgeNotificationChannels{}
	service, _ := nudgeNotificationChannelService(repository)
	service.AuditRateLimiter = allowLimiter{}
	if _, err := service.GetNudgeNotificationChannel(context.Background(), "Bearer session"); !errors.Is(err, platformapp.ErrRateLimited) || repository.getCalls != 0 {
		t.Fatalf("err=%v repository=%+v", err, repository)
	}
}

func TestNudgeNotificationChannelRejectsMalformedRepositoryResults(t *testing.T) {
	for _, preference := range []NudgeNotificationChannelPreference{
		{Enabled: false, Revision: 0},
		{Enabled: true, Revision: 4},
	} {
		repository := &controlledNudgeNotificationChannels{preference: preference}
		service, _ := nudgeNotificationChannelService(repository)
		_, err := service.UpdateNudgeNotificationChannel(context.Background(), "Bearer session", "nudge-channel-key", 2, false)
		if !errors.Is(err, errInvalidDependencies) {
			t.Fatalf("preference=%+v err=%v", preference, err)
		}
	}
}

func TestNudgeNotificationChannelIdempotencyBindsRevisionAndValue(t *testing.T) {
	base := nudgeNotificationChannelIdempotency("viewer", "key", 2, true)
	for _, variant := range []ports.Idempotency{
		nudgeNotificationChannelIdempotency("viewer", "key", 3, true),
		nudgeNotificationChannelIdempotency("viewer", "key", 2, false),
	} {
		if string(base.RequestHash) == string(variant.RequestHash) {
			t.Fatalf("unbound digest: base=%x variant=%x", base.RequestHash, variant.RequestHash)
		}
	}
}
