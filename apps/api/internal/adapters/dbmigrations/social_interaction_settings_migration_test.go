package dbmigrations

import (
	"strings"
	"testing"
)

func TestSocialInteractionSettingsMigrationDefaultsIndependentControlsAndDurableReplay(t *testing.T) {
	up, err := Files.ReadFile("000051_social_interaction_settings.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sql := string(up)
	for _, required := range []string{
		"comments_enabled boolean NOT NULL DEFAULT true",
		"reactions_enabled boolean NOT NULL DEFAULT true",
		"social_interaction_setting_replay_models",
		"social.interaction_settings.update",
		"GRANT SELECT, INSERT, UPDATE ON public.social_interaction_setting_models TO app",
		"GRANT SELECT, INSERT ON public.social_interaction_setting_replay_models TO app",
		"interaction_disabled_reason IN ('comments', 'reactions')",
		"GRANT DELETE ON public.notification_push_outbox_models TO app",
		"GRANT DELETE ON public.notification_push_delivery_models TO app",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{"GRANT DELETE ON public.social_interaction_setting_models", "GRANT UPDATE ON public.social_interaction_setting_replay_models"} {
		if strings.Contains(sql, forbidden) {
			t.Fatalf("migration grants forbidden capability %q", forbidden)
		}
	}
	down, err := Files.ReadFile("000051_social_interaction_settings.down.sql")
	if err != nil || !strings.Contains(string(down), "cannot remove social interaction settings while configured data exists") || !strings.Contains(string(down), "interaction_disabled_reason IS NOT NULL") {
		t.Fatalf("unsafe down migration: %v", err)
	}
}
