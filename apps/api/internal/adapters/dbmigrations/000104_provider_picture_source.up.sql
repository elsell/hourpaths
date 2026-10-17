BEGIN;
ALTER TABLE public.user_models ADD COLUMN provider_picture_source text NOT NULL DEFAULT '';
ALTER TABLE public.user_models ADD CONSTRAINT provider_picture_source_provisional
 CHECK (length(provider_picture_source) <= 4096 AND (status = 'provisional' OR provider_picture_source = ''));
CREATE FUNCTION public.clear_provider_picture_source() RETURNS trigger
LANGUAGE plpgsql SET search_path=pg_catalog,public AS $$
BEGIN
 IF NEW.status <> 'provisional' THEN NEW.provider_picture_source := ''; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER clear_provider_picture_source_before_status
BEFORE UPDATE OF status ON public.user_models
FOR EACH ROW EXECUTE FUNCTION public.clear_provider_picture_source();
COMMIT;
