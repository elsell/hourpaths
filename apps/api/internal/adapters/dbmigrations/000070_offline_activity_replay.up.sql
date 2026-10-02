BEGIN;
ALTER TABLE public.recorded_activity_revision_models ADD COLUMN superseded boolean NOT NULL DEFAULT false;
CREATE TABLE public.activity_edit_order_models (
 activity_id text PRIMARY KEY REFERENCES public.recorded_activity_models(id) ON DELETE CASCADE,
 authored_at timestamptz NOT NULL,
 counter bigint NOT NULL CHECK (counter BETWEEN 0 AND 9007199254740991),
 operation_id text NOT NULL CHECK (length(operation_id) BETWEEN 1 AND 128)
);
CREATE TABLE public.offline_activity_replay_models (
 participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 key text NOT NULL CHECK (length(key) BETWEEN 16 AND 128),
 path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
 activity_id text NOT NULL,
 request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
 outcome text NOT NULL CHECK (outcome IN ('accepted','deleted','archived')),
 PRIMARY KEY (participant_id,key)
);
CREATE INDEX offline_activity_identity_idx ON public.offline_activity_replay_models(participant_id,path_id,activity_id);
GRANT SELECT, INSERT, UPDATE ON public.activity_edit_order_models TO app;
GRANT SELECT, INSERT ON public.offline_activity_replay_models TO app;
COMMIT;
