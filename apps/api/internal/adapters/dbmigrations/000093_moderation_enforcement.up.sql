BEGIN;
-- Subject-facing notices are separate from restricted reporter/evidence data.
-- Only operational tooling may issue decisions; the runtime can read notices
-- through subject-scoped repository operations and submit immutable appeals.
CREATE TABLE public.moderation_enforcement_models (
 id text PRIMARY KEY,
 case_id text REFERENCES public.moderation_case_models(id) ON DELETE SET NULL,
 subject_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 action text NOT NULL CHECK(action IN ('content_removal','warning','suspension','ban')),
 policy_reason text NOT NULL CHECK(length(btrim(policy_reason))>0),
 issued_at timestamptz NOT NULL,
 until_at timestamptz,
 CHECK((action='suspension' AND until_at IS NOT NULL AND until_at>issued_at) OR (action<>'suspension' AND until_at IS NULL))
);
CREATE INDEX moderation_enforcement_subject_idx ON public.moderation_enforcement_models(subject_user_id,issued_at DESC,id);
CREATE TABLE public.moderation_appeal_models (
 enforcement_id text PRIMARY KEY REFERENCES public.moderation_enforcement_models(id) ON DELETE CASCADE,
 id text NOT NULL CHECK(length(id) BETWEEN 16 AND 128),
 explanation text NOT NULL,
 submitted_at timestamptz NOT NULL
);
CREATE TABLE public.moderation_appeal_decision_models (
 enforcement_id text PRIMARY KEY REFERENCES public.moderation_appeal_models(enforcement_id) ON DELETE CASCADE,
 outcome text NOT NULL CHECK(outcome IN ('upheld','reversed')),
 reviewer text NOT NULL CHECK(length(btrim(reviewer))>0),
 reason text NOT NULL CHECK(length(btrim(reason))>0),
 decided_at timestamptz NOT NULL
);
REVOKE ALL ON public.moderation_enforcement_models,public.moderation_appeal_models,public.moderation_appeal_decision_models FROM PUBLIC,app,app_audit_retention;
GRANT SELECT ON public.moderation_enforcement_models,public.moderation_appeal_models,public.moderation_appeal_decision_models TO app;
GRANT INSERT ON public.moderation_appeal_models TO app;
ALTER TABLE public.moderation_review_event_models DROP CONSTRAINT moderation_review_event_models_operation_check;
ALTER TABLE public.moderation_review_event_models ADD CONSTRAINT moderation_review_event_models_operation_check
 CHECK(operation IN ('list','read','reviewing','dismissed','warning','appeal_upheld','appeal_reversed'));

CREATE FUNCTION public.moderation_warn_user(review_case_id text, notice_id text, policy_reason text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; decision_at timestamptz;
BEGIN
 policy_reason:=btrim(policy_reason);
 IF policy_reason IS NULL OR char_length(policy_reason) NOT BETWEEN 1 AND 1000 OR notice_id IS NULL OR char_length(notice_id) NOT BETWEEN 1 AND 200 THEN
  RAISE EXCEPTION 'invalid warning';
 END IF;
 SELECT c.subject_user_id INTO subject_id FROM public.moderation_case_models c WHERE c.id=review_case_id;
 IF subject_id IS NULL THEN RAISE EXCEPTION 'case unavailable'; END IF;
 -- Keep lock order consistent with appeal submission and account deletion.
 PERFORM 1 FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable'; END IF;
 PERFORM 1 FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.subject_user_id=subject_id AND c.state IN ('open','reviewing') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case state changed' USING ERRCODE='40001'; END IF;
 decision_at:=clock_timestamp();
 INSERT INTO public.moderation_enforcement_models(id,case_id,subject_user_id,action,policy_reason,issued_at)
 VALUES(notice_id,review_case_id,subject_id,'warning',policy_reason,decision_at);
 UPDATE public.moderation_case_models SET state='actioned',reviewer=session_user,closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'warning',policy_reason,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_warn_user(text,text,text) FROM PUBLIC,app,app_audit_retention;
CREATE FUNCTION public.moderation_decide_warning_appeal(notice_id text, outcome text, decision_reason text, same_reviewer_reason text DEFAULT '')
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; review_case_id text; original_reviewer text; submitted_at timestamptz; decision_at timestamptz;
BEGIN
 decision_reason:=btrim(decision_reason); same_reviewer_reason:=btrim(COALESCE(same_reviewer_reason,''));
 IF outcome IS NULL OR outcome NOT IN ('upheld','reversed') OR decision_reason IS NULL OR char_length(decision_reason) NOT BETWEEN 1 AND 1000 OR char_length(same_reviewer_reason)>1000 THEN
  RAISE EXCEPTION 'invalid appeal decision';
 END IF;
 SELECT e.subject_user_id,e.case_id INTO subject_id,review_case_id FROM public.moderation_enforcement_models e WHERE e.id=notice_id AND e.action='warning';
 IF subject_id IS NULL OR review_case_id IS NULL THEN RAISE EXCEPTION 'appeal unavailable'; END IF;
 PERFORM 1 FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable'; END IF;
 SELECT c.reviewer INTO original_reviewer FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.state='actioned' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case unavailable'; END IF;
 IF original_reviewer=session_user AND same_reviewer_reason='' THEN RAISE EXCEPTION 'record why a different reviewer is unavailable'; END IF;
 SELECT a.submitted_at INTO submitted_at FROM public.moderation_appeal_models a WHERE a.enforcement_id=notice_id;
 IF submitted_at IS NULL THEN RAISE EXCEPTION 'appeal unavailable'; END IF;
 decision_at:=clock_timestamp();
 IF decision_at<submitted_at THEN RAISE EXCEPTION 'appeal submission is in the future'; END IF;
 INSERT INTO public.moderation_appeal_decision_models(enforcement_id,outcome,reviewer,reason,decided_at)
 VALUES(notice_id,outcome,session_user,decision_reason,decision_at);
 -- Reset case retention from final resolution; preserve the original warning
 -- and appeal as immutable records rather than rewriting either decision.
 UPDATE public.moderation_case_models SET closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'appeal_'||outcome,
 jsonb_build_object('decisionReason',decision_reason,'sameReviewerReason',same_reviewer_reason)::text,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_decide_warning_appeal(text,text,text,text) FROM PUBLIC,app,app_audit_retention;
CREATE OR REPLACE FUNCTION public.perform_moderation_case_retention() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 -- Start one day before the deadline; the existing hourly runner drains bounded
 -- batches and retains an immutable combined summary without API delete grants.
 WITH candidates AS (
  SELECT id FROM public.moderation_case_models
  WHERE closed_at IS NOT NULL AND closed_at<=NEW.completed_at-interval '89 days'
  AND NOT EXISTS (
   SELECT 1 FROM public.moderation_enforcement_models e
   JOIN public.moderation_appeal_models a ON a.enforcement_id=e.id
   LEFT JOIN public.moderation_appeal_decision_models d ON d.enforcement_id=e.id
   WHERE e.case_id=moderation_case_models.id AND d.enforcement_id IS NULL
  )
  ORDER BY closed_at,id
  LIMIT GREATEST(0,NEW.batch_limit-NEW.deleted_count-NEW.outbox_deleted_count-NEW.receipt_deleted_count)
  FOR UPDATE SKIP LOCKED
 ), deleted AS (
  DELETE FROM public.moderation_case_models c USING candidates x WHERE c.id=x.id RETURNING c.id
 ) SELECT count(*) INTO NEW.moderation_deleted_count FROM deleted;
 RETURN NEW;
END $$;
CREATE FUNCTION public.moderation_list_appeals(page_limit integer DEFAULT 20)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE result jsonb;
BEGIN
 IF page_limit IS NULL OR page_limit<1 OR page_limit>100 THEN RAISE EXCEPTION 'invalid appeal page size'; END IF;
 INSERT INTO public.moderation_review_event_models(actor_role,operation) VALUES(session_user,'list');
 SELECT COALESCE(jsonb_agg(to_jsonb(p)), '[]'::jsonb) INTO result FROM (
  SELECT e.id AS notice_id,e.case_id,e.action,a.submitted_at
  FROM public.moderation_enforcement_models e JOIN public.moderation_appeal_models a ON a.enforcement_id=e.id
  LEFT JOIN public.moderation_appeal_decision_models d ON d.enforcement_id=e.id
  WHERE d.enforcement_id IS NULL AND e.case_id IS NOT NULL ORDER BY a.submitted_at,e.id LIMIT page_limit
 ) p;
 RETURN result;
END $$;
CREATE FUNCTION public.moderation_read_appeal(notice_id text)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE result jsonb; review_case_id text;
BEGIN
 SELECT e.case_id,jsonb_build_object('notice',to_jsonb(e),'appeal',to_jsonb(a),'decision',to_jsonb(d)) INTO review_case_id,result
 FROM public.moderation_enforcement_models e JOIN public.moderation_appeal_models a ON a.enforcement_id=e.id
 LEFT JOIN public.moderation_appeal_decision_models d ON d.enforcement_id=e.id
 WHERE e.id=notice_id;
 IF result IS NULL OR review_case_id IS NULL THEN RAISE EXCEPTION 'appeal unavailable'; END IF;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation) VALUES(review_case_id,session_user,'read');
 RETURN result;
END $$;
REVOKE ALL ON FUNCTION public.moderation_list_appeals(integer),public.moderation_read_appeal(text) FROM PUBLIC,app,app_audit_retention;
COMMIT;
