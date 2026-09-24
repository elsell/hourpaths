BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.social_nudge_models) THEN
    RAISE EXCEPTION 'cannot roll back social nudges while nudge evidence exists';
  END IF;
  IF EXISTS (SELECT 1 FROM public.social_nudge_replay_models) THEN
    RAISE EXCEPTION 'cannot roll back social nudges while send replay evidence exists';
  END IF;
  IF EXISTS (SELECT 1 FROM public.path_nudge_preference_replay_models) THEN
    RAISE EXCEPTION 'cannot roll back social nudges while preference replay evidence exists';
  END IF;
  IF EXISTS (SELECT 1 FROM public.notification_channel_preference_replay_models) THEN
    RAISE EXCEPTION 'cannot roll back social nudges while notification preference replay evidence exists';
  END IF;
END $$;

DROP INDEX public.notification_models_nudge_recipient_idx;
ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_channel_check,
  DROP COLUMN nudge_id,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'path_member_left', 'path_member_removed',
      'path_member_role_changed', 'new_follower',
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
      'path_deleted', 'path_member_left', 'path_member_removed',
      'path_member_role_changed', 'new_follower', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted'
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL)
    OR (kind = 'path_member_left'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL)
    OR (kind = 'path_member_removed'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IN ('participant', 'supporter'))
    OR (kind = 'path_member_role_changed'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IN ('participant', 'supporter', 'administrator'))
    OR (kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind = 'new_follower' AND path_id IS NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NOT NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL)
    OR (kind IN ('follow_request_received', 'follow_request_accepted') AND path_id IS NULL
      AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL)
    OR (kind = 'practice_reaction' AND path_id IS NOT NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NULL AND social_feed_event_id IS NOT NULL
      AND reaction_type IS NOT NULL AND comment_id IS NULL AND offered_role IS NULL)
    OR (kind IN ('practice_comment', 'comment_heart') AND path_id IS NOT NULL
      AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NULL AND comment_id IS NOT NULL AND offered_role IS NULL)
  ),
  ADD CONSTRAINT notification_models_channel_check CHECK (
    channel IN ('path_access', 'following', 'reactions', 'comments', 'comment_hearts')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following')
      OR (kind = 'practice_reaction' AND channel = 'reactions')
      OR (kind = 'practice_comment' AND channel = 'comments')
      OR (kind = 'comment_heart' AND channel = 'comment_hearts'))
  );

DROP TABLE public.social_nudge_replay_models;
DROP TABLE public.social_nudge_models;
DROP TABLE public.notification_channel_preference_replay_models;
DROP TABLE public.notification_channel_preference_models;
DROP TABLE public.path_nudge_preference_replay_models;
DROP TRIGGER path_membership_remove_ineligible_nudge_preference ON public.path_membership_models;
DROP FUNCTION public.remove_ineligible_path_nudge_preference();
DROP TABLE public.path_nudge_preference_models;
DROP FUNCTION public.enforce_path_nudge_preference_participant();

COMMIT;
