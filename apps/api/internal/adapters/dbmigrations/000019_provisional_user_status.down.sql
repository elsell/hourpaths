BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.user_models WHERE status = 'provisional') THEN
    RAISE EXCEPTION 'cannot remove provisional user status while provisional users exist';
  END IF;
END
$$;

ALTER TABLE public.user_models
  DROP CONSTRAINT user_models_status_check;
ALTER TABLE public.user_models
  ADD CONSTRAINT user_models_status_check
  CHECK (status IN ('active', 'disabled'));
ALTER TABLE public.user_models
  ALTER COLUMN status SET DEFAULT 'active';

COMMIT;
