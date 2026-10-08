BEGIN;
ALTER TABLE public.identity_models DROP CONSTRAINT identity_models_user_id_key;
ALTER TABLE public.identity_models ADD COLUMN provider text NOT NULL DEFAULT '' CHECK (provider IN ('', 'google', 'apple'));
CREATE INDEX identity_models_user_id_idx ON public.identity_models(user_id);
CREATE UNIQUE INDEX identity_models_user_provider_key ON public.identity_models(user_id,provider) WHERE provider <> '';
CREATE TABLE public.identity_link_challenge_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 provider text NOT NULL CHECK (provider IN ('google','apple')),
 id text NOT NULL UNIQUE,
 nonce_hash bytea NOT NULL CHECK (octet_length(nonce_hash)=32),
 created_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL,
 PRIMARY KEY(user_id,provider),
 CHECK (expires_at = created_at + interval '10 minutes')
);
REVOKE ALL ON public.identity_link_challenge_models FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.identity_link_challenge_models TO app;
COMMIT;
