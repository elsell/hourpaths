BEGIN;

ALTER TABLE public.follow_models
  ADD COLUMN activity_notifications_revision bigint NOT NULL DEFAULT 0
    CHECK (activity_notifications_revision >= 0);
GRANT UPDATE (activity_notifications_enabled, activity_notifications_revision)
  ON public.follow_models TO app;

CREATE TABLE public.path_timer_subscription_models (
  user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  enabled boolean NOT NULL,
  revision bigint NOT NULL CHECK (revision > 0),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL CHECK (updated_at >= created_at),
  PRIMARY KEY (user_id, path_id)
);

CREATE TABLE public.timer_subscription_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 128),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  following_user_id text,
  path_id text REFERENCES public.path_models(id) ON DELETE CASCADE,
  result_enabled boolean NOT NULL,
  result_revision bigint NOT NULL CHECK (result_revision > 0),
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, idempotency_key),
  FOREIGN KEY (actor_user_id, following_user_id)
    REFERENCES public.follow_models(follower_user_id, following_user_id) ON DELETE CASCADE,
  CHECK (num_nonnulls(following_user_id, path_id) = 1)
);

REVOKE ALL ON public.path_timer_subscription_models FROM app;
GRANT SELECT, INSERT, UPDATE (enabled, revision, updated_at)
  ON public.path_timer_subscription_models TO app;
REVOKE ALL ON public.timer_subscription_replay_models FROM app;
GRANT SELECT, INSERT ON public.timer_subscription_replay_models TO app;

COMMIT;
