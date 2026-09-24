BEGIN;

ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK (action IN ('user.provisioned', 'user.deactivated', 'user.profile_synchronized', 'user.viewed', 'session.created', 'session.revoked', 'resource.listed', 'resource.viewed', 'resource.created', 'resource.updated', 'resource.deleted', 'resource.access_denied', 'authorization.relationship_applied', 'authorization.relationship_failed', 'authorization.dead_letters_listed', 'authorization.dead_letter_requeued', 'invitation.created', 'invitation.listed', 'invitation.revoked', 'invitation.consumed', 'duplicate_email_recovery.declined'));

CREATE TABLE duplicate_email_recovery_declines (
  provisional_user_id text NOT NULL REFERENCES user_models(id) ON DELETE CASCADE,
  normalized_email text NOT NULL,
  created_at timestamptz NOT NULL,
  CHECK (normalized_email <> '' AND normalized_email = lower(btrim(normalized_email))),
  PRIMARY KEY (provisional_user_id, normalized_email)
);

REVOKE UPDATE, DELETE, TRUNCATE ON duplicate_email_recovery_declines FROM app;
GRANT SELECT, INSERT ON duplicate_email_recovery_declines TO app;

COMMIT;
