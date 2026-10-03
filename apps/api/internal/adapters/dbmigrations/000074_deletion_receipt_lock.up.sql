BEGIN;
-- Only capability-matching rows can be locked. No mutation privilege is added.
CREATE FUNCTION public.lock_account_deletion_receipt(owner_id text, digest bytea, observed_at timestamptz)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
  PERFORM 1 FROM public.account_deletion_models
  WHERE user_id = owner_id AND receipt_hash = digest
    AND deleted_at > observed_at - interval '30 days' AND deleted_at <= observed_at
  FOR UPDATE;
  RETURN FOUND;
END;
$$;
REVOKE ALL ON FUNCTION public.lock_account_deletion_receipt(text, bytea, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.lock_account_deletion_receipt(text, bytea, timestamptz) TO app;
COMMIT;
