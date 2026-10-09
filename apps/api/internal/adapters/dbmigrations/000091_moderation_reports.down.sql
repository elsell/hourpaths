BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.moderation_case_models) OR EXISTS(SELECT 1 FROM public.moderation_report_receipt_models) THEN
  RAISE EXCEPTION 'cannot roll back nonempty moderation reports';
 END IF;
END $$;
DROP TABLE public.moderation_report_receipt_models;
DROP TABLE public.moderation_case_models;
COMMIT;
