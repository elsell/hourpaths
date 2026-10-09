BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.user_week_start_preference_mutation_models LIMIT 1) THEN
  RAISE EXCEPTION 'cannot remove durable week-start mutation receipts';
 END IF;
END $$;
DROP TABLE public.user_week_start_preference_mutation_models;
REVOKE UPDATE (first_day_of_week) ON public.user_preference_models FROM app;
COMMIT;
