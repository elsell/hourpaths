BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.social_feed_event_models) THEN
    RAISE EXCEPTION 'cannot remove social feed persistence while feed events exist';
  END IF;
END
$$;

DROP TRIGGER recorded_activity_publish_feed_event ON public.recorded_activity_models;
DROP FUNCTION public.publish_practice_feed_event();
REVOKE ALL ON public.social_feed_event_models FROM app;
DROP TABLE public.social_feed_event_models;
ALTER TABLE public.recorded_activity_models
  DROP CONSTRAINT recorded_activity_models_feed_source_unique;

COMMIT;
