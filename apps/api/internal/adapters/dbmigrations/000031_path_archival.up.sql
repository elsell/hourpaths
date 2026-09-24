BEGIN;

ALTER TABLE public.path_models
  ADD COLUMN archived_at timestamptz,
  ADD CONSTRAINT path_models_archive_time_check CHECK (
    archived_at IS NULL OR archived_at >= created_at
  );

CREATE INDEX path_models_active_page_idx
  ON public.path_models(created_at, id)
  WHERE archived_at IS NULL;

CREATE INDEX path_models_archived_page_idx
  ON public.path_models(archived_at DESC, id)
  WHERE archived_at IS NOT NULL;

COMMIT;
