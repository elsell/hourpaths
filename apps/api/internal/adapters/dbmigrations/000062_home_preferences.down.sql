BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.home_preference_mutation_models) THEN
    RAISE EXCEPTION 'cannot roll back Home preferences while mutation evidence exists';
  END IF;
END
$$;

REVOKE UPDATE (home_order_method, home_order_revision, home_order_updated_at)
  ON public.user_preference_models FROM app;
REVOKE ALL ON public.home_preference_mutation_models FROM app;
REVOKE ALL ON public.home_path_preference_models FROM app;
DROP INDEX public.recorded_activity_models_home_recent_idx;
DROP TABLE public.home_preference_mutation_models;
DROP TABLE public.home_path_preference_models;
ALTER TABLE public.user_preference_models
  DROP COLUMN home_order_updated_at,
  DROP COLUMN home_order_revision,
  DROP COLUMN home_order_method;

COMMIT;
