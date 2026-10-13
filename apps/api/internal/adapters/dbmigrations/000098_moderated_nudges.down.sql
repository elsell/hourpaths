BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.moderation_enforcement_models WHERE affected_nudge_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot discard nudge-removal decisions';
 END IF;
END $$;
DROP FUNCTION public.moderation_decide_nudge_appeal(text,text,text,text);
DROP FUNCTION public.moderation_remove_nudge(text,text,text);
DROP TABLE public.moderation_removed_nudge_models;
ALTER TABLE public.moderation_enforcement_models DROP COLUMN affected_nudge_id, DROP COLUMN affected_nudge_sent_at;
CREATE OR REPLACE FUNCTION public.moderation_decide_comment_appeal(notice_id text, outcome text, decision_reason text, same_reviewer_reason text DEFAULT '')
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; review_case_id text; original_reviewer text; submitted_at timestamptz; decision_at timestamptz;
BEGIN
 decision_reason:=btrim(decision_reason); same_reviewer_reason:=btrim(COALESCE(same_reviewer_reason,''));
 IF outcome IS NULL OR outcome NOT IN ('upheld','reversed') OR decision_reason IS NULL OR char_length(decision_reason) NOT BETWEEN 1 AND 1000 OR char_length(same_reviewer_reason)>1000 THEN
  RAISE EXCEPTION 'invalid appeal decision';
 END IF;
 SELECT e.subject_user_id,e.case_id INTO subject_id,review_case_id FROM public.moderation_enforcement_models e WHERE e.id=notice_id AND e.action='content_removal';
 IF subject_id IS NULL OR review_case_id IS NULL THEN RAISE EXCEPTION 'appeal unavailable'; END IF;
 PERFORM 1 FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable'; END IF;
 SELECT c.reviewer INTO original_reviewer FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.state='actioned' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case unavailable'; END IF;
 IF original_reviewer=session_user AND same_reviewer_reason='' THEN RAISE EXCEPTION 'record why a different reviewer is unavailable'; END IF;
 SELECT a.submitted_at INTO submitted_at FROM public.moderation_appeal_models a WHERE a.enforcement_id=notice_id;
 IF submitted_at IS NULL THEN RAISE EXCEPTION 'appeal unavailable'; END IF;
 -- An ordinary deletion may have removed the row; reversal never recreates it.
 IF outcome='reversed' THEN
  DELETE FROM public.moderation_removed_comment_models r WHERE r.enforcement_id=notice_id;
 END IF;
 decision_at:=clock_timestamp();
 IF decision_at<submitted_at THEN RAISE EXCEPTION 'appeal submission is in the future'; END IF;
 INSERT INTO public.moderation_appeal_decision_models(enforcement_id,outcome,reviewer,reason,decided_at)
 VALUES(notice_id,outcome,session_user,decision_reason,decision_at);
 -- Reset case retention from final resolution; preserve the original removal
 -- and appeal as immutable records rather than rewriting either decision.
 UPDATE public.moderation_case_models SET closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'appeal_'||outcome,
 jsonb_build_object('decisionReason',decision_reason,'sameReviewerReason',same_reviewer_reason)::text,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_decide_comment_appeal(text,text,text,text) FROM PUBLIC,app,app_audit_retention;
COMMIT;
