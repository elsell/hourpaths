BEGIN;

-- Audit identifiers describe historical actors. Removing a profile must not
-- require modifying immutable audit records or keeping its personal data.
ALTER TABLE public.audit_event_models
  DROP CONSTRAINT audit_event_models_owner_user_id_fkey,
  DROP CONSTRAINT audit_event_models_actor_user_id_fkey;

-- These personal records disappear through the account foreign key. The API
-- still has no direct DELETE privilege on transfer or batch-outbox tables.
ALTER TABLE public.path_ownership_transfer_models
  DROP CONSTRAINT path_ownership_transfer_models_initiator_user_id_fkey,
  DROP CONSTRAINT path_ownership_transfer_models_recipient_user_id_fkey,
  ADD CONSTRAINT path_ownership_transfer_models_initiator_user_id_fkey
    FOREIGN KEY (initiator_user_id) REFERENCES public.user_models(id) ON DELETE CASCADE,
  ADD CONSTRAINT path_ownership_transfer_models_recipient_user_id_fkey
    FOREIGN KEY (recipient_user_id) REFERENCES public.user_models(id) ON DELETE CASCADE;
ALTER TABLE public.authorization_batch_outbox_models
  DROP CONSTRAINT authorization_batch_outbox_models_owner_user_id_fkey,
  DROP CONSTRAINT authorization_batch_outbox_models_actor_user_id_fkey,
  ADD CONSTRAINT authorization_batch_outbox_models_owner_user_id_fkey
    FOREIGN KEY (owner_user_id) REFERENCES public.user_models(id) ON DELETE CASCADE,
  ADD CONSTRAINT authorization_batch_outbox_models_actor_user_id_fkey
    FOREIGN KEY (actor_user_id) REFERENCES public.user_models(id) ON DELETE CASCADE;

CREATE TABLE public.account_deletion_models (
  user_id text PRIMARY KEY CHECK (user_id <> '' AND user_id = btrim(user_id)),
  deleted_at timestamptz NOT NULL,
  audit_event_id text NOT NULL UNIQUE,
  receipt_hash bytea NOT NULL CHECK (octet_length(receipt_hash) = 32)
);
REVOKE ALL ON public.account_deletion_models FROM app;
GRANT SELECT, INSERT ON public.account_deletion_models TO app;

COMMIT;
