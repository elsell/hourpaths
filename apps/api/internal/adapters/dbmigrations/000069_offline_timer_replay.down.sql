BEGIN;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM public.offline_timer_state_models)
     OR EXISTS (SELECT 1 FROM public.offline_timer_replay_models) THEN
    RAISE EXCEPTION 'offline timer state cannot be discarded by rollback';
  END IF;
END $$;
DROP TABLE public.offline_timer_replay_models;
DROP TABLE public.offline_timer_state_models;
COMMIT;
