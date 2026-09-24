BEGIN;

-- Take the receipt-table lock first. Runtime deletion writes its receipt before
-- feed-event cascades, so this order cannot deadlock with an in-flight delete.
ALTER TABLE public.activity_mutation_models
  ADD COLUMN result_session_count bigint,
  ADD COLUMN result_unread_notification_count bigint,
  ADD COLUMN result_removed_feed_event_ids text[];

UPDATE public.activity_mutation_models AS mutation
SET result_session_count = (
      SELECT count(*)
      FROM public.recorded_activity_models AS activity
      WHERE activity.participant_id = mutation.participant_id
        AND activity.path_id = mutation.path_id
    ),
    result_unread_notification_count = (
      SELECT count(*)
      FROM public.notification_models AS notification
      LEFT JOIN public.path_invitation_models AS invitation ON invitation.id = notification.path_invitation_id
      LEFT JOIN public.path_ownership_transfer_models AS transfer ON transfer.id = notification.path_ownership_transfer_id
      LEFT JOIN public.follow_request_models AS follow_request ON follow_request.id = notification.follow_request_id
      JOIN public.user_models AS notification_actor ON notification_actor.id = notification.actor_user_id
      WHERE notification.recipient_user_id = mutation.participant_id
        AND notification.read_at IS NULL
        AND notification.deleted_at IS NULL
        AND NOT EXISTS (
          SELECT 1 FROM public.block_models AS notification_block
          WHERE (notification_block.blocker_user_id = notification.recipient_user_id AND notification_block.blocked_user_id = notification.actor_user_id)
             OR (notification_block.blocker_user_id = notification.actor_user_id AND notification_block.blocked_user_id = notification.recipient_user_id)
        )
        AND (notification.kind <> 'path_invitation_received' OR (invitation.accepted_at IS NULL AND invitation.rejected_at IS NULL AND invitation.canceled_at IS NULL))
        AND (notification.kind <> 'path_ownership_transfer_received' OR (transfer.accepted_at IS NULL AND transfer.declined_at IS NULL AND transfer.canceled_at IS NULL AND transfer.expired_at IS NULL AND transfer.expires_at > CURRENT_TIMESTAMP))
        AND (notification.kind <> 'follow_request_received' OR (follow_request.accepted_at IS NULL AND follow_request.rejected_at IS NULL AND follow_request.canceled_at IS NULL))
    ),
    result_removed_feed_event_ids = ARRAY['practice:' || mutation.result_activity_id]
WHERE mutation.operation = 'activity.delete';

ALTER TABLE public.activity_mutation_models
  ADD CONSTRAINT activity_mutation_models_deletion_projection_receipt_check CHECK (
    (
      operation = 'activity.delete'
      AND result_session_count IS NOT NULL AND result_session_count >= 0
      AND result_unread_notification_count IS NOT NULL AND result_unread_notification_count >= 0
      AND result_removed_feed_event_ids IS NOT NULL
      AND cardinality(result_removed_feed_event_ids) >= 1
      AND result_removed_feed_event_ids @> ARRAY['practice:' || result_activity_id]
    )
    OR (
      operation <> 'activity.delete'
      AND result_session_count IS NULL
      AND result_unread_notification_count IS NULL
      AND result_removed_feed_event_ids IS NULL
    )
  );

-- Serialize the orphan repair with feed-event deletion before adding the
-- lifecycle constraints. ALTER TABLE takes the corresponding child locks.
LOCK TABLE public.social_feed_event_models IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE public.social_practice_comment_replay_models IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE public.social_practice_comment_heart_replay_models IN SHARE ROW EXCLUSIVE MODE;

DELETE FROM public.social_practice_comment_replay_models AS replay
WHERE NOT EXISTS (
  SELECT 1
  FROM public.social_feed_event_models AS event
  WHERE event.id = replay.social_feed_event_id
);

DELETE FROM public.social_practice_comment_heart_replay_models AS replay
WHERE NOT EXISTS (
  SELECT 1
  FROM public.social_feed_event_models AS event
  WHERE event.id = replay.social_feed_event_id
);

ALTER TABLE public.social_practice_comment_replay_models
  ADD CONSTRAINT social_practice_comment_replay_models_event_fkey
  FOREIGN KEY (social_feed_event_id)
  REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE;

ALTER TABLE public.social_practice_comment_heart_replay_models
  ADD CONSTRAINT social_practice_comment_heart_replay_models_event_fkey
  FOREIGN KEY (social_feed_event_id)
  REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE;

COMMIT;
