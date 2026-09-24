package social

import (
	"context"
	"testing"
	"time"
)

type controlledInteractionSettings struct {
	settings InteractionSettings
	command  *InteractionSettingsCommand
}

func (store *controlledInteractionSettings) GetInteractionSettings(context.Context, string) (InteractionSettings, error) {
	return store.settings, nil
}
func (store *controlledInteractionSettings) UpdateInteractionSettings(_ context.Context, command InteractionSettingsCommand) (InteractionSettingsResult, error) {
	store.command = &command
	store.settings = command.Settings
	return InteractionSettingsResult{Settings: command.Settings}, nil
}

func TestInteractionSettingsReadDefaultsAndIndependentIdempotentUpdate(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	store := &controlledInteractionSettings{settings: InteractionSettings{CommentsEnabled: true, ReactionsEnabled: true}}
	service := testService(&controlledProfiles{}, &controlledAudits{})
	service.InteractionSettings = store
	service.Clock = fixedClock{now: now}
	got, err := service.GetInteractionSettings(context.Background(), "Bearer session")
	if err != nil || got != store.settings {
		t.Fatalf("settings=%+v err=%v", got, err)
	}
	want := InteractionSettings{CommentsEnabled: false, ReactionsEnabled: true}
	got, err = service.UpdateInteractionSettings(context.Background(), "Bearer session", "interaction-key-01", want)
	if err != nil || got != want || store.command == nil || store.command.ActorUserID != "viewer" || store.command.Settings != want || store.command.Idempotency.Operation != UpdateInteractionSettingsOperation || !store.command.Audit.Valid() {
		t.Fatalf("settings=%+v command=%+v err=%v", got, store.command, err)
	}
}
