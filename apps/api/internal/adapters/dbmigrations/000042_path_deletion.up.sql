BEGIN;

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  ALTER COLUMN path_id DROP NOT NULL,
  ADD COLUMN path_name_snapshot text,
  ADD COLUMN actor_username_snapshot text,
  ADD COLUMN actor_display_name_snapshot text,
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
  ADD CONSTRAINT notification_models_path_deletion_snapshot_check CHECK (
    (kind = 'path_deleted'
      AND path_name_snapshot = btrim(path_name_snapshot) AND path_name_snapshot <> ''
      AND actor_username_snapshot = btrim(actor_username_snapshot) AND actor_username_snapshot <> ''
      AND actor_display_name_snapshot = btrim(actor_display_name_snapshot) AND actor_display_name_snapshot <> '')
    OR
    (kind <> 'path_deleted' AND path_name_snapshot IS NULL AND actor_username_snapshot IS NULL AND actor_display_name_snapshot IS NULL)
  );

ALTER TABLE public.authorization_batch_outbox_models
  DROP CONSTRAINT IF EXISTS authorization_batch_outbox_models_path_ownership_transfer_id_fk,
  DROP CONSTRAINT IF EXISTS authorization_batch_transfer_id_fkey,
  DROP CONSTRAINT IF EXISTS authorization_batch_outbox_models_resource_id_fkey,
  DROP CONSTRAINT IF EXISTS authorization_batch_resource_id_fkey,
  ADD CONSTRAINT authorization_batch_transfer_id_fkey
    FOREIGN KEY (path_ownership_transfer_id) REFERENCES public.path_ownership_transfer_models(id) ON DELETE CASCADE,
  ADD CONSTRAINT authorization_batch_resource_id_fkey
    FOREIGN KEY (resource_id) REFERENCES public.path_models(id) ON DELETE CASCADE;

COMMIT;
