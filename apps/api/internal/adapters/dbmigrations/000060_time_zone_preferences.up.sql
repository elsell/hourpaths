BEGIN;

CREATE TABLE public.user_time_zone_preference_mutation_models (
  user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation = 'account.time_zone.update'),
  idempotency_key text NOT NULL CHECK (
    idempotency_key = btrim(idempotency_key)
    AND char_length(idempotency_key) BETWEEN 16 AND 128
  ),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  reviewed_time_zone text NOT NULL CHECK (btrim(reviewed_time_zone) <> ''),
  proposed_time_zone text NOT NULL CHECK (btrim(proposed_time_zone) <> ''),
  result_time_zone text NOT NULL CHECK (btrim(result_time_zone) <> ''),
  result_effective_at timestamptz NOT NULL,
  result_changed boolean NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, operation, idempotency_key)
);

REVOKE ALL ON public.user_time_zone_preference_mutation_models FROM app;
GRANT SELECT, INSERT ON public.user_time_zone_preference_mutation_models TO app;
GRANT UPDATE (current_time_zone, updated_at) ON public.user_preference_models TO app;

COMMIT;
