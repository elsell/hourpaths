BEGIN;
DROP FUNCTION public.lock_account_deletion_receipt(text, bytea, timestamptz);
COMMIT;
