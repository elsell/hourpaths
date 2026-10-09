BEGIN;
DROP FUNCTION public.moderation_list_appeals(integer),public.moderation_read_appeal(text);
CREATE OR REPLACE FUNCTION public.perform_moderation_case_retention() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 -- Start one day before the deadline; the existing hourly runner drains bounded
 -- batches and retains an immutable combined summary without API delete grants.
 WITH candidates AS (
  SELECT id FROM public.moderation_case_models
  WHERE closed_at IS NOT NULL AND closed_at<=NEW.completed_at-interval '89 days'
  ORDER BY closed_at,id
  LIMIT GREATEST(0,NEW.batch_limit-NEW.deleted_count-NEW.outbox_deleted_count-NEW.receipt_deleted_count)
  FOR UPDATE SKIP LOCKED
 ), deleted AS (
  DELETE FROM public.moderation_case_models c USING candidates x WHERE c.id=x.id RETURNING c.id
 ) SELECT count(*) INTO NEW.moderation_deleted_count FROM deleted;
 RETURN NEW;
END $$;

DROP FUNCTION public.moderation_decide_warning_appeal(text,text,text,text);
DROP FUNCTION public.moderation_warn_user(text,text,text);
DROP TABLE public.moderation_appeal_decision_models;
DROP TABLE public.moderation_appeal_models;
DROP TABLE public.moderation_enforcement_models;
COMMIT;
