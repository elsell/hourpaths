BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.support_request_models) OR EXISTS(SELECT 1 FROM public.platform_admin_models) THEN
  RAISE EXCEPTION 'private support has persisted data; forward repair required';
 END IF;
END $$;
DROP TRIGGER platform_report_notice ON public.moderation_case_models;
DROP TRIGGER platform_appeal_notice ON public.moderation_appeal_models;
DROP FUNCTION public.notify_platform_moderation();
DROP FUNCTION public.read_admin_attention(text,uuid,timestamptz);
DROP TABLE public.admin_attention_models;
DROP FUNCTION public.grant_platform_admin(text,text,text);
DROP TABLE public.support_notice_models,public.support_message_models,public.support_request_models,public.platform_admin_models;
-- Audit history is retained across rollback.
COMMIT;
