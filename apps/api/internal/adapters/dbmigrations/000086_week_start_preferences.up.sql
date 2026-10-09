BEGIN;
CREATE TABLE public.user_week_start_preference_mutation_models (
 user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 operation text NOT NULL CHECK (operation = 'account.week_start.update'),
 idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND char_length(idempotency_key) BETWEEN 16 AND 128),
 request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
 reviewed_first_day_of_week smallint NOT NULL CHECK (reviewed_first_day_of_week BETWEEN 1 AND 7),
 proposed_first_day_of_week smallint NOT NULL CHECK (proposed_first_day_of_week BETWEEN 1 AND 7),
 result_first_day_of_week smallint NOT NULL CHECK (result_first_day_of_week BETWEEN 1 AND 7),
 result_changed boolean NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY (user_id, operation, idempotency_key)
);
REVOKE ALL ON public.user_week_start_preference_mutation_models FROM app;
GRANT SELECT, INSERT ON public.user_week_start_preference_mutation_models TO app;
GRANT UPDATE (first_day_of_week) ON public.user_preference_models TO app;
COMMIT;
