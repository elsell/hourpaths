ALTER TABLE public.path_ownership_transfer_models
  ADD COLUMN reviewed_at timestamptz;

UPDATE public.path_ownership_transfer_models
SET reviewed_at = created_at
WHERE reviewed_at IS NULL;

ALTER TABLE public.path_ownership_transfer_models
  ALTER COLUMN reviewed_at SET NOT NULL,
  ADD CONSTRAINT path_ownership_transfer_reviewed_before_creation
    CHECK (reviewed_at <= created_at);
