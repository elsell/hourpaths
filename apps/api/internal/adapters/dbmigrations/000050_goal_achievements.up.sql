BEGIN;

CREATE TABLE public.social_goal_achievement_models (
  id text PRIMARY KEY CHECK (id = btrim(id) AND id <> ''),
  participant_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (kind IN ('interval', 'overall')),
  target_seconds bigint NOT NULL CHECK (target_seconds > 0),
  interval_started_at timestamptz,
  interval_ended_at timestamptz,
  published_at timestamptz NOT NULL,
  CONSTRAINT social_goal_achievement_interval_shape_check CHECK (
    (kind = 'interval' AND interval_started_at IS NOT NULL AND interval_ended_at IS NOT NULL AND interval_ended_at > interval_started_at)
    OR (kind = 'overall' AND interval_started_at IS NULL AND interval_ended_at IS NULL)
  )
);
CREATE UNIQUE INDEX social_goal_achievement_interval_supported_idx
  ON public.social_goal_achievement_models(participant_user_id, path_id, target_seconds, interval_started_at, interval_ended_at)
  WHERE kind = 'interval';
CREATE UNIQUE INDEX social_goal_achievement_overall_supported_idx
  ON public.social_goal_achievement_models(participant_user_id, path_id, target_seconds)
  WHERE kind = 'overall';

ALTER TABLE public.social_feed_event_models
  ALTER COLUMN source_activity_id DROP NOT NULL,
  DROP CONSTRAINT social_feed_event_models_id_check,
  ADD COLUMN achievement_id text UNIQUE,
  ADD CONSTRAINT social_feed_event_models_achievement_id_fkey
    FOREIGN KEY (achievement_id) REFERENCES public.social_goal_achievement_models(id) ON DELETE CASCADE,
  ADD CONSTRAINT social_feed_event_models_one_source_check
    CHECK (num_nonnulls(source_activity_id, achievement_id) = 1),
  ADD CONSTRAINT social_feed_event_models_typed_id_check CHECK (
    (source_activity_id IS NOT NULL AND id = 'practice:' || source_activity_id)
    OR (achievement_id IS NOT NULL AND id = 'achievement:' || achievement_id)
  );

REVOKE ALL ON public.social_goal_achievement_models FROM app;
GRANT SELECT, INSERT, DELETE ON public.social_goal_achievement_models TO app;

COMMIT;
