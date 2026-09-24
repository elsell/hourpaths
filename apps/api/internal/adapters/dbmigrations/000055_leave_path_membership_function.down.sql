BEGIN;

REVOKE EXECUTE ON FUNCTION public.leave_path_membership(text, text) FROM app;
DROP FUNCTION public.leave_path_membership(text, text);

COMMIT;
