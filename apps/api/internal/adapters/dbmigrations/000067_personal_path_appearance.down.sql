BEGIN;
DROP TRIGGER IF EXISTS delete_path_appearance_after_membership ON public.path_membership_models;
DROP FUNCTION IF EXISTS public.delete_path_appearance_after_membership();
DROP TABLE IF EXISTS public.path_appearance_mutation_models;
DROP TABLE IF EXISTS public.path_appearance_models;
COMMIT;
