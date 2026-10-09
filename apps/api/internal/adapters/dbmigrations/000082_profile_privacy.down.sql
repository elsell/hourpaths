BEGIN;
DROP TABLE public.user_profile_privacy_mutation_models;
ALTER TABLE public.user_models DROP COLUMN profile_privacy_revision;
COMMIT;
