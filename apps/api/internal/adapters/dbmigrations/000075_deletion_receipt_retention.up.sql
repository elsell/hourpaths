BEGIN;
ALTER TABLE public.account_deletion_models ADD COLUMN journal_retired_at timestamptz;
ALTER TABLE public.account_deletion_retention_run_models ADD COLUMN receipt_deleted_count integer NOT NULL DEFAULT 0 CHECK (receipt_deleted_count >= 0);
CREATE OR REPLACE FUNCTION public.lock_account_deletion_receipt(owner_id text, digest bytea, observed_at timestamptz)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
BEGIN
  PERFORM 1 FROM public.account_deletion_models
  WHERE user_id = owner_id AND receipt_hash = digest AND journal_retired_at IS NULL
    AND deleted_at > observed_at - interval '30 days' AND deleted_at <= observed_at
  FOR UPDATE;
  RETURN FOUND;
END;
$$;
CREATE FUNCTION public.prepare_deletion_journal_retirement(owner_id text, digest bytea, accepted_at timestamptz)
RETURNS boolean LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
DECLARE marker public.account_deletion_models%ROWTYPE;
BEGIN
  IF accepted_at > clock_timestamp() - interval '29 days' OR octet_length(digest) <> 32 THEN RETURN false; END IF;
  SELECT * INTO marker FROM public.account_deletion_models WHERE user_id = owner_id FOR UPDATE;
  IF FOUND AND (marker.receipt_hash <> digest OR marker.deleted_at <> accepted_at) THEN RETURN false; END IF;
  IF EXISTS (SELECT 1 FROM public.user_models WHERE id = owner_id) THEN RETURN false; END IF;
  IF EXISTS (SELECT 1 FROM public.authorization_outbox_models AS delivery
    WHERE delivery.completed_at IS NULL AND (delivery.owner_user_id = owner_id OR delivery.actor_user_id = owner_id
      OR (delivery.subject_type = 'user' AND delivery.subject_id = owner_id)
      OR (delivery.resource_type = 'user' AND delivery.resource_id = owner_id))) THEN RETURN false; END IF;
  UPDATE public.account_deletion_models SET journal_retired_at = COALESCE(journal_retired_at, clock_timestamp()) WHERE user_id = owner_id;
  RETURN true;
END;
$$;
REVOKE ALL ON FUNCTION public.prepare_deletion_journal_retirement(text, bytea, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.prepare_deletion_journal_retirement(text, bytea, timestamptz) TO app;
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
  WITH candidates AS (
    SELECT deletion.user_id FROM public.account_deletion_models AS deletion
    WHERE deletion.journal_retired_at IS NOT NULL
      AND deletion.deleted_at <= NEW.completed_at - interval '29 days'
      AND NOT EXISTS (SELECT 1 FROM public.user_models WHERE id = deletion.user_id)
      AND NOT EXISTS (SELECT 1 FROM public.audit_event_models AS events
        WHERE events.owner_user_id = deletion.user_id OR events.actor_user_id = deletion.user_id
          OR (events.target_type IN ('user', 'account', 'account_deletion') AND events.target_id = deletion.user_id))
      AND NOT EXISTS (SELECT 1 FROM public.authorization_outbox_models AS delivery
        WHERE delivery.owner_user_id = deletion.user_id OR delivery.actor_user_id = deletion.user_id
          OR (delivery.subject_type = 'user' AND delivery.subject_id = deletion.user_id)
          OR (delivery.resource_type = 'user' AND delivery.resource_id = deletion.user_id))
    ORDER BY deletion.deleted_at, deletion.user_id
    LIMIT (NEW.batch_limit - NEW.deleted_count - NEW.outbox_deleted_count)
    FOR UPDATE SKIP LOCKED
  ), deleted AS (
    DELETE FROM public.account_deletion_models AS deletion USING candidates
    WHERE deletion.user_id = candidates.user_id RETURNING deletion.user_id
  ) SELECT count(*) INTO NEW.receipt_deleted_count FROM deleted;
  RETURN NEW;
END;
$$;

COMMIT;
