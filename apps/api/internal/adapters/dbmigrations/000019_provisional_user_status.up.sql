BEGIN;

ALTER TABLE public.user_models
  DROP CONSTRAINT user_models_status_check;
ALTER TABLE public.user_models
  ADD CONSTRAINT user_models_status_check
  CHECK (status IN ('provisional', 'active', 'disabled'));
ALTER TABLE public.user_models
  ALTER COLUMN status SET DEFAULT 'provisional';

COMMIT;
