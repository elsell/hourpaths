BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.user_models
    WHERE username IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot remove active usernames while username data exists';
  END IF;
END
$$;

DROP INDEX public.user_models_normalized_username_unique_idx;
ALTER TABLE public.user_models
  DROP CONSTRAINT user_models_provisional_username_check,
  DROP CONSTRAINT user_models_username_format_check,
  DROP COLUMN username;

COMMIT;
