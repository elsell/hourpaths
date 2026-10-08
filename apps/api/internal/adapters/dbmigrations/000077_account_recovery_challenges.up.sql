BEGIN;
CREATE TABLE public.account_recovery_challenge_models (
 user_id text PRIMARY KEY REFERENCES public.user_models(id) ON DELETE CASCADE,
 id text NOT NULL UNIQUE,
 source_issuer text NOT NULL CHECK (source_issuer <> ''),
 source_subject text NOT NULL CHECK (source_subject <> ''),
 source_provider text NOT NULL CHECK (source_provider IN ('google','apple')),
 nonce_hash bytea NOT NULL CHECK (octet_length(nonce_hash) = 32),
 session_hash bytea NOT NULL CHECK (octet_length(session_hash) = 32),
 created_at timestamptz NOT NULL,
 expires_at timestamptz NOT NULL,
 CHECK (expires_at = created_at + interval '10 minutes')
);
REVOKE ALL ON public.account_recovery_challenge_models FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.account_recovery_challenge_models TO app;
COMMIT;
