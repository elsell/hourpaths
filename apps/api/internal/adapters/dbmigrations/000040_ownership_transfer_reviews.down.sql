DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.path_ownership_transfer_models) THEN
    RAISE EXCEPTION 'cannot roll back ownership-transfer reviews while transfer evidence exists';
  END IF;
END
$$;

ALTER TABLE public.path_ownership_transfer_models
  DROP CONSTRAINT path_ownership_transfer_reviewed_before_creation,
  DROP COLUMN reviewed_at;
