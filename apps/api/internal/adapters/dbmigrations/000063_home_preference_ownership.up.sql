BEGIN;

CREATE FUNCTION public.delete_home_path_preference_after_membership()
RETURNS trigger
LANGUAGE plpgsql
SET search_path = pg_catalog, public
AS $$
BEGIN
  DELETE FROM public.home_path_preference_models
  WHERE path_id = OLD.path_id AND user_id = OLD.user_id;
  RETURN OLD;
END;
$$;

REVOKE ALL ON FUNCTION public.delete_home_path_preference_after_membership() FROM PUBLIC;

CREATE TRIGGER delete_home_path_preference_after_membership
AFTER DELETE ON public.path_membership_models
FOR EACH ROW
EXECUTE FUNCTION public.delete_home_path_preference_after_membership();

ALTER TABLE public.home_path_preference_models
  DROP CONSTRAINT home_path_preference_models_path_id_user_id_fkey,
  ADD CONSTRAINT home_path_preference_models_path_id_fkey
    FOREIGN KEY (path_id) REFERENCES public.path_models(id) ON DELETE CASCADE,
  ADD CONSTRAINT home_path_preference_models_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES public.user_models(id) ON DELETE CASCADE;

COMMIT;
