BEGIN;

ALTER TABLE public.path_invitation_models
  ADD COLUMN rejection_unread_count bigint;

UPDATE public.path_invitation_models
SET rejection_unread_count = 0
WHERE rejected_at IS NOT NULL;

ALTER TABLE public.path_invitation_models
  ADD CONSTRAINT path_invitation_models_rejection_result_check CHECK (
    (rejected_at IS NULL) = (rejection_unread_count IS NULL)
    AND (rejection_unread_count IS NULL OR rejection_unread_count >= 0)
  );

COMMIT;
