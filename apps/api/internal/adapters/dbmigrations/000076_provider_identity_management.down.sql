BEGIN;
-- Restoring the old single-identity model must fail if any account has linked
-- another provider. Never silently discard a working sign-in method.
ALTER TABLE public.identity_models ADD CONSTRAINT identity_models_user_id_key UNIQUE(user_id);
DROP TABLE public.identity_link_challenge_models;
DROP INDEX public.identity_models_user_provider_key;
DROP INDEX public.identity_models_user_id_idx;
ALTER TABLE public.identity_models DROP COLUMN provider;
COMMIT;
