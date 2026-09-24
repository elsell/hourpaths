BEGIN;

REVOKE INSERT ON public.path_membership_models FROM app;

COMMIT;
