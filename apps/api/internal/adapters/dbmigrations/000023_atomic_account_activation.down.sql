BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.user_account_activation_models)
    OR EXISTS (SELECT 1 FROM public.user_policy_acceptance_models)
    OR EXISTS (SELECT 1 FROM public.user_preference_models)
    OR EXISTS (SELECT 1 FROM public.user_time_zone_history_models)
    OR EXISTS (SELECT 1 FROM public.user_models WHERE profile_visibility IS NOT NULL)
    OR EXISTS (SELECT 1 FROM public.audit_event_models WHERE action = 'user.onboarding_completed')
  THEN
    RAISE EXCEPTION 'cannot remove account activation persistence while activation data exists';
  END IF;
END
$$;

DROP TRIGGER user_account_activation_owner_active ON public.user_account_activation_models;
DROP TRIGGER user_policy_acceptance_owner_active ON public.user_policy_acceptance_models;
DROP TRIGGER user_preference_owner_active ON public.user_preference_models;
DROP TRIGGER user_time_zone_history_owner_active ON public.user_time_zone_history_models;
DROP FUNCTION public.enforce_active_onboarding_owner();
DROP TRIGGER user_models_complete_account_activation ON public.user_models;
DROP FUNCTION public.enforce_complete_account_activation();

REVOKE ALL ON public.user_time_zone_history_models FROM app;
DROP TABLE public.user_time_zone_history_models;
REVOKE ALL ON public.user_preference_models FROM app;
DROP TABLE public.user_preference_models;
REVOKE ALL ON public.user_policy_acceptance_models FROM app;
DROP TABLE public.user_policy_acceptance_models;
REVOKE ALL ON public.user_account_activation_models FROM app;
DROP TABLE public.user_account_activation_models;

ALTER TABLE public.user_models DROP COLUMN profile_visibility;

ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK (action IN ('user.provisioned', 'user.deactivated', 'user.profile_synchronized', 'user.viewed', 'session.created', 'session.revoked', 'resource.listed', 'resource.viewed', 'resource.created', 'resource.updated', 'resource.deleted', 'resource.access_denied', 'authorization.relationship_applied', 'authorization.relationship_failed', 'authorization.dead_letters_listed', 'authorization.dead_letter_requeued', 'invitation.created', 'invitation.listed', 'invitation.revoked', 'invitation.consumed', 'duplicate_email_recovery.declined'));

COMMIT;
