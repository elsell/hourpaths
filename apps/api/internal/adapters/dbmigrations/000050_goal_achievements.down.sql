BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.social_goal_achievement_models) THEN
    RAISE EXCEPTION 'cannot remove goal achievements while achievement data exists';
  END IF;
END
$$;

ALTER TABLE public.social_feed_event_models
  DROP CONSTRAINT social_feed_event_models_achievement_id_fkey,
  DROP CONSTRAINT social_feed_event_models_one_source_check,
  DROP CONSTRAINT social_feed_event_models_typed_id_check,
  DROP CONSTRAINT social_feed_event_models_achievement_id_key,
  DROP COLUMN achievement_id,
  ALTER COLUMN source_activity_id SET NOT NULL,
  ADD CONSTRAINT social_feed_event_models_id_check CHECK (id = 'practice:' || source_activity_id);

DROP INDEX public.social_goal_achievement_overall_supported_idx;
DROP INDEX public.social_goal_achievement_interval_supported_idx;
DROP TABLE public.social_goal_achievement_models;

COMMIT;
