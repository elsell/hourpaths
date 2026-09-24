BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.follow_models)
    OR EXISTS (SELECT 1 FROM public.block_models)
    OR EXISTS (
      SELECT 1
      FROM public.user_models
      WHERE description IS NOT NULL OR profile_picture_url IS NOT NULL
    ) THEN
    RAISE EXCEPTION 'cannot remove social profile discovery persistence while profile or relationship data exists';
  END IF;
END
$$;

DROP TABLE public.block_models;
DROP TABLE public.follow_models;
ALTER TABLE public.user_models
  DROP CONSTRAINT user_models_profile_picture_url_check,
  DROP CONSTRAINT user_models_description_check,
  DROP COLUMN profile_picture_url,
  DROP COLUMN description;

COMMIT;
