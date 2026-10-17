BEGIN;
CREATE TABLE public.platform_admin_models (
 user_id text PRIMARY KEY REFERENCES public.user_models(id) ON DELETE CASCADE,
 granted_at timestamptz NOT NULL, granted_by text NOT NULL
);
CREATE TABLE public.support_request_models (
 id text PRIMARY KEY CHECK(id ~ '^[0-7][0-9A-HJKMNP-TV-Z]{25}$'),
 key_hash text NOT NULL CHECK (key_hash ~ '^[0-9a-f]{64}$'),
 subject text NOT NULL CHECK (char_length(subject) BETWEEN 1 AND 160),
 created_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL CHECK (expires_at>created_at)
);
CREATE TABLE public.support_message_models (
 id text PRIMARY KEY, request_id text NOT NULL REFERENCES public.support_request_models(id) ON DELETE CASCADE,
 body text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 5000),
 operator boolean NOT NULL, created_at timestamptz NOT NULL
);
CREATE INDEX support_message_request_idx ON public.support_message_models(request_id,created_at,id);
CREATE TABLE public.support_notice_models (
 id text NOT NULL, recipient_id text NOT NULL REFERENCES public.platform_admin_models(user_id) ON DELETE CASCADE,
 request_id text NOT NULL REFERENCES public.support_request_models(id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL, read_at timestamptz,
 PRIMARY KEY(id,recipient_id)
);
CREATE INDEX support_notice_recipient_idx ON public.support_notice_models(recipient_id,created_at,id);
CREATE TABLE IF NOT EXISTS public.support_audit_models (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 request_id text NOT NULL, actor text NOT NULL,
 operation text NOT NULL CHECK(operation IN ('create','read','reply','list','notices','grant','deny')),
 occurred_at timestamptz NOT NULL
);
DROP TRIGGER IF EXISTS support_audit_append_only ON public.support_audit_models;
DROP TRIGGER IF EXISTS support_audit_no_truncate ON public.support_audit_models;
CREATE TRIGGER support_audit_append_only BEFORE UPDATE OR DELETE ON public.support_audit_models
FOR EACH ROW EXECUTE FUNCTION public.reject_audit_event_mutation();
CREATE TRIGGER support_audit_no_truncate BEFORE TRUNCATE ON public.support_audit_models
FOR EACH STATEMENT EXECUTE FUNCTION public.reject_audit_event_mutation();
REVOKE ALL ON public.platform_admin_models,public.support_request_models,public.support_message_models,public.support_notice_models,public.support_audit_models FROM PUBLIC,app;
GRANT SELECT ON public.platform_admin_models TO app;
GRANT SELECT,INSERT ON public.support_request_models,public.support_message_models TO app;
GRANT SELECT,INSERT,UPDATE(read_at) ON public.support_notice_models TO app;
GRANT INSERT ON public.support_audit_models TO app;
GRANT USAGE ON SEQUENCE public.support_audit_models_id_seq TO app;

-- Operator-only bootstrap: verify immutable identity, persist role, enqueue its
-- SpiceDB grant, and audit atomically. Email never authorizes this function.
CREATE FUNCTION public.grant_platform_admin(account_id text, expected_issuer text, expected_subject text)
RETURNS void LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 PERFORM 1 FROM public.user_models u JOIN public.identity_models i ON i.user_id=u.id
 WHERE u.id=account_id AND u.status='active' AND i.issuer=expected_issuer AND i.subject=expected_subject FOR UPDATE OF u;
 IF NOT FOUND THEN RAISE EXCEPTION 'account identity unavailable'; END IF;
 INSERT INTO public.platform_admin_models(user_id,granted_at,granted_by) VALUES(account_id,clock_timestamp(),session_user) ON CONFLICT DO NOTHING;
 INSERT INTO public.authorization_outbox_models(id,resource_type,resource_id,relation,subject_type,subject_id,owner_user_id,actor_user_id,operation,created_at)
 VALUES(gen_random_uuid()::text,'resource','hourpaths-platform-admin','owner','user',account_id,account_id,account_id,'touch',clock_timestamp());
 INSERT INTO public.support_audit_models(request_id,actor,operation,occurred_at) VALUES('platform-admin',session_user,'grant',clock_timestamp());
END $$;
REVOKE ALL ON FUNCTION public.grant_platform_admin(text,text,text) FROM PUBLIC,app,app_audit_retention;
CREATE TABLE public.admin_attention_models (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
 recipient_id text NOT NULL REFERENCES public.platform_admin_models(user_id) ON DELETE CASCADE,
 case_id text REFERENCES public.moderation_case_models(id) ON DELETE CASCADE,
 enforcement_id text REFERENCES public.moderation_appeal_models(enforcement_id) ON DELETE CASCADE,
 created_at timestamptz NOT NULL, read_at timestamptz,
 CHECK ((case_id IS NULL) <> (enforcement_id IS NULL)),
 UNIQUE(recipient_id,case_id), UNIQUE(recipient_id,enforcement_id)
);
CREATE INDEX admin_attention_recipient_idx ON public.admin_attention_models(recipient_id,created_at DESC);
REVOKE ALL ON public.admin_attention_models FROM PUBLIC,app,app_audit_retention;
GRANT SELECT, UPDATE(read_at) ON public.admin_attention_models TO app;
CREATE FUNCTION public.notify_platform_moderation() RETURNS trigger
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
BEGIN
 IF TG_TABLE_NAME='moderation_case_models' THEN
  INSERT INTO public.admin_attention_models(recipient_id,case_id,created_at)
  SELECT a.user_id,NEW.id,NEW.created_at FROM public.platform_admin_models a
  JOIN public.user_models u ON u.id=a.user_id AND u.status='active' ON CONFLICT DO NOTHING;
 ELSE
  INSERT INTO public.admin_attention_models(recipient_id,enforcement_id,created_at)
  SELECT a.user_id,NEW.enforcement_id,NEW.submitted_at FROM public.platform_admin_models a
  JOIN public.user_models u ON u.id=a.user_id AND u.status='active' ON CONFLICT DO NOTHING;
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION public.notify_platform_moderation() FROM PUBLIC,app,app_audit_retention;
CREATE TRIGGER platform_report_notice AFTER INSERT ON public.moderation_case_models
FOR EACH ROW EXECUTE FUNCTION public.notify_platform_moderation();
CREATE TRIGGER platform_appeal_notice AFTER INSERT ON public.moderation_appeal_models
FOR EACH ROW EXECUTE FUNCTION public.notify_platform_moderation();

-- Narrow capability: current admin plus a notice addressed to that admin. This
-- does not grant SELECT on case evidence, enforcement, or operator mutations.
CREATE FUNCTION public.read_admin_attention(actor_id text, notice_id uuid, read_time timestamptz)
RETURNS jsonb LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public AS $$
DECLARE notice public.admin_attention_models; result jsonb;
BEGIN
 IF NOT EXISTS(SELECT 1 FROM public.platform_admin_models a JOIN public.user_models u ON u.id=a.user_id AND u.status='active' WHERE a.user_id=actor_id) THEN RETURN NULL; END IF;
 SELECT * INTO notice FROM public.admin_attention_models n WHERE n.id=notice_id AND n.recipient_id=actor_id FOR UPDATE;
 IF NOT FOUND THEN RETURN NULL; END IF;
 IF notice.case_id IS NOT NULL THEN
  SELECT jsonb_build_object('id',notice.id,'kind','report','reason',c.reason,'explanation',c.explanation,'state',c.state,'createdAt',c.created_at) INTO result
  FROM public.moderation_case_models c WHERE c.id=notice.case_id;
 ELSE
  SELECT jsonb_build_object('id',notice.id,'kind','appeal','reason',e.policy_reason,'explanation',a.explanation,'state',coalesce(d.outcome,'pending'),'createdAt',a.submitted_at) INTO result
  FROM public.moderation_appeal_models a JOIN public.moderation_enforcement_models e ON e.id=a.enforcement_id
  LEFT JOIN public.moderation_appeal_decision_models d ON d.enforcement_id=a.enforcement_id WHERE a.enforcement_id=notice.enforcement_id;
 END IF;
 IF result IS NULL THEN RETURN NULL; END IF;
 UPDATE public.admin_attention_models SET read_at=read_time WHERE id=notice_id;
 INSERT INTO public.support_audit_models(request_id,actor,operation,occurred_at) VALUES(notice.id::text,actor_id,'read',read_time);
 RETURN result;
END $$;
REVOKE ALL ON FUNCTION public.read_admin_attention(text,uuid,timestamptz) FROM PUBLIC,app_audit_retention;
GRANT EXECUTE ON FUNCTION public.read_admin_attention(text,uuid,timestamptz) TO app;

COMMIT;
