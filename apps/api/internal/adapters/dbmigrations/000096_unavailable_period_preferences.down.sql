BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.user_unavailable_period_mutation_models LIMIT 1) THEN
  RAISE EXCEPTION 'cannot remove durable unavailable-period mutation receipts';
 END IF;
END $$;
DROP TABLE public.user_unavailable_period_mutation_models;
DROP TABLE public.user_unavailable_period_models;
COMMIT;
