BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.audit_event_models
    WHERE action LIKE 'path_ownership_transfer.%'
  ) THEN
    RAISE EXCEPTION 'cannot remove Path ownership transfer audit taxonomy while evidence exists';
  END IF;
END
$$;

ALTER TABLE public.audit_event_models
  DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE public.audit_event_models
  ADD CONSTRAINT audit_event_models_action_check CHECK (
    action IN (
      'user.provisioned', 'user.deactivated', 'user.profile_synchronized',
      'user.viewed', 'session.created', 'session.revoked', 'resource.listed',
      'resource.viewed', 'resource.created', 'resource.updated',
      'resource.deleted', 'resource.access_denied',
      'authorization.relationship_applied', 'authorization.relationship_failed',
      'authorization.dead_letters_listed', 'authorization.dead_letter_requeued',
      'invitation.created', 'invitation.listed', 'invitation.revoked',
      'invitation.consumed', 'duplicate_email_recovery.declined',
      'user.onboarding_completed', 'activity.timer_started',
      'activity.timer_stopped', 'path_invitation.created',
      'path_invitation.listed', 'path_invitation.accepted',
      'path_invitation.rejected'
    )
  );

COMMIT;
