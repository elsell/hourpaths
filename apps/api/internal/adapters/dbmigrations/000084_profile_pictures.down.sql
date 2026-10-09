BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.user_profile_picture_models) OR EXISTS (SELECT 1 FROM public.user_profile_picture_mutation_models) THEN
  RAISE EXCEPTION 'profile picture data must not be discarded by downgrade';
 END IF;
END $$;
DROP TABLE public.user_profile_picture_mutation_models;
DROP TABLE public.user_profile_picture_models;
ALTER TABLE public.user_models DROP COLUMN picture_revision;
COMMIT;
