BEGIN;

CREATE FUNCTION public.leave_path_membership(path_identifier text, member_user_identifier text)
RETURNS boolean
LANGUAGE sql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
  WITH deleted AS (
    DELETE FROM public.path_membership_models
    WHERE path_id = path_identifier
      AND user_id = member_user_identifier
      AND role IN ('administrator', 'participant', 'supporter')
    RETURNING 1
  )
  SELECT EXISTS (SELECT 1 FROM deleted);
$$;

REVOKE ALL ON FUNCTION public.leave_path_membership(text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.leave_path_membership(text, text) TO app;

COMMIT;
