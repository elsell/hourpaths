BEGIN;

CREATE TABLE public.path_ownership_transfer_models (
  id text PRIMARY KEY,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  initiator_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE RESTRICT,
  recipient_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL,
  expires_at timestamptz NOT NULL,
  accepted_at timestamptz,
  declined_at timestamptz,
  canceled_at timestamptz,
  expired_at timestamptz,
  CHECK (initiator_user_id <> recipient_user_id),
  CHECK (num_nonnulls(accepted_at, declined_at, canceled_at, expired_at) <= 1),
  CHECK (expires_at > created_at),
  CHECK (accepted_at IS NULL OR accepted_at >= created_at),
  CHECK (declined_at IS NULL OR declined_at >= created_at),
  CHECK (canceled_at IS NULL OR canceled_at >= created_at),
  CHECK (expired_at IS NULL OR expired_at >= expires_at)
);

CREATE UNIQUE INDEX path_ownership_transfer_models_one_pending_idx
  ON public.path_ownership_transfer_models(path_id)
  WHERE accepted_at IS NULL AND declined_at IS NULL AND canceled_at IS NULL AND expired_at IS NULL;

CREATE INDEX path_ownership_transfer_models_recipient_pending_idx
  ON public.path_ownership_transfer_models(recipient_user_id, created_at DESC, id)
  WHERE accepted_at IS NULL AND declined_at IS NULL AND canceled_at IS NULL AND expired_at IS NULL;

REVOKE ALL ON public.path_ownership_transfer_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.path_ownership_transfer_models TO app;

COMMIT;
