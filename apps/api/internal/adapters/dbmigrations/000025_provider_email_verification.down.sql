BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.user_models WHERE provider_email_verified)
  THEN
    RAISE EXCEPTION 'cannot remove provider email verification provenance while verified provider emails exist';
  END IF;
END
$$;

ALTER TABLE public.user_models
  DROP CONSTRAINT user_models_verified_provider_email_check,
  DROP COLUMN provider_email_verified;

COMMIT;
