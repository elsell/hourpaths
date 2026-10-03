ALTER TABLE public.account_deletion_retention_run_models ADD COLUMN outbox_deleted_count integer NOT NULL DEFAULT 0 CHECK (outbox_deleted_count >= 0);
CREATE OR REPLACE FUNCTION public.perform_deleted_account_audit_retention() RETURNS trigger
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
  WITH candidates AS (
    SELECT delivery.id FROM public.authorization_outbox_models AS delivery
    WHERE delivery.completed_at IS NOT NULL AND EXISTS (
      SELECT 1 FROM public.account_deletion_models AS deletion
      WHERE deletion.deleted_at <= NEW.completed_at - interval '29 days'
      AND (delivery.owner_user_id = deletion.user_id OR delivery.actor_user_id = deletion.user_id
        OR (delivery.subject_type = 'user' AND delivery.subject_id = deletion.user_id)
        OR (delivery.resource_type = 'user' AND delivery.resource_id = deletion.user_id))
    )
    ORDER BY delivery.created_at, delivery.id
    LIMIT (NEW.batch_limit - NEW.deleted_count) FOR UPDATE SKIP LOCKED
  ), deleted AS (
    DELETE FROM public.authorization_outbox_models AS delivery USING candidates
    WHERE delivery.id = candidates.id RETURNING delivery.id
  ) SELECT count(*) INTO NEW.outbox_deleted_count FROM deleted;
  RETURN NEW;
END;
$$;
