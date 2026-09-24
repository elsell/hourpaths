BEGIN;

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_follow_subject_actor_check;

COMMIT;
