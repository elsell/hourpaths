CREATE TABLE public.account_deletion_retention_run_models (
  id text PRIMARY KEY,
  batch_limit integer NOT NULL CHECK (batch_limit BETWEEN 1 AND 10000),
  deleted_count integer NOT NULL DEFAULT 0 CHECK (deleted_count >= 0),
  completed_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE FUNCTION public.perform_deleted_account_audit_retention() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
  NEW.completed_at := clock_timestamp();
  WITH candidates AS (
    SELECT events.id FROM public.audit_event_models AS events
    WHERE EXISTS (
      SELECT 1 FROM public.account_deletion_models AS deletion
      WHERE deletion.deleted_at <= NEW.completed_at - interval '29 days'
      AND (events.owner_user_id = deletion.user_id OR events.actor_user_id = deletion.user_id
        OR (events.target_type IN ('user', 'account', 'account_deletion') AND events.target_id = deletion.user_id))
    )
    ORDER BY events.occurred_at, events.id LIMIT NEW.batch_limit FOR UPDATE SKIP LOCKED
  ), deleted AS (
    DELETE FROM public.audit_event_models AS events USING candidates
    WHERE events.id = candidates.id RETURNING events.id
  ) SELECT count(*) INTO NEW.deleted_count FROM deleted;
  RETURN NEW;
END;
$$;
REVOKE ALL ON FUNCTION public.perform_deleted_account_audit_retention() FROM PUBLIC, app;
CREATE TRIGGER account_deletion_retention_perform_run
  BEFORE INSERT ON public.account_deletion_retention_run_models
  FOR EACH ROW EXECUTE FUNCTION public.perform_deleted_account_audit_retention();
CREATE TRIGGER account_deletion_retention_append_only
  BEFORE UPDATE OR DELETE ON public.account_deletion_retention_run_models
  FOR EACH ROW EXECUTE FUNCTION public.reject_audit_retention_summary_mutation();
CREATE TRIGGER account_deletion_retention_no_truncate
  BEFORE TRUNCATE ON public.account_deletion_retention_run_models
  FOR EACH STATEMENT EXECUTE FUNCTION public.reject_audit_retention_summary_mutation();
REVOKE ALL ON public.account_deletion_retention_run_models FROM app;
GRANT SELECT, INSERT ON public.account_deletion_retention_run_models TO app_audit_retention;
