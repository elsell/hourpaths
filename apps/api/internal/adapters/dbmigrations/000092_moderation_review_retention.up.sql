BEGIN;
CREATE TABLE public.moderation_review_event_models (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 case_id text REFERENCES public.moderation_case_models(id) ON DELETE CASCADE,
 actor_role text NOT NULL,
 operation text NOT NULL CHECK(operation IN ('list','read','reviewing','dismissed')),
 reason text NOT NULL DEFAULT '',
 occurred_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
REVOKE ALL ON public.moderation_review_event_models FROM PUBLIC,app,app_audit_retention;

-- These operational capabilities are deliberately not granted to application
-- or retention credentials. An operator login may receive EXECUTE explicitly.
CREATE FUNCTION public.moderation_list_cases(page_limit integer DEFAULT 20)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE result jsonb;
BEGIN
 IF page_limit IS NULL OR page_limit<1 OR page_limit>100 THEN RAISE EXCEPTION 'invalid case page size'; END IF;
 INSERT INTO public.moderation_review_event_models(actor_role,operation) VALUES(session_user,'list');
 SELECT COALESCE(jsonb_agg(to_jsonb(c)), '[]'::jsonb) INTO result FROM (
  SELECT id,target_kind,state,reviewer,created_at FROM public.moderation_case_models
  WHERE state IN ('open','reviewing') ORDER BY created_at,id LIMIT page_limit
 ) c;
 RETURN result;
END $$;
CREATE FUNCTION public.moderation_read_case(case_id text)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE result jsonb;
BEGIN
 SELECT (to_jsonb(c)-'evidence_jpeg') || jsonb_build_object('pictureBase64',encode(c.evidence_jpeg,'base64')) INTO result
 FROM public.moderation_case_models c WHERE c.id=case_id;
 IF result IS NULL THEN RAISE EXCEPTION 'case unavailable'; END IF;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation) VALUES(case_id,session_user,'read');
 RETURN result;
END $$;
CREATE FUNCTION public.moderation_set_case_state(case_id text, expected_state text, next_state text, decision_reason text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 decision_reason := btrim(decision_reason);
 IF decision_reason IS NULL OR char_length(decision_reason) NOT BETWEEN 1 AND 1000
  OR expected_state NOT IN ('open','reviewing') OR next_state NOT IN ('reviewing','dismissed') THEN
  RAISE EXCEPTION 'invalid moderation review';
 END IF;
 UPDATE public.moderation_case_models c SET state=next_state,reviewer=session_user,
  closed_at=CASE WHEN next_state='dismissed' THEN clock_timestamp() ELSE NULL END
 WHERE c.id=case_id AND c.state=expected_state;
 IF NOT FOUND THEN RAISE EXCEPTION 'case state changed' USING ERRCODE='40001'; END IF;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason)
 VALUES(case_id,session_user,next_state,decision_reason);
END $$;
REVOKE ALL ON FUNCTION public.moderation_list_cases(integer), public.moderation_read_case(text), public.moderation_set_case_state(text,text,text,text) FROM PUBLIC,app,app_audit_retention;

ALTER TABLE public.account_deletion_retention_run_models ADD COLUMN moderation_deleted_count integer NOT NULL DEFAULT 0 CHECK(moderation_deleted_count>=0);
CREATE FUNCTION public.perform_moderation_case_retention() RETURNS trigger
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
REVOKE ALL ON FUNCTION public.perform_moderation_case_retention() FROM PUBLIC,app,app_audit_retention;
CREATE TRIGGER moderation_case_retention_perform_run BEFORE INSERT ON public.account_deletion_retention_run_models
 FOR EACH ROW EXECUTE FUNCTION public.perform_moderation_case_retention();
COMMIT;
