BEGIN;

CREATE TABLE public.authorization_batch_outbox_models (
  id text PRIMARY KEY,
  path_ownership_transfer_id text NOT NULL UNIQUE
    REFERENCES public.path_ownership_transfer_models(id) ON DELETE RESTRICT,
  resource_type text NOT NULL CHECK (resource_type = 'path'),
  resource_id text NOT NULL REFERENCES public.path_models(id) ON DELETE RESTRICT,
  owner_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE RESTRICT,
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE RESTRICT,
  relationship_updates jsonb NOT NULL CHECK (
    jsonb_typeof(relationship_updates) = 'array'
    AND jsonb_array_length(relationship_updates) = 4
  ),
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  locked_by text,
  locked_until timestamptz,
  completed_at timestamptz,
  failure_code text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CHECK ((locked_by IS NULL) = (locked_until IS NULL))
);

CREATE OR REPLACE FUNCTION public.order_authorization_outbox_change()
RETURNS trigger
LANGUAGE plpgsql
SECURITY INVOKER
SET search_path = pg_catalog, public
AS $$
DECLARE
    previous_created_at timestamptz;
BEGIN
    INSERT INTO public.authorization_resource_lock_models (resource_type, resource_id)
    VALUES (NEW.resource_type, NEW.resource_id)
    ON CONFLICT DO NOTHING;

    PERFORM 1
    FROM public.authorization_resource_lock_models
    WHERE resource_type = NEW.resource_type
      AND resource_id = NEW.resource_id
    FOR UPDATE;

    SELECT max(created_at)
    INTO previous_created_at
    FROM (
      SELECT created_at FROM public.authorization_outbox_models
      WHERE resource_type = NEW.resource_type AND resource_id = NEW.resource_id
      UNION ALL
      SELECT created_at FROM public.authorization_batch_outbox_models
      WHERE resource_type = NEW.resource_type AND resource_id = NEW.resource_id
    ) AS resource_changes;

    NEW.created_at := GREATEST(
        clock_timestamp(),
        COALESCE(previous_created_at + INTERVAL '1 microsecond', '-infinity'::timestamptz)
    );
    RETURN NEW;
END;
$$;

CREATE TRIGGER authorization_batch_outbox_ordering
BEFORE INSERT ON public.authorization_batch_outbox_models
FOR EACH ROW EXECUTE FUNCTION public.order_authorization_outbox_change();

CREATE INDEX authorization_batch_outbox_claim_idx
  ON public.authorization_batch_outbox_models(created_at, id)
  WHERE completed_at IS NULL;

REVOKE ALL ON public.authorization_batch_outbox_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.authorization_batch_outbox_models TO app;

COMMIT;
