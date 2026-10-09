BEGIN;
ALTER TABLE public.user_models ADD COLUMN picture_revision bigint NOT NULL DEFAULT 1 CHECK (picture_revision > 0);
CREATE TABLE public.user_profile_picture_models (
 id uuid PRIMARY KEY,
 user_id text NOT NULL UNIQUE REFERENCES public.user_models(id) ON DELETE CASCADE,
 jpeg bytea NOT NULL CHECK (octet_length(jpeg) BETWEEN 4 AND 1048576),
 created_at timestamptz NOT NULL
);
CREATE TABLE public.user_profile_picture_mutation_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 idempotency_key text NOT NULL CHECK (length(idempotency_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
 asset_id text NOT NULL,
 url text NOT NULL,
 revision bigint NOT NULL CHECK (revision > 0),
 created_at timestamptz NOT NULL,
 PRIMARY KEY (user_id,idempotency_key)
);
REVOKE ALL ON public.user_profile_picture_models, public.user_profile_picture_mutation_models FROM PUBLIC, app;
GRANT SELECT, INSERT, DELETE ON public.user_profile_picture_models TO app;
GRANT SELECT, INSERT ON public.user_profile_picture_mutation_models TO app;
COMMIT;
