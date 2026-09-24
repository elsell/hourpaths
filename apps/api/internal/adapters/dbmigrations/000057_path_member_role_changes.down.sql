BEGIN;

DROP FUNCTION IF EXISTS public.change_path_member_role(text, text, text, text, text);
DELETE FROM public.notification_models WHERE kind IN ('path_member_removed', 'path_member_role_changed');

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'path_member_left', 'new_follower',
      'follow_request_received', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received', 'follow_request_received')
      AND presentation_class = 'actionable')
    OR (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'path_member_left', 'new_follower', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted' AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind = 'path_member_left' AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind IN ('path_invitation_received', 'path_invitation_accepted', 'path_ownership_transfer_received', 'path_ownership_transfer_accepted', 'path_ownership_transfer_declined', 'path_ownership_transfer_canceled') AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1 AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind = 'new_follower' AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL AND follow_subject_user_id IS NOT NULL AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind IN ('follow_request_received', 'follow_request_accepted') AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind = 'practice_reaction' AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NOT NULL AND reaction_type IS NOT NULL AND comment_id IS NULL)
    OR (kind IN ('practice_comment', 'comment_heart') AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NOT NULL AND reaction_type IS NULL AND comment_id IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK ((path_invitation_id IS NULL) = (offered_role IS NULL));

COMMIT;
