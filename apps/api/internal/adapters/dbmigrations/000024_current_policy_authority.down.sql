BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.current_policy_set_models)
    OR EXISTS (SELECT 1 FROM public.user_account_activation_models WHERE policy_set_revision IS NOT NULL)
  THEN
    RAISE EXCEPTION 'cannot remove current policy authority while policy authority or revision evidence exists';
  END IF;
END
$$;

CREATE OR REPLACE FUNCTION public.enforce_complete_account_activation() RETURNS trigger AS $$
BEGIN
  IF OLD.status = 'provisional' AND NEW.status = 'active' THEN
    IF NEW.username IS NULL
      OR btrim(NEW.display_name) = ''
      OR NEW.profile_visibility IS NULL
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_account_activation_models activation
        WHERE activation.user_id = NEW.id
      )
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_preference_models preference
        WHERE preference.user_id = NEW.id
      )
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_account_activation_models activation
        JOIN public.user_preference_models preference ON preference.user_id = activation.user_id
        JOIN public.user_time_zone_history_models history
          ON history.user_id = activation.user_id
          AND history.effective_at = activation.completed_at
          AND history.time_zone = preference.current_time_zone
        WHERE activation.user_id = NEW.id
          AND preference.created_at = activation.completed_at
          AND preference.updated_at = activation.completed_at
      )
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_policy_acceptance_models acceptance
        JOIN public.user_account_activation_models activation ON activation.user_id = acceptance.user_id
        WHERE acceptance.user_id = NEW.id
          AND acceptance.policy = 'terms'
          AND acceptance.acknowledgement = 'accepted'
          AND acceptance.accepted_at <= activation.completed_at
      )
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_policy_acceptance_models acceptance
        JOIN public.user_account_activation_models activation ON activation.user_id = acceptance.user_id
        WHERE acceptance.user_id = NEW.id
          AND acceptance.policy = 'privacy'
          AND acceptance.acknowledgement = 'acknowledged'
          AND acceptance.accepted_at <= activation.completed_at
      )
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_policy_acceptance_models acceptance
        JOIN public.user_account_activation_models activation ON activation.user_id = acceptance.user_id
        WHERE acceptance.user_id = NEW.id
          AND acceptance.policy = 'community_guidelines'
          AND acceptance.acknowledgement = 'accepted'
          AND acceptance.accepted_at <= activation.completed_at
      )
    THEN
      RAISE EXCEPTION 'provisional account activation requires a complete onboarding aggregate';
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

ALTER TABLE public.user_account_activation_models DROP COLUMN policy_set_revision;

REVOKE ALL ON public.current_policy_set_models FROM app;
DROP TABLE public.current_policy_set_models;

COMMIT;
