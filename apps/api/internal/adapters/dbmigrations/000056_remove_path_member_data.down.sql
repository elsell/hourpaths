BEGIN;
REVOKE EXECUTE ON FUNCTION public.remove_path_member_data(text, text, text, text) FROM app;
DROP FUNCTION public.remove_path_member_data(text, text, text, text);
ALTER TABLE public.path_membership_models DROP COLUMN joined_at;
COMMIT;
