BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.path_models
    WHERE archived_at IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot remove Path archival state while archived Paths exist';
  END IF;
END
$$;

DROP INDEX public.path_models_archived_page_idx;
DROP INDEX public.path_models_active_page_idx;

ALTER TABLE public.path_models
  DROP CONSTRAINT path_models_archive_time_check,
  DROP COLUMN archived_at;

COMMIT;
