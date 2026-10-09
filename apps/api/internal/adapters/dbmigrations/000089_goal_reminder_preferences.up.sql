BEGIN;
CREATE TABLE public.goal_reminder_preference_models (
 participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
 enabled boolean NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL CHECK (updated_at >= created_at),
 PRIMARY KEY (participant_id,path_id)
);
CREATE TABLE public.goal_reminder_preference_replay_models (
 participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 128),
 path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
 request_hash bytea NOT NULL CHECK (octet_length(request_hash)=32),
 enabled boolean NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (participant_id,idempotency_key)
);
REVOKE ALL ON public.goal_reminder_preference_models,public.goal_reminder_preference_replay_models FROM app;
GRANT SELECT,INSERT,UPDATE (enabled,revision,updated_at) ON public.goal_reminder_preference_models TO app;
GRANT SELECT,INSERT ON public.goal_reminder_preference_replay_models TO app;
COMMIT;
