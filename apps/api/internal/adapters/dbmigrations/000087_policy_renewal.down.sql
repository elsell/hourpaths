BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.user_policy_renewal_mutation_models) THEN
  RAISE EXCEPTION 'cannot roll back retained policy renewal receipts';
 END IF;
END $$;
DROP TABLE public.user_policy_renewal_mutation_models;
COMMIT;
