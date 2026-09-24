BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.notification_models WHERE kind = 'path_deleted') THEN
    RAISE EXCEPTION 'cannot remove Path deletion persistence while deletion notices exist';
  END IF;
END
$$;

ALTER TABLE public.authorization_batch_outbox_models
  DROP CONSTRAINT authorization_batch_transfer_id_fkey,
  DROP CONSTRAINT authorization_batch_resource_id_fkey,
  ADD CONSTRAINT authorization_batch_transfer_id_fkey
    FOREIGN KEY (path_ownership_transfer_id) REFERENCES public.path_ownership_transfer_models(id) ON DELETE RESTRICT,
  ADD CONSTRAINT authorization_batch_resource_id_fkey
    FOREIGN KEY (resource_id) REFERENCES public.path_models(id) ON DELETE RESTRICT;

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_path_deletion_snapshot_check,
  DROP COLUMN path_name_snapshot,
  DROP COLUMN actor_username_snapshot,
  DROP COLUMN actor_display_name_snapshot,
  ALTER COLUMN path_id SET NOT NULL,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received') AND presentation_class = 'actionable')
    OR
    (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
  );

COMMIT;
