BEGIN;
CREATE TABLE public.offline_timer_state_models (
  participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  client_timer_id text NOT NULL,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  identity_hash bytea NOT NULL CHECK (octet_length(identity_hash) = 32),
  canonical_timer_id text NOT NULL UNIQUE,
  outcome text NOT NULL CHECK (outcome IN ('active','stopped','conflict','archived','archive_pending','archive_waiting')),
  activity_id text REFERENCES public.recorded_activity_models(id) ON DELETE SET NULL,
  terminal_at timestamptz,
  saved_seconds bigint NOT NULL DEFAULT 0 CHECK (saved_seconds >= 0),
  PRIMARY KEY (participant_id, client_timer_id)
);
CREATE INDEX offline_timer_state_path_idx ON public.offline_timer_state_models(path_id, participant_id);
CREATE TABLE public.offline_timer_replay_models (
  participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  key text NOT NULL CHECK (length(key) BETWEEN 16 AND 128),
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  client_timer_id text NOT NULL,
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  outcome text NOT NULL CHECK (outcome IN ('accepted','conflict','archived')),
  saved_seconds bigint NOT NULL CHECK (saved_seconds >= 0),
  discarded_seconds bigint NOT NULL CHECK (discarded_seconds >= 0),
  PRIMARY KEY (participant_id, key)
);
GRANT SELECT, INSERT, UPDATE ON public.offline_timer_state_models TO app;
GRANT SELECT, INSERT ON public.offline_timer_replay_models TO app;
COMMIT;
