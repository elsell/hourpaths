BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.moderation_removed_profile_field_models) THEN RAISE EXCEPTION 'cannot discard profile-field decisions';END IF;
END $$;
DROP FUNCTION public.moderation_decide_profile_fields_appeal(text,text,text,text);
DROP FUNCTION public.moderation_remove_profile_fields(text,text,text,text[],text);
DROP TABLE public.moderation_removed_profile_field_models;
COMMIT;
