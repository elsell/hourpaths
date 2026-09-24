BEGIN;

CREATE TABLE public.push_installation_models (
  id text PRIMARY KEY,
  owner_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  provider text NOT NULL CHECK (provider IN ('expo', 'apns', 'fcm')),
  platform text NOT NULL CHECK (platform IN ('ios', 'android')),
  locale text NOT NULL CHECK (locale IN ('en', 'es')),
  token_ciphertext bytea,
  token_nonce bytea,
  token_hash bytea,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  deleted_at timestamptz,
  CHECK (updated_at >= created_at),
  CHECK (deleted_at IS NULL OR deleted_at >= created_at),
  CHECK (
    (deleted_at IS NULL AND octet_length(token_ciphertext) > 16
      AND octet_length(token_nonce) = 12 AND octet_length(token_hash) = 32)
    OR
    (deleted_at IS NOT NULL AND token_ciphertext IS NULL
      AND token_nonce IS NULL AND token_hash IS NULL)
  )
);

CREATE UNIQUE INDEX push_installation_models_active_token_idx
  ON public.push_installation_models(token_hash)
  WHERE deleted_at IS NULL;
CREATE INDEX push_installation_models_owner_active_idx
  ON public.push_installation_models(owner_user_id, id)
  WHERE deleted_at IS NULL;

CREATE TABLE public.notification_push_delivery_models (
  notification_id text NOT NULL REFERENCES public.notification_models(id) ON DELETE CASCADE,
  installation_id text NOT NULL,
  recipient_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  provider text NOT NULL CHECK (provider IN ('expo', 'apns', 'fcm')),
  platform text NOT NULL CHECK (platform IN ('ios', 'android')),
  locale text NOT NULL CHECK (locale IN ('en', 'es')),
  token_ciphertext bytea,
  token_nonce bytea,
  token_hash bytea,
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  available_at timestamptz NOT NULL,
  provider_ticket text NOT NULL DEFAULT '',
  locked_by text,
  locked_until timestamptz,
  delivered_at timestamptz,
  suppressed_at timestamptz,
  permanently_failed_at timestamptz,
  failure_code text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  PRIMARY KEY (notification_id, installation_id),
  CHECK (available_at >= created_at),
  CHECK (num_nonnulls(delivered_at, suppressed_at, permanently_failed_at) <= 1),
  CHECK (delivered_at IS NULL OR delivered_at >= created_at),
  CHECK (suppressed_at IS NULL OR suppressed_at >= created_at),
  CHECK (permanently_failed_at IS NULL OR permanently_failed_at >= created_at),
  CHECK ((locked_by IS NULL) = (locked_until IS NULL)),
  CHECK (delivered_at IS NULL OR failure_code = ''),
  CHECK (suppressed_at IS NULL OR failure_code <> ''),
  CHECK (permanently_failed_at IS NULL OR failure_code <> ''),
  CHECK (
    (delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL
      AND octet_length(token_ciphertext) > 16
      AND octet_length(token_nonce) = 12 AND octet_length(token_hash) = 32)
    OR
    (num_nonnulls(delivered_at, suppressed_at, permanently_failed_at) = 1
      AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL)
  )
);

CREATE FUNCTION public.validate_push_delivery_installation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM public.push_installation_models i
    JOIN public.notification_models n
      ON n.id = NEW.notification_id
     AND n.recipient_user_id = NEW.recipient_user_id
    WHERE i.id = NEW.installation_id
      AND i.owner_user_id = NEW.recipient_user_id
      AND i.deleted_at IS NULL
  ) THEN
    RAISE EXCEPTION 'push delivery installation is not active for recipient';
  END IF;
  RETURN NEW;
END
$$;

CREATE TRIGGER notification_push_delivery_installation_check
BEFORE INSERT OR UPDATE OF installation_id, recipient_user_id
ON public.notification_push_delivery_models
FOR EACH ROW EXECUTE FUNCTION public.validate_push_delivery_installation();

CREATE INDEX notification_push_delivery_models_claim_idx
  ON public.notification_push_delivery_models(available_at, created_at, notification_id, installation_id)
  WHERE delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL;

REVOKE ALL ON public.push_installation_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.push_installation_models TO app;
REVOKE ALL ON public.notification_push_delivery_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.notification_push_delivery_models TO app;

COMMIT;
