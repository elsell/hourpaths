BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.moderation_enforcement_models WHERE affected_path_id IS NOT NULL) THEN RAISE EXCEPTION 'cannot discard path-name decisions';END IF;
END $$;
DROP FUNCTION public.moderation_decide_path_name_appeal(text,text,text,text);
DROP FUNCTION public.moderation_remove_path_name(text,text,text,text);
DROP TABLE public.moderation_removed_path_name_models;
ALTER TABLE public.moderation_enforcement_models DROP COLUMN affected_path_id;
DROP TRIGGER path_name_revision ON public.path_models;
DROP FUNCTION public.advance_path_name_revision();
ALTER TABLE public.path_models DROP COLUMN name_revision;
COMMIT;
