BEGIN;

-- Prevent an activity accepted by the prior runtime from crossing the
-- backfill boundary without a corresponding feed event. After this migration
-- commits, the trigger covers both old and new runtime revisions.
LOCK TABLE public.recorded_activity_models IN SHARE ROW EXCLUSIVE MODE;

ALTER TABLE public.recorded_activity_models
  ADD CONSTRAINT recorded_activity_models_feed_source_unique
  UNIQUE (id, participant_id, path_id);

CREATE TABLE public.social_feed_event_models (
  id text PRIMARY KEY CHECK (id = btrim(id) AND id <> ''),
  source_activity_id text NOT NULL UNIQUE,
  participant_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  published_at timestamptz NOT NULL,
  FOREIGN KEY (source_activity_id, participant_user_id, path_id)
    REFERENCES public.recorded_activity_models(id, participant_id, path_id)
    ON DELETE CASCADE,
  CHECK (id = 'practice:' || source_activity_id)
);
CREATE INDEX social_feed_event_models_chronological_idx
  ON public.social_feed_event_models(published_at DESC, id DESC);

CREATE FUNCTION public.publish_practice_feed_event()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  INSERT INTO public.social_feed_event_models (
    id, source_activity_id, participant_user_id, path_id, published_at
  ) VALUES (
    'practice:' || NEW.id, NEW.id, NEW.participant_id, NEW.path_id, NEW.created_at
  )
  ON CONFLICT (source_activity_id) DO NOTHING;
  RETURN NEW;
END
$$;

CREATE TRIGGER recorded_activity_publish_feed_event
AFTER INSERT ON public.recorded_activity_models
FOR EACH ROW EXECUTE FUNCTION public.publish_practice_feed_event();

INSERT INTO public.social_feed_event_models (
  id, source_activity_id, participant_user_id, path_id, published_at
)
SELECT 'practice:' || activity.id, activity.id, activity.participant_id, activity.path_id, activity.created_at
FROM public.recorded_activity_models AS activity
ON CONFLICT (source_activity_id) DO NOTHING;

REVOKE ALL ON public.social_feed_event_models FROM app;
GRANT SELECT, INSERT ON public.social_feed_event_models TO app;

COMMIT;
