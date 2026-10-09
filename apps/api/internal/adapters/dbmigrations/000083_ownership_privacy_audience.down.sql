BEGIN;
-- Refuse downgrade if an audience-bearing transfer batch still exists.
ALTER TABLE public.authorization_batch_outbox_models
 DROP CONSTRAINT authorization_batch_outbox_models_relationship_updates_check,
 ADD CONSTRAINT authorization_batch_outbox_models_relationship_updates_check
 CHECK (jsonb_typeof(relationship_updates) = 'array' AND jsonb_array_length(relationship_updates) = 4);
COMMIT;
