BEGIN;
CREATE TABLE public.user_unavailable_period_models (
 user_id text PRIMARY KEY REFERENCES public.user_models(id) ON DELETE CASCADE,
 enabled boolean NOT NULL,
 start_minute smallint NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
 end_minute smallint NOT NULL CHECK (end_minute BETWEEN 0 AND 1439),
 revision bigint NOT NULL CHECK (revision > 0),
 updated_at timestamptz NOT NULL
);
CREATE TABLE public.user_unavailable_period_mutation_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 operation text NOT NULL CHECK (operation = 'account.unavailable_period.update'),
 idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND char_length(idempotency_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
 result_enabled boolean NOT NULL,
 result_start_minute smallint NOT NULL CHECK (result_start_minute BETWEEN 0 AND 1439),
 result_end_minute smallint NOT NULL CHECK (result_end_minute BETWEEN 0 AND 1439),
 result_revision bigint NOT NULL CHECK (result_revision > 0),
 result_time_zone text NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, operation, idempotency_key)
);
REVOKE ALL ON public.user_unavailable_period_models, public.user_unavailable_period_mutation_models FROM app;
GRANT SELECT, INSERT ON public.user_unavailable_period_models, public.user_unavailable_period_mutation_models TO app;
GRANT UPDATE (enabled, start_minute, end_minute, revision, updated_at) ON public.user_unavailable_period_models TO app;
COMMIT;
