BEGIN;

REVOKE ALL ON public.path_membership_models FROM app;
GRANT SELECT, INSERT ON public.path_membership_models TO app;

COMMIT;
