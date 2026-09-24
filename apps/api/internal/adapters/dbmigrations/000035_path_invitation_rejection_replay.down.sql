BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM public.path_invitation_models
    WHERE rejection_unread_count IS NOT NULL
  ) THEN
    RAISE EXCEPTION 'cannot remove Path invitation rejection replay results while evidence exists';
  END IF;
END
$$;

ALTER TABLE public.path_invitation_models
  DROP CONSTRAINT path_invitation_models_rejection_result_check;
ALTER TABLE public.path_invitation_models
  DROP COLUMN rejection_unread_count;

COMMIT;
