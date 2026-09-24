BEGIN;

LOCK TABLE public.path_models IN SHARE ROW EXCLUSIVE MODE;
LOCK TABLE public.authorization_outbox_models IN SHARE ROW EXCLUSIVE MODE;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.authorization_outbox_models
    WHERE relation = 'public_viewer'
      AND (completed_at IS NOT NULL OR locked_until IS NOT NULL OR attempts > 0)
  ) THEN
    RAISE EXCEPTION 'cannot remove public Path view authorization after delivery may have begun'
      USING ERRCODE = '55000';
  END IF;
END;
$$;

DELETE FROM public.authorization_outbox_models
WHERE relation = 'public_viewer'
  AND completed_at IS NULL
  AND locked_until IS NULL
  AND attempts = 0;

DROP TRIGGER authorization_outbox_publish_public_path_viewer
  ON public.authorization_outbox_models;
DROP FUNCTION public.publish_public_path_viewer_authorization();

COMMIT;
