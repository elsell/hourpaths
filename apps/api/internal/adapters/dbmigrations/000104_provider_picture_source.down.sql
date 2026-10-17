BEGIN;
DROP TRIGGER clear_provider_picture_source_before_status ON public.user_models;
DROP FUNCTION public.clear_provider_picture_source();
ALTER TABLE public.user_models DROP COLUMN provider_picture_source;
COMMIT;
