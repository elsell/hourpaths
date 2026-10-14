BEGIN;
CREATE TABLE public.live_activity_registration_models (
 id text PRIMARY KEY,
 owner_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 account_lease text NOT NULL,
 token_ciphertext bytea,
 token_nonce bytea,
 token_hash bytea,
 boundaries_json jsonb NOT NULL CHECK (jsonb_typeof(boundaries_json)='array' AND jsonb_array_length(boundaries_json)<=64),
 version bigint NOT NULL CHECK (version>0),
 failures integer NOT NULL DEFAULT 0 CHECK (failures>=0),
 expires_at timestamptz NOT NULL,
 available_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 updated_at timestamptz NOT NULL,
 revoked_at timestamptz,
 locked_by text,
 locked_until timestamptz,
 CHECK (expires_at>created_at AND expires_at<=created_at+interval '8 hours'),
 CHECK ((locked_by IS NULL)=(locked_until IS NULL)),
 CHECK ((revoked_at IS NULL AND token_ciphertext IS NOT NULL AND token_nonce IS NOT NULL AND token_hash IS NOT NULL AND octet_length(token_ciphertext)>16 AND octet_length(token_nonce)=12 AND octet_length(token_hash)=32)
 OR (revoked_at IS NOT NULL AND token_ciphertext IS NULL AND token_nonce IS NULL AND token_hash IS NULL))
);
CREATE UNIQUE INDEX live_activity_registration_token_idx ON public.live_activity_registration_models(token_hash) WHERE revoked_at IS NULL;
CREATE INDEX live_activity_registration_owner_idx ON public.live_activity_registration_models(owner_user_id);
CREATE INDEX live_activity_registration_due_idx ON public.live_activity_registration_models(available_at,locked_until);
REVOKE ALL ON public.live_activity_registration_models FROM PUBLIC,app,app_audit_retention;
GRANT SELECT,INSERT,UPDATE,DELETE ON public.live_activity_registration_models TO app;
COMMIT;
