BEGIN;
ALTER TABLE public.moderation_enforcement_models ADD COLUMN affected_feed_event_id text,
 ADD COLUMN affected_feed_event_published_at timestamptz,
 ADD CONSTRAINT moderation_affected_feed_event_check CHECK (
  (affected_feed_event_id IS NULL AND affected_feed_event_published_at IS NULL) OR
  (action='content_removal' AND affected_comment_id IS NULL AND affected_nudge_id IS NULL AND affected_feed_event_id IS NOT NULL AND length(btrim(affected_feed_event_id))>0 AND affected_feed_event_published_at IS NOT NULL)
 );
CREATE TABLE public.moderation_removed_feed_event_models (
 feed_event_id text PRIMARY KEY REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE,
 enforcement_id text NOT NULL UNIQUE REFERENCES public.moderation_enforcement_models(id) ON DELETE CASCADE
);
REVOKE ALL ON public.moderation_removed_feed_event_models FROM PUBLIC,app,app_audit_retention;
GRANT SELECT ON public.moderation_removed_feed_event_models TO app;
CREATE FUNCTION public.moderation_remove_feed_event(review_case_id text, notice_id text, policy_reason text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; removed_id text; feed_event_published_at timestamptz; decision_at timestamptz;
BEGIN
 policy_reason:=btrim(policy_reason);
 IF policy_reason IS NULL OR char_length(policy_reason) NOT BETWEEN 1 AND 1000 OR notice_id IS NULL OR char_length(notice_id) NOT BETWEEN 1 AND 200 THEN
  RAISE EXCEPTION 'invalid content removal';
 END IF;
 SELECT c.subject_user_id,n.id INTO subject_id,removed_id FROM public.moderation_case_models c JOIN public.social_feed_event_models n ON n.id=c.target_id AND n.participant_user_id=c.subject_user_id WHERE c.id=review_case_id AND c.target_kind='feed_event';
 IF subject_id IS NULL THEN RAISE EXCEPTION 'feed_event case unavailable'; END IF;
 PERFORM 1 FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR NO KEY UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable'; END IF;
 PERFORM 1 FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.subject_user_id=subject_id AND c.target_kind='feed_event' AND c.state IN ('open','reviewing') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case state changed' USING ERRCODE='40001'; END IF;
 -- Same byte-length-prefixed identity as sociallock.Key; also held through push handoff.
 PERFORM pg_advisory_xact_lock(hashtextextended('30:social-practice-reaction-event'||octet_length(removed_id)::text||':'||removed_id,0));
 SELECT c.published_at INTO feed_event_published_at FROM public.social_feed_event_models c WHERE c.id=removed_id AND c.participant_user_id=subject_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'feed_event unavailable'; END IF;
 decision_at:=clock_timestamp();
 INSERT INTO public.moderation_enforcement_models(id,case_id,subject_user_id,action,policy_reason,issued_at,affected_feed_event_id,affected_feed_event_published_at)
 VALUES(notice_id,review_case_id,subject_id,'content_removal',policy_reason,decision_at,removed_id,feed_event_published_at);
 INSERT INTO public.moderation_removed_feed_event_models(feed_event_id,enforcement_id) VALUES(removed_id,notice_id);
 -- Retain social history for a successful appeal, but never resend old push.
 UPDATE public.notification_push_delivery_models d SET suppressed_at=decision_at, failure_code='moderated_content', locked_by=NULL, locked_until=NULL, token_ciphertext=NULL, token_nonce=NULL, token_hash=NULL FROM public.notification_models n WHERE n.id=d.notification_id AND n.social_feed_event_id=removed_id AND d.delivered_at IS NULL AND d.suppressed_at IS NULL AND d.permanently_failed_at IS NULL;
 UPDATE public.moderation_case_models SET state='actioned',reviewer=session_user,closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'content_removal',policy_reason,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_remove_feed_event(text,text,text) FROM PUBLIC,app,app_audit_retention;
CREATE FUNCTION public.moderation_decide_feed_event_appeal(notice_id text, outcome text, decision_reason text, same_reviewer_reason text DEFAULT '')
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; review_case_id text; original_reviewer text; submitted_at timestamptz; decision_at timestamptz;
BEGIN
 decision_reason:=btrim(decision_reason); same_reviewer_reason:=btrim(COALESCE(same_reviewer_reason,''));
 IF outcome IS NULL OR outcome NOT IN ('upheld','reversed') OR decision_reason IS NULL OR char_length(decision_reason) NOT BETWEEN 1 AND 1000 OR char_length(same_reviewer_reason)>1000 THEN
  RAISE EXCEPTION 'invalid appeal decision';
 END IF;
 SELECT e.subject_user_id,e.case_id INTO subject_id,review_case_id FROM public.moderation_enforcement_models e WHERE e.id=notice_id AND e.action='content_removal' AND e.affected_feed_event_id IS NOT NULL;
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
  DELETE FROM public.moderation_removed_feed_event_models r WHERE r.enforcement_id=notice_id;
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
REVOKE ALL ON FUNCTION public.moderation_decide_feed_event_appeal(text,text,text,text) FROM PUBLIC,app,app_audit_retention;
COMMIT;
