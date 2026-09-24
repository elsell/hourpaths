BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.path_ownership_transfer_models) THEN
    RAISE EXCEPTION 'cannot remove Path ownership transfer state while evidence exists';
  END IF;
END
$$;

DROP TABLE public.path_ownership_transfer_models;

COMMIT;
