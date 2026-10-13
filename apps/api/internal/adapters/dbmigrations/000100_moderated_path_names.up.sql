BEGIN;
ALTER TABLE public.path_models ADD COLUMN name_revision bigint NOT NULL DEFAULT 0 CHECK(name_revision>=0);
CREATE FUNCTION public.advance_path_name_revision() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public AS $$
BEGIN
 NEW.name_revision:=OLD.name_revision+CASE WHEN NEW.name IS DISTINCT FROM OLD.name OR NEW.owner_user_id IS DISTINCT FROM OLD.owner_user_id THEN 1 ELSE 0 END;
 RETURN NEW;
END $$;
CREATE TRIGGER path_name_revision BEFORE UPDATE ON public.path_models
FOR EACH ROW EXECUTE FUNCTION public.advance_path_name_revision();
REVOKE ALL ON FUNCTION public.advance_path_name_revision() FROM PUBLIC,app,app_audit_retention;
ALTER TABLE public.moderation_enforcement_models ADD COLUMN affected_path_id text,
 ADD CONSTRAINT moderation_affected_path_check CHECK(affected_path_id IS NULL OR
 (action='content_removal' AND length(btrim(affected_path_id))>0 AND affected_comment_id IS NULL AND affected_nudge_id IS NULL AND affected_feed_event_id IS NULL));
CREATE TABLE public.moderation_removed_path_name_models (
 enforcement_id text PRIMARY KEY REFERENCES public.moderation_enforcement_models(id) ON DELETE CASCADE,
 case_id text NOT NULL REFERENCES public.moderation_case_models(id) ON DELETE CASCADE,
 path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
 original_name text NOT NULL,
 replacement_name text NOT NULL,
 removed_revision bigint NOT NULL
);
REVOKE ALL ON public.moderation_removed_path_name_models FROM PUBLIC,app,app_audit_retention;
CREATE FUNCTION public.moderation_remove_path_name(review_case_id text,notice_id text,policy_reason text,replacement_name text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; removed_id text; reported_name text; original_name text; revision bigint; decision_at timestamptz;
BEGIN
 policy_reason:=btrim(policy_reason); replacement_name:=btrim(replacement_name);
 IF policy_reason IS NULL OR char_length(policy_reason) NOT BETWEEN 1 AND 1000 OR notice_id IS NULL OR char_length(notice_id) NOT BETWEEN 1 AND 200
 OR replacement_name IS NULL OR char_length(replacement_name) NOT BETWEEN 1 AND 100 OR replacement_name ~ '[[:cntrl:]]' THEN RAISE EXCEPTION 'invalid name removal'; END IF;
 SELECT c.subject_user_id,c.target_id INTO subject_id,removed_id FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.target_kind='path';
 IF subject_id IS NULL THEN RAISE EXCEPTION 'path case unavailable'; END IF;
 PERFORM 1 FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR NO KEY UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable'; END IF;
 SELECT c.evidence->>'title' INTO reported_name FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.subject_user_id=subject_id AND c.target_kind='path' AND c.state IN ('open','reviewing') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case state changed' USING ERRCODE='40001'; END IF;
 SELECT p.name INTO original_name FROM public.path_models p WHERE p.id=removed_id AND p.owner_user_id=subject_id FOR UPDATE;
 IF NOT FOUND OR original_name IS DISTINCT FROM reported_name OR original_name=replacement_name THEN RAISE EXCEPTION 'reported name changed' USING ERRCODE='40001'; END IF;
 decision_at:=clock_timestamp();
 UPDATE public.path_models p SET name=replacement_name,updated_at=decision_at WHERE p.id=removed_id RETURNING p.name_revision INTO revision;
 -- Retire already composed/queued push; in-app history reads the current name.
 UPDATE public.notification_push_delivery_models d SET suppressed_at=decision_at,failure_code='moderated_content',locked_by=NULL,locked_until=NULL,token_ciphertext=NULL,token_nonce=NULL,token_hash=NULL
 FROM public.notification_models n WHERE n.id=d.notification_id
 AND (n.path_id=removed_id OR EXISTS(SELECT 1 FROM public.goal_reminder_receipt_models r WHERE r.notification_id=n.id AND r.path_id=removed_id))
 AND d.delivered_at IS NULL AND d.suppressed_at IS NULL AND d.permanently_failed_at IS NULL;
 INSERT INTO public.moderation_enforcement_models(id,case_id,subject_user_id,action,policy_reason,issued_at,affected_path_id)
 VALUES(notice_id,review_case_id,subject_id,'content_removal',policy_reason,decision_at,removed_id);
 INSERT INTO public.moderation_removed_path_name_models(enforcement_id,case_id,path_id,original_name,replacement_name,removed_revision)
 VALUES(notice_id,review_case_id,removed_id,original_name,replacement_name,revision);
 UPDATE public.moderation_case_models SET state='actioned',reviewer=session_user,closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'content_removal',policy_reason,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_remove_path_name(text,text,text,text) FROM PUBLIC,app,app_audit_retention;
CREATE FUNCTION public.moderation_decide_path_name_appeal(notice_id text,outcome text,decision_reason text,same_reviewer_reason text DEFAULT '')
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; review_case_id text; original_reviewer text; submitted_at timestamptz; decision_at timestamptz;
BEGIN
 decision_reason:=btrim(decision_reason);same_reviewer_reason:=btrim(COALESCE(same_reviewer_reason,''));
 IF outcome IS NULL OR outcome NOT IN ('upheld','reversed') OR decision_reason IS NULL OR char_length(decision_reason) NOT BETWEEN 1 AND 1000 OR char_length(same_reviewer_reason)>1000 THEN RAISE EXCEPTION 'invalid appeal decision';END IF;
 SELECT e.subject_user_id,e.case_id INTO subject_id,review_case_id FROM public.moderation_enforcement_models e WHERE e.id=notice_id AND e.action='content_removal' AND e.affected_path_id IS NOT NULL;
 IF subject_id IS NULL OR review_case_id IS NULL THEN RAISE EXCEPTION 'appeal unavailable';END IF;
 PERFORM 1 FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable';END IF;
 SELECT c.reviewer INTO original_reviewer FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.state='actioned' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case unavailable';END IF;
 IF original_reviewer=session_user AND same_reviewer_reason='' THEN RAISE EXCEPTION 'record why a different reviewer is unavailable';END IF;
 SELECT a.submitted_at INTO submitted_at FROM public.moderation_appeal_models a WHERE a.enforcement_id=notice_id;
 IF submitted_at IS NULL THEN RAISE EXCEPTION 'appeal unavailable';END IF;
 decision_at:=clock_timestamp();
 IF decision_at<submitted_at THEN RAISE EXCEPTION 'appeal submission is in the future';END IF;
 -- A duplicate final decision fails before any restoration can take place.
 INSERT INTO public.moderation_appeal_decision_models(enforcement_id,outcome,reviewer,reason,decided_at)
 VALUES(notice_id,outcome,session_user,decision_reason,decision_at);
 IF outcome='reversed' THEN
  UPDATE public.path_models p SET name=r.original_name,updated_at=decision_at
  FROM public.moderation_removed_path_name_models r
  WHERE r.enforcement_id=notice_id AND p.id=r.path_id AND p.owner_user_id=subject_id
   AND p.name_revision=r.removed_revision AND p.name=r.replacement_name;
 END IF;
 UPDATE public.moderation_case_models SET closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'appeal_'||outcome,jsonb_build_object('decisionReason',decision_reason,'sameReviewerReason',same_reviewer_reason)::text,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_decide_path_name_appeal(text,text,text,text) FROM PUBLIC,app,app_audit_retention;
COMMIT;
