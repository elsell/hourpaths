BEGIN;

LOCK TABLE public.authorization_outbox_models IN SHARE ROW EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM public.authorization_outbox_models
        WHERE completed_at IS NULL
    ) THEN
        RAISE EXCEPTION 'migration 18 requires every authorization outbox row to be completed; drain authorization delivery before retrying'
            USING ERRCODE = '55000';
    END IF;
END;
$$;

CREATE FUNCTION public.order_authorization_outbox_change()
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
    FROM public.authorization_outbox_models
    WHERE resource_type = NEW.resource_type
      AND resource_id = NEW.resource_id;

    NEW.created_at := GREATEST(
        clock_timestamp(),
        COALESCE(previous_created_at + INTERVAL '1 microsecond', '-infinity'::timestamptz)
    );
    RETURN NEW;
END;
$$;

REVOKE ALL ON FUNCTION public.order_authorization_outbox_change() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.order_authorization_outbox_change() TO app;

CREATE TRIGGER authorization_outbox_ordering
BEFORE INSERT ON public.authorization_outbox_models
FOR EACH ROW EXECUTE FUNCTION public.order_authorization_outbox_change();

CREATE INDEX authorization_outbox_owner_dead_letter_idx
ON public.authorization_outbox_models(owner_user_id, dead_lettered_at DESC, id)
WHERE dead_lettered_at IS NOT NULL AND completed_at IS NULL;

CREATE INDEX authorization_outbox_resource_created_idx
ON public.authorization_outbox_models(resource_type, resource_id, created_at DESC);

COMMIT;
