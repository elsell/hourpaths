package dbmigrations_test

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/dbmigrations"
)

func TestNudgeMigrationPreservesPersonalAudienceRateAndDeliveryEvidence(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000064_social_nudges.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	for _, required := range []string{
		"CREATE TABLE public.path_nudge_preference_models",
		"CREATE FUNCTION public.enforce_path_nudge_preference_participant()",
		"membership.role IN ('administrator', 'participant')",
		"path.owner_user_id = NEW.user_id",
		"CREATE FUNCTION public.remove_ineligible_path_nudge_preference()",
		"AFTER DELETE OR UPDATE OF role ON public.path_membership_models",
		"SECURITY DEFINER SET search_path = pg_catalog, public",
		"audience IN ('nobody', 'path_members', 'followers', 'everyone')",
		"CREATE TABLE public.path_nudge_preference_replay_models",
		"CREATE TABLE public.notification_channel_preference_models",
		"'following', 'path_access', 'tracking_activity', 'achievements'",
		"'comments', 'reactions', 'comment_hearts', 'nudges'",
		"'goal_reminders', 'timer_health'",
		"CREATE TABLE public.notification_channel_preference_replay_models",
		"CREATE TABLE public.social_nudge_models",
		"content_kind text NOT NULL CHECK (content_kind = 'preset')",
		"preset IN (",
		"'you_have_got_this', 'lets_go', 'little_progress_counts',",
		"'keep_it_going', 'time_to_work'",
		"CHECK (sender_user_id <> recipient_user_id)",
		"CREATE UNIQUE INDEX social_nudge_models_interval_limit_idx",
		"WHERE interval_started_at IS NOT NULL",
		"CREATE TABLE public.social_nudge_replay_models",
		"ADD COLUMN nudge_id text REFERENCES public.social_nudge_models(id) ON DELETE CASCADE",
		"'nudge_received'",
		"channel = 'nudges'",
		"GRANT SELECT, INSERT, UPDATE (audience, revision, updated_at)",
		"GRANT SELECT, INSERT, UPDATE (enabled, revision, updated_at)",
		"GRANT SELECT, INSERT ON public.social_nudge_models TO app;",
		"GRANT SELECT, INSERT ON public.social_nudge_replay_models TO app;",
	} {
		if !strings.Contains(migration, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"GRANT UPDATE ON public.path_nudge_preference_models",
		"GRANT UPDATE ON public.notification_channel_preference_models",
		"GRANT UPDATE ON public.social_nudge_models",
		"custom_message",
	} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("migration contains forbidden capability %q", forbidden)
		}
	}
}

func TestNudgeDownMigrationRefusesToDiscardNudgesOrReplayEvidence(t *testing.T) {
	contents, err := fs.ReadFile(dbmigrations.Files, "000064_social_nudges.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	migration := string(contents)
	guard := strings.Index(migration, "IF EXISTS (SELECT 1 FROM public.social_nudge_models)")
	drop := strings.Index(migration, "DROP TABLE public.social_nudge_models")
	if guard < 0 || drop < 0 || guard > drop {
		t.Fatal("down migration must preflight retained nudge evidence")
	}
	for _, table := range []string{
		"social_nudge_replay_models",
		"path_nudge_preference_replay_models",
		"notification_channel_preference_replay_models",
	} {
		if strings.Index(migration, "IF EXISTS (SELECT 1 FROM public."+table+")") > strings.Index(migration, "DROP TABLE public."+table) {
			t.Fatalf("down migration drops %s before its evidence preflight", table)
		}
	}
}

func TestNudgeMigrationAdvancesReadinessVersion(t *testing.T) {
	if dbmigrations.LatestVersion != 66 {
		t.Fatalf("LatestVersion=%d, want 66", dbmigrations.LatestVersion)
	}
}
