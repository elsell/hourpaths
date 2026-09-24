BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.user_time_zone_preference_mutation_models) THEN
    RAISE EXCEPTION 'cannot roll back time-zone preferences while mutation evidence exists';
  END IF;
END
$$;

REVOKE UPDATE (current_time_zone, updated_at) ON public.user_preference_models FROM app;
REVOKE ALL ON public.user_time_zone_preference_mutation_models FROM app;
DROP TABLE public.user_time_zone_preference_mutation_models;

COMMIT;
