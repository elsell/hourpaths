BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.notification_models
    WHERE path_ownership_transfer_id IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot remove Path ownership transfer notification support while evidence exists';
  END IF;
END
$$;

DROP INDEX public.notification_models_transfer_kind_recipient_idx;

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  DROP CONSTRAINT notification_models_offered_role_check,
  DROP COLUMN path_ownership_transfer_id,
  ALTER COLUMN path_invitation_id SET NOT NULL,
  ALTER COLUMN offered_role SET NOT NULL,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN ('path_invitation_received', 'path_invitation_accepted')
  ),
  ADD CONSTRAINT notification_models_offered_role_check CHECK (
    offered_role IN ('participant', 'supporter')
  ),
  ADD CONSTRAINT notification_models_check2 CHECK (
    (kind = 'path_invitation_received' AND presentation_class = 'actionable')
    OR
    (kind = 'path_invitation_accepted' AND presentation_class = 'informational')
  );

COMMIT;
