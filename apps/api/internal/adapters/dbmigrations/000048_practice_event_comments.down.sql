BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.social_practice_comment_models)
    OR EXISTS (SELECT 1 FROM public.social_practice_comment_replay_models)
    OR EXISTS (SELECT 1 FROM public.notification_models WHERE kind = 'practice_comment') THEN
    RAISE EXCEPTION 'cannot remove practice comments while comment data exists';
  END IF;
END
$$;

DROP INDEX public.notification_models_active_practice_comment_idx;
ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_channel_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_kind_check,
  DROP COLUMN comment_id,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_received', 'follow_request_accepted',
      'practice_reaction'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received', 'follow_request_received')
      AND presentation_class = 'actionable')
    OR
    (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_accepted', 'practice_reaction'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted'
      AND path_id IS NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND path_id IS NOT NULL
      AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind = 'new_follower'
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind IN ('follow_request_received', 'follow_request_accepted')
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind = 'practice_reaction'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK (
    (path_invitation_id IS NULL) = (offered_role IS NULL)
  ),
  ADD CONSTRAINT notification_models_channel_check CHECK (
    channel IN ('path_access', 'following', 'reactions')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following')
      OR (kind = 'practice_reaction' AND channel = 'reactions'))
  );

DROP TABLE public.social_practice_comment_replay_models;
DROP TABLE public.social_practice_comment_revision_models;
DROP TABLE public.social_practice_comment_models;
REVOKE ALL ON public.social_feed_event_models FROM app;
GRANT SELECT, INSERT ON public.social_feed_event_models TO app;

COMMIT;
