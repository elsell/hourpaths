BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.home_path_preference_models preference
    LEFT JOIN public.path_membership_models member
      ON member.path_id = preference.path_id AND member.user_id = preference.user_id
    WHERE member.path_id IS NULL
  ) THEN
    RAISE EXCEPTION 'cannot roll back Home preference ownership while preferences without membership exist';
  END IF;
END
$$;

ALTER TABLE public.home_path_preference_models
  DROP CONSTRAINT home_path_preference_models_path_id_fkey,
  DROP CONSTRAINT home_path_preference_models_user_id_fkey,
  ADD CONSTRAINT home_path_preference_models_path_id_user_id_fkey
    FOREIGN KEY (path_id, user_id)
    REFERENCES public.path_membership_models(path_id, user_id) ON DELETE CASCADE;

DROP TRIGGER delete_home_path_preference_after_membership ON public.path_membership_models;
DROP FUNCTION public.delete_home_path_preference_after_membership();

COMMIT;
