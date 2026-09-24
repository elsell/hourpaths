BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.social_block_replay_models) THEN
    RAISE EXCEPTION 'cannot remove social blocking while replay records exist';
  END IF;
END
$$;

REVOKE ALL ON public.social_block_replay_models FROM app;
DROP TABLE public.social_block_replay_models;
DROP INDEX public.block_models_blocker_page_idx;
REVOKE INSERT, DELETE ON public.block_models FROM app;
GRANT SELECT ON public.block_models TO app;

COMMIT;
