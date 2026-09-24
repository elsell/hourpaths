BEGIN;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM public.audit_event_models
    WHERE action IN ('activity.timer_started', 'activity.timer_stopped')
  ) OR EXISTS (
    SELECT 1 FROM public.running_timer_models
  ) OR EXISTS (
    SELECT 1 FROM public.recorded_activity_models
  ) OR EXISTS (
    SELECT 1 FROM public.activity_mutation_models
  ) THEN
    RAISE EXCEPTION 'cannot remove activity persistence while timer, activity, mutation, or immutable audit evidence exists';
  END IF;
END
$$;

REVOKE ALL ON public.activity_mutation_models FROM app;
REVOKE ALL ON public.recorded_activity_models FROM app;
REVOKE ALL ON public.running_timer_models FROM app;

DROP TABLE public.activity_mutation_models;
DROP TABLE public.recorded_activity_models;
DROP TABLE public.running_timer_models;

ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK (action IN ('user.provisioned', 'user.deactivated', 'user.profile_synchronized', 'user.viewed', 'session.created', 'session.revoked', 'resource.listed', 'resource.viewed', 'resource.created', 'resource.updated', 'resource.deleted', 'resource.access_denied', 'authorization.relationship_applied', 'authorization.relationship_failed', 'authorization.dead_letters_listed', 'authorization.dead_letter_requeued', 'invitation.created', 'invitation.listed', 'invitation.revoked', 'invitation.consumed', 'duplicate_email_recovery.declined', 'user.onboarding_completed'));

COMMIT;
