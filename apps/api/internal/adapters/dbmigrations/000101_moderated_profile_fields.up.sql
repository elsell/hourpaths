BEGIN;
CREATE TABLE public.moderation_removed_profile_field_models (
 enforcement_id text PRIMARY KEY REFERENCES public.moderation_enforcement_models(id) ON DELETE CASCADE,
 case_id text NOT NULL REFERENCES public.moderation_case_models(id) ON DELETE CASCADE,
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 fields text[] NOT NULL,
 original_name text NOT NULL,
 original_description text,
 original_picture_url text,
 original_picture_id uuid,
 original_picture_jpeg bytea,
 original_picture_created_at timestamptz,
 removed_profile_revision bigint NOT NULL,
 removed_picture_revision bigint NOT NULL
);
REVOKE ALL ON public.moderation_removed_profile_field_models FROM PUBLIC,app,app_audit_retention;
CREATE FUNCTION public.moderation_remove_profile_fields(review_case_id text,notice_id text,policy_reason text,removed_fields text[],replacement_name text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; reviewed record; original public.user_models%ROWTYPE; picture public.user_profile_picture_models%ROWTYPE; decision_at timestamptz;
BEGIN
 policy_reason:=btrim(policy_reason);replacement_name:=btrim(replacement_name);
 IF policy_reason IS NULL OR char_length(policy_reason) NOT BETWEEN 1 AND 1000 OR notice_id IS NULL OR char_length(notice_id) NOT BETWEEN 1 AND 200
 OR removed_fields IS NULL OR cardinality(removed_fields) NOT BETWEEN 1 AND 3
 OR array_position(removed_fields,NULL) IS NOT NULL
 OR NOT removed_fields <@ ARRAY['display_name','description','picture']::text[]
 OR cardinality(removed_fields)<>(SELECT count(DISTINCT f) FROM unnest(removed_fields) f)
 OR ('display_name'=ANY(removed_fields) AND (replacement_name IS NULL OR char_length(replacement_name) NOT BETWEEN 1 AND 100 OR replacement_name ~ '[[:cntrl:]]')) THEN RAISE EXCEPTION 'invalid profile removal';END IF;
 SELECT c.subject_user_id INTO subject_id FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.target_kind='profile' AND c.target_id=c.subject_user_id;
 IF subject_id IS NULL THEN RAISE EXCEPTION 'profile case unavailable';END IF;
 SELECT * INTO original FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable';END IF;
 SELECT c.evidence,c.evidence_jpeg INTO reviewed FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.subject_user_id=subject_id AND c.target_kind='profile' AND c.state IN ('open','reviewing') FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case state changed' USING ERRCODE='40001';END IF;
 SELECT * INTO picture FROM public.user_profile_picture_models p WHERE p.user_id=subject_id;
 IF ('display_name'=ANY(removed_fields) AND (original.display_name IS DISTINCT FROM reviewed.evidence->>'title' OR original.display_name=replacement_name))
 OR ('description'=ANY(removed_fields) AND (original.description IS NULL OR original.description IS DISTINCT FROM reviewed.evidence->>'text'))
 OR ('picture'=ANY(removed_fields) AND (picture.id IS NULL OR picture.jpeg IS DISTINCT FROM reviewed.evidence_jpeg)) THEN RAISE EXCEPTION 'reported profile content changed' USING ERRCODE='40001';END IF;
 decision_at:=clock_timestamp();
 UPDATE public.user_models u SET
 display_name=CASE WHEN 'display_name'=ANY(removed_fields) THEN replacement_name ELSE u.display_name END,
 description=CASE WHEN 'description'=ANY(removed_fields) THEN NULL ELSE u.description END,
 profile_picture_url=CASE WHEN 'picture'=ANY(removed_fields) THEN NULL ELSE u.profile_picture_url END,
 profile_revision=u.profile_revision+CASE WHEN removed_fields && ARRAY['display_name','description'] THEN 1 ELSE 0 END,
 picture_revision=u.picture_revision+CASE WHEN 'picture'=ANY(removed_fields) THEN 1 ELSE 0 END,
 updated_at=decision_at WHERE u.id=subject_id;
 IF 'picture'=ANY(removed_fields) THEN DELETE FROM public.user_profile_picture_models WHERE user_id=subject_id;END IF;
 IF 'display_name'=ANY(removed_fields) THEN
  UPDATE public.notification_push_delivery_models d SET suppressed_at=decision_at,failure_code='moderated_content',locked_by=NULL,locked_until=NULL,token_ciphertext=NULL,token_nonce=NULL,token_hash=NULL
  FROM public.notification_models n WHERE n.id=d.notification_id AND n.actor_user_id=subject_id
  AND d.delivered_at IS NULL AND d.suppressed_at IS NULL AND d.permanently_failed_at IS NULL;
 END IF;
 INSERT INTO public.moderation_enforcement_models(id,case_id,subject_user_id,action,policy_reason,issued_at)
 VALUES(notice_id,review_case_id,subject_id,'content_removal',policy_reason,decision_at);
 INSERT INTO public.moderation_removed_profile_field_models(enforcement_id,case_id,user_id,fields,original_name,original_description,original_picture_url,original_picture_id,original_picture_jpeg,original_picture_created_at,removed_profile_revision,removed_picture_revision)
 SELECT notice_id,review_case_id,subject_id,removed_fields,CASE WHEN 'display_name'=ANY(removed_fields) THEN original.display_name ELSE '' END,CASE WHEN 'description'=ANY(removed_fields) THEN original.description END,CASE WHEN 'picture'=ANY(removed_fields) THEN original.profile_picture_url END,
 CASE WHEN 'picture'=ANY(removed_fields) THEN picture.id END,CASE WHEN 'picture'=ANY(removed_fields) THEN picture.jpeg END,CASE WHEN 'picture'=ANY(removed_fields) THEN picture.created_at END,u.profile_revision,u.picture_revision FROM public.user_models u WHERE u.id=subject_id;
 UPDATE public.moderation_case_models SET state='actioned',reviewer=session_user,closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'content_removal',policy_reason,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_remove_profile_fields(text,text,text,text[],text) FROM PUBLIC,app,app_audit_retention;
CREATE FUNCTION public.moderation_decide_profile_fields_appeal(notice_id text,outcome text,decision_reason text,same_reviewer_reason text DEFAULT '')
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE subject_id text; review_case_id text; original_reviewer text; submitted_at timestamptz; decision_at timestamptz; retained public.moderation_removed_profile_field_models%ROWTYPE; current_profile public.user_models%ROWTYPE;
BEGIN
 decision_reason:=btrim(decision_reason);same_reviewer_reason:=btrim(COALESCE(same_reviewer_reason,''));
 IF outcome IS NULL OR outcome NOT IN ('upheld','reversed') OR decision_reason IS NULL OR char_length(decision_reason) NOT BETWEEN 1 AND 1000 OR char_length(same_reviewer_reason)>1000 THEN RAISE EXCEPTION 'invalid appeal decision';END IF;
 SELECT r.* INTO retained FROM public.moderation_removed_profile_field_models r WHERE r.enforcement_id=notice_id;
 IF NOT FOUND THEN RAISE EXCEPTION 'appeal unavailable';END IF;
 subject_id:=retained.user_id;review_case_id:=retained.case_id;
 SELECT * INTO current_profile FROM public.user_models u WHERE u.id=subject_id AND u.status='active' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'subject unavailable';END IF;
 SELECT c.reviewer INTO original_reviewer FROM public.moderation_case_models c WHERE c.id=review_case_id AND c.state='actioned' FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'case unavailable';END IF;
 IF original_reviewer=session_user AND same_reviewer_reason='' THEN RAISE EXCEPTION 'record why a different reviewer is unavailable';END IF;
 SELECT a.submitted_at INTO submitted_at FROM public.moderation_appeal_models a WHERE a.enforcement_id=notice_id;
 IF submitted_at IS NULL THEN RAISE EXCEPTION 'appeal unavailable';END IF;
 decision_at:=clock_timestamp();
 IF decision_at<submitted_at THEN RAISE EXCEPTION 'appeal submission is in the future';END IF;
 INSERT INTO public.moderation_appeal_decision_models(enforcement_id,outcome,reviewer,reason,decided_at)
 VALUES(notice_id,outcome,session_user,decision_reason,decision_at);
 IF outcome='reversed' THEN
  IF current_profile.profile_revision=retained.removed_profile_revision AND retained.fields && ARRAY['display_name','description'] THEN
   UPDATE public.user_models u SET display_name=CASE WHEN 'display_name'=ANY(retained.fields) THEN retained.original_name ELSE u.display_name END,
   description=CASE WHEN 'description'=ANY(retained.fields) THEN retained.original_description ELSE u.description END,
   profile_revision=u.profile_revision+1,updated_at=decision_at WHERE u.id=subject_id;
  END IF;
  IF current_profile.picture_revision=retained.removed_picture_revision AND 'picture'=ANY(retained.fields) AND current_profile.profile_picture_url IS NULL
   AND NOT EXISTS(SELECT 1 FROM public.user_profile_picture_models WHERE user_id=subject_id) THEN
   INSERT INTO public.user_profile_picture_models(id,user_id,jpeg,created_at) VALUES(retained.original_picture_id,subject_id,retained.original_picture_jpeg,retained.original_picture_created_at);
   UPDATE public.user_models SET profile_picture_url=retained.original_picture_url,picture_revision=picture_revision+1,updated_at=decision_at WHERE id=subject_id;
  END IF;
 END IF;
 UPDATE public.moderation_case_models SET closed_at=decision_at WHERE id=review_case_id;
 INSERT INTO public.moderation_review_event_models(case_id,actor_role,operation,reason,occurred_at)
 VALUES(review_case_id,session_user,'appeal_'||outcome,jsonb_build_object('decisionReason',decision_reason,'sameReviewerReason',same_reviewer_reason)::text,decision_at);
END $$;
REVOKE ALL ON FUNCTION public.moderation_decide_profile_fields_appeal(text,text,text,text) FROM PUBLIC,app,app_audit_retention;
COMMIT;
