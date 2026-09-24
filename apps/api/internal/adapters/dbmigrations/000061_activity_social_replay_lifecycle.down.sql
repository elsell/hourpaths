BEGIN;

LOCK TABLE public.activity_mutation_models IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM public.activity_mutation_models
    WHERE operation = 'activity.delete'
      AND result_removed_feed_event_ids IS NOT NULL
      AND cardinality(result_removed_feed_event_ids) > 0
  ) THEN
    RAISE EXCEPTION 'cannot remove activity deletion feed-event receipts while deletion evidence exists';
  END IF;
END $$;

ALTER TABLE public.social_practice_comment_heart_replay_models
  DROP CONSTRAINT social_practice_comment_heart_replay_models_event_fkey;

ALTER TABLE public.social_practice_comment_replay_models
  DROP CONSTRAINT social_practice_comment_replay_models_event_fkey;

ALTER TABLE public.activity_mutation_models
  DROP CONSTRAINT activity_mutation_models_deletion_projection_receipt_check,
  DROP COLUMN result_session_count,
  DROP COLUMN result_unread_notification_count,
  DROP COLUMN result_removed_feed_event_ids;

COMMIT;
