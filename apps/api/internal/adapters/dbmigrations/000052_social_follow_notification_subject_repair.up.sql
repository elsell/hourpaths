BEGIN;

ALTER TABLE public.notification_models
  ADD CONSTRAINT notification_models_follow_subject_actor_check CHECK (
    kind NOT IN ('new_follower', 'follow_request_received', 'follow_request_accepted')
    OR follow_subject_user_id = actor_user_id
  ) NOT VALID;

UPDATE public.notification_models
SET follow_subject_user_id = actor_user_id
WHERE kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted')
  AND follow_subject_user_id IS DISTINCT FROM actor_user_id;

ALTER TABLE public.notification_models
  VALIDATE CONSTRAINT notification_models_follow_subject_actor_check;

COMMIT;
