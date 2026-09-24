BEGIN;

CREATE INDEX block_models_blocker_page_idx
  ON public.block_models(blocker_user_id, created_at DESC, blocked_user_id DESC);

CREATE TABLE public.social_block_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation IN ('social.block', 'social.unblock')),
  idempotency_key text NOT NULL CHECK (
    idempotency_key = btrim(idempotency_key) AND idempotency_key <> ''
  ),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  target_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  target_username text NOT NULL CHECK (target_username = btrim(target_username) AND target_username <> ''),
  target_display_name text NOT NULL CHECK (target_display_name = btrim(target_display_name) AND target_display_name <> ''),
  result_blocked boolean NOT NULL,
  changed boolean NOT NULL,
  authorization_change_ids text[] NOT NULL CHECK (
    cardinality(authorization_change_ids) <= 2
    AND array_position(authorization_change_ids, NULL) IS NULL
  ),
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key),
  CHECK (actor_user_id <> target_user_id),
  CHECK ((operation = 'social.block' AND result_blocked)
    OR (operation = 'social.unblock' AND NOT result_blocked)),
  CHECK (changed OR cardinality(authorization_change_ids) = 0)
);

REVOKE ALL ON public.block_models FROM app;
GRANT SELECT, INSERT, DELETE ON public.block_models TO app;
REVOKE ALL ON public.social_block_replay_models FROM app;
GRANT SELECT, INSERT ON public.social_block_replay_models TO app;

COMMIT;
