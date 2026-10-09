BEGIN;
ALTER TABLE public.user_models ADD COLUMN profile_privacy_revision bigint NOT NULL DEFAULT 1 CHECK (profile_privacy_revision > 0);
CREATE TABLE public.user_profile_privacy_mutation_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
 visibility text NOT NULL CHECK (visibility IN ('public', 'private')),
 revision bigint NOT NULL CHECK (revision > 0),
 affected_path_ids jsonb NOT NULL CHECK (jsonb_typeof(affected_path_ids) = 'array'),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, idempotency_key)
);
REVOKE ALL ON public.user_profile_privacy_mutation_models FROM PUBLIC, app;
GRANT SELECT, INSERT ON public.user_profile_privacy_mutation_models TO app;
COMMIT;
