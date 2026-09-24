BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.follow_request_models)
    OR EXISTS (SELECT 1 FROM public.social_relationship_replay_models)
    OR EXISTS (
      SELECT 1 FROM public.notification_models
      WHERE kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted')
    )
    OR EXISTS (
      SELECT 1 FROM public.follow_models
      WHERE activity_notifications_enabled OR authorization_change_id IS NOT NULL
    ) THEN
    RAISE EXCEPTION 'cannot remove social relationship state while durable evidence exists';
  END IF;
END
$$;

DROP INDEX public.notification_models_follow_request_kind_recipient_idx;
ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_channel_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_kind_check,
  DROP COLUMN follow_subject_user_id,
  DROP COLUMN follow_request_id,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received') AND presentation_class = 'actionable')
    OR
    (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted' AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL)
    OR
    (kind <> 'path_deleted' AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK (
    (path_invitation_id IS NULL) = (offered_role IS NULL)
  ),
  ADD CONSTRAINT notification_models_channel_check CHECK (channel = 'path_access');

DROP TABLE public.social_relationship_replay_models;
DROP TABLE public.follow_request_models;

REVOKE ALL ON public.follow_models FROM app;
GRANT SELECT ON public.follow_models TO app;
ALTER TABLE public.follow_models
  DROP COLUMN authorization_change_id,
  DROP COLUMN activity_notifications_enabled;

COMMIT;
