BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.offline_activity_replay_models)
 OR EXISTS (SELECT 1 FROM public.activity_edit_order_models) THEN
  RAISE EXCEPTION 'activity replay and causal order cannot be discarded by rollback';
 END IF;
END $$;
DROP TABLE public.offline_activity_replay_models;
DROP TABLE public.activity_edit_order_models;
ALTER TABLE public.recorded_activity_revision_models DROP COLUMN superseded;
COMMIT;
