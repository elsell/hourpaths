BEGIN;

ALTER TABLE public.user_preference_models
  ADD COLUMN home_order_method text NOT NULL DEFAULT 'recent'
    CHECK (home_order_method IN ('recent', 'alphabetical', 'manual')),
  ADD COLUMN home_order_revision bigint NOT NULL DEFAULT 0
    CHECK (home_order_revision >= 0),
  ADD COLUMN home_order_updated_at timestamptz;

CREATE TABLE public.home_path_preference_models (
  user_id text NOT NULL,
  path_id text NOT NULL,
  pinned_position bigint,
  manual_position bigint NOT NULL,
  PRIMARY KEY (user_id, path_id),
  FOREIGN KEY (path_id, user_id)
    REFERENCES public.path_membership_models(path_id, user_id) ON DELETE CASCADE,
  CHECK (pinned_position IS NULL OR pinned_position >= 0),
  CHECK (manual_position >= 0)
);
CREATE UNIQUE INDEX home_path_preference_pinned_position_idx
  ON public.home_path_preference_models(user_id, pinned_position)
  WHERE pinned_position IS NOT NULL;
CREATE UNIQUE INDEX home_path_preference_manual_position_idx
  ON public.home_path_preference_models(user_id, manual_position);

CREATE TABLE public.home_preference_mutation_models (
  user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation = 'home.preferences.update'),
  idempotency_key text NOT NULL CHECK (
    idempotency_key = btrim(idempotency_key)
    AND char_length(idempotency_key) BETWEEN 16 AND 128
  ),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  result_order_method text NOT NULL CHECK (result_order_method IN ('recent', 'alphabetical', 'manual')),
  result_revision bigint NOT NULL CHECK (result_revision > 0),
  result_pinned_path_ids text[] NOT NULL,
  result_manual_path_ids text[] NOT NULL,
  result_updated_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, operation, idempotency_key)
);

CREATE INDEX recorded_activity_models_home_recent_idx
  ON public.recorded_activity_models(participant_id, path_id, ended_at DESC, id);

REVOKE ALL ON public.home_path_preference_models FROM app;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.home_path_preference_models TO app;
REVOKE ALL ON public.home_preference_mutation_models FROM app;
GRANT SELECT, INSERT ON public.home_preference_mutation_models TO app;
GRANT UPDATE (home_order_method, home_order_revision, home_order_updated_at)
  ON public.user_preference_models TO app;

COMMIT;
