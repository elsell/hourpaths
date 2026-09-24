BEGIN;

DROP INDEX public.user_models_normalized_email_unique_idx;

CREATE INDEX user_models_normalized_email_lookup_idx
ON public.user_models (lower(email))
WHERE email <> '';

COMMIT;
