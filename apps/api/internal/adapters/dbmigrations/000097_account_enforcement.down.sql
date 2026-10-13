BEGIN;
DROP FUNCTION public.moderation_decide_account_appeal(text,text,text,text);
-- Preserve the expanded review-event constraint: a rollback must not erase or
-- invalidate existing suspension/ban evidence. Older clients ignore these events.
COMMIT;
