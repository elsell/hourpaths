BEGIN;
ALTER TABLE public.authorization_batch_outbox_models
 DROP CONSTRAINT authorization_batch_outbox_models_relationship_updates_check,
 ADD CONSTRAINT authorization_batch_outbox_models_relationship_updates_check
 CHECK (jsonb_typeof(relationship_updates) = 'array' AND jsonb_array_length(relationship_updates) IN (4, 6));
COMMIT;
