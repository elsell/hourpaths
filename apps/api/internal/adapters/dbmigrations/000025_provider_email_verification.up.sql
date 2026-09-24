BEGIN;

ALTER TABLE public.user_models
  ADD COLUMN provider_email_verified boolean NOT NULL DEFAULT false,
  ADD CONSTRAINT user_models_verified_provider_email_check
    CHECK (NOT provider_email_verified OR btrim(email) <> '');

COMMIT;
