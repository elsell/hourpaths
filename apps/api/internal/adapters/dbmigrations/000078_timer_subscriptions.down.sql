BEGIN;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM public.path_timer_subscription_models)
    OR EXISTS (SELECT 1 FROM public.timer_subscription_replay_models)
    OR EXISTS (SELECT 1 FROM public.follow_models WHERE activity_notifications_revision > 0)
  THEN RAISE EXCEPTION 'timer subscription data must be retained; rollback refused';
  END IF;
END $$;
DROP TABLE public.timer_subscription_replay_models;
DROP TABLE public.path_timer_subscription_models;
REVOKE UPDATE (activity_notifications_enabled, activity_notifications_revision)
  ON public.follow_models FROM app;
ALTER TABLE public.follow_models DROP COLUMN activity_notifications_revision;
COMMIT;
