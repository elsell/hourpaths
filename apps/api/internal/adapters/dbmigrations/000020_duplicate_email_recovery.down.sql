BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT lower(email)
    FROM public.user_models
    WHERE email <> ''
    GROUP BY lower(email)
    HAVING count(*) > 1
  ) THEN
    RAISE EXCEPTION 'cannot restore unique user email index while duplicate normalized emails exist';
  END IF;
END
$$;

DROP INDEX public.user_models_normalized_email_lookup_idx;

CREATE UNIQUE INDEX user_models_normalized_email_unique_idx
ON public.user_models (lower(email))
WHERE email <> '';

COMMIT;
