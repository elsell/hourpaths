BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.moderation_enforcement_models WHERE affected_feed_event_id IS NOT NULL) THEN
  RAISE EXCEPTION 'cannot discard feed-event removal decisions';
 END IF;
END $$;
DROP FUNCTION public.moderation_decide_feed_event_appeal(text,text,text,text);
DROP FUNCTION public.moderation_remove_feed_event(text,text,text);
DROP TABLE public.moderation_removed_feed_event_models;
ALTER TABLE public.moderation_enforcement_models DROP COLUMN affected_feed_event_id, DROP COLUMN affected_feed_event_published_at;
COMMIT;
