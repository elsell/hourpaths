BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.moderation_review_event_models) THEN RAISE EXCEPTION 'cannot remove moderation review evidence'; END IF;
END $$;
DROP TRIGGER moderation_case_retention_perform_run ON public.account_deletion_retention_run_models;
DROP FUNCTION public.perform_moderation_case_retention();
ALTER TABLE public.account_deletion_retention_run_models DROP COLUMN moderation_deleted_count;
DROP FUNCTION public.moderation_list_cases(integer),public.moderation_read_case(text),public.moderation_set_case_state(text,text,text,text);
DROP TABLE public.moderation_review_event_models;
COMMIT;
