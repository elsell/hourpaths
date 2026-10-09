BEGIN;
DROP TABLE public.user_profile_mutation_models;
ALTER TABLE public.user_models DROP COLUMN profile_revision;
COMMIT;
