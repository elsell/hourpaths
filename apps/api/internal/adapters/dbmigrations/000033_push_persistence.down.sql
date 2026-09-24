BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.notification_push_delivery_models)
    OR EXISTS (SELECT 1 FROM public.push_installation_models)
  THEN
    RAISE EXCEPTION 'cannot remove push persistence while delivery evidence exists';
  END IF;
END
$$;

DROP TABLE public.notification_push_delivery_models;
DROP FUNCTION public.validate_push_delivery_installation();
DROP TABLE public.push_installation_models;

COMMIT;
