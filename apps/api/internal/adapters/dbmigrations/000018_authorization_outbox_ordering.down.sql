DROP INDEX IF EXISTS public.authorization_outbox_resource_created_idx;
DROP INDEX IF EXISTS public.authorization_outbox_owner_dead_letter_idx;
DROP TRIGGER IF EXISTS authorization_outbox_ordering ON public.authorization_outbox_models;
DROP FUNCTION IF EXISTS public.order_authorization_outbox_change();
