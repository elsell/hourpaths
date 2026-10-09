BEGIN;
CREATE TABLE public.user_policy_renewal_mutation_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 operation text NOT NULL CHECK (operation = 'account.policies.accept'),
 idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND char_length(idempotency_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
 policy_revision bigint NOT NULL CHECK (policy_revision > 0),
 terms_version text NOT NULL CHECK (btrim(terms_version) <> '' AND char_length(terms_version) <= 128),
 privacy_policy_version text NOT NULL CHECK (btrim(privacy_policy_version) <> '' AND char_length(privacy_policy_version) <= 128),
 community_guidelines_version text NOT NULL CHECK (btrim(community_guidelines_version) <> '' AND char_length(community_guidelines_version) <= 128),
 accepted_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, operation, idempotency_key)
);
REVOKE ALL ON public.user_policy_renewal_mutation_models FROM app;
GRANT SELECT, INSERT ON public.user_policy_renewal_mutation_models TO app;
COMMIT;
