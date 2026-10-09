BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.moderation_enforcement_models WHERE action='content_removal') THEN
  RAISE EXCEPTION 'cannot discard content-removal decisions';
 END IF;
END $$;
DROP FUNCTION public.moderation_decide_comment_appeal(text,text,text,text);
DROP FUNCTION public.moderation_remove_comment(text,text,text);
DROP TABLE public.moderation_removed_comment_models;
ALTER TABLE public.moderation_enforcement_models DROP COLUMN affected_comment_id, DROP COLUMN affected_comment_created_at;
ALTER TABLE public.moderation_review_event_models DROP CONSTRAINT moderation_review_event_models_operation_check;
ALTER TABLE public.moderation_review_event_models ADD CONSTRAINT moderation_review_event_models_operation_check
 CHECK(operation IN ('list','read','reviewing','dismissed','warning','appeal_upheld','appeal_reversed'));
COMMIT;
