BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.goal_reminder_preference_models) THEN
  RAISE EXCEPTION 'cannot remove saved goal reminder preferences';
 END IF;
END $$;
DROP TABLE public.goal_reminder_preference_replay_models;
DROP TABLE public.goal_reminder_preference_models;
COMMIT;
