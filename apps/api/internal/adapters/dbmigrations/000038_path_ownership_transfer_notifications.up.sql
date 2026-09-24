BEGIN;

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_offered_role_check,
  DROP CONSTRAINT notification_models_check2,
  ALTER COLUMN path_invitation_id DROP NOT NULL,
  ADD COLUMN path_ownership_transfer_id text REFERENCES public.path_ownership_transfer_models(id) ON DELETE CASCADE,
  ALTER COLUMN offered_role DROP NOT NULL,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received',
      'path_invitation_accepted',
      'path_ownership_transfer_received',
      'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined',
      'path_ownership_transfer_canceled'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received') AND presentation_class = 'actionable')
    OR
    (kind IN (
      'path_invitation_accepted',
      'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined',
      'path_ownership_transfer_canceled'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK (
    (path_invitation_id IS NULL) = (offered_role IS NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_check CHECK (
    offered_role IS NULL OR offered_role IN ('participant', 'supporter')
  );

CREATE UNIQUE INDEX notification_models_transfer_kind_recipient_idx
  ON public.notification_models(path_ownership_transfer_id, kind, recipient_user_id)
  WHERE path_ownership_transfer_id IS NOT NULL;

COMMIT;
