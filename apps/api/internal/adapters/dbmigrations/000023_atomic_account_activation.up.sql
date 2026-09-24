BEGIN;

ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK (action IN ('user.provisioned', 'user.deactivated', 'user.profile_synchronized', 'user.viewed', 'session.created', 'session.revoked', 'resource.listed', 'resource.viewed', 'resource.created', 'resource.updated', 'resource.deleted', 'resource.access_denied', 'authorization.relationship_applied', 'authorization.relationship_failed', 'authorization.dead_letters_listed', 'authorization.dead_letter_requeued', 'invitation.created', 'invitation.listed', 'invitation.revoked', 'invitation.consumed', 'duplicate_email_recovery.declined', 'user.onboarding_completed'));

ALTER TABLE public.user_models ADD COLUMN profile_visibility text NULL
  CONSTRAINT user_models_profile_visibility_check
  CHECK (profile_visibility IS NULL OR profile_visibility IN ('public', 'private'));

CREATE TABLE public.user_account_activation_models (
  user_id text PRIMARY KEY REFERENCES public.user_models(id) ON DELETE CASCADE,
  minimum_age_attested smallint NOT NULL CHECK (minimum_age_attested = 16),
  age_attested_at timestamptz NOT NULL,
  completed_at timestamptz NOT NULL,
  CHECK (age_attested_at <= completed_at)
);

CREATE TABLE public.user_policy_acceptance_models (
  user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  policy text NOT NULL CHECK (policy IN ('terms', 'privacy', 'community_guidelines')),
  version text NOT NULL CHECK (btrim(version) <> '' AND char_length(version) <= 128),
  acknowledgement text NOT NULL CHECK (acknowledgement IN ('accepted', 'acknowledged')),
  accepted_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, policy, version),
  CHECK (
    (policy IN ('terms', 'community_guidelines') AND acknowledgement = 'accepted')
    OR (policy = 'privacy' AND acknowledgement = 'acknowledged')
  )
);

CREATE TABLE public.user_preference_models (
  user_id text PRIMARY KEY REFERENCES public.user_models(id) ON DELETE CASCADE,
  first_day_of_week smallint NOT NULL CHECK (first_day_of_week BETWEEN 1 AND 7),
  current_time_zone text NOT NULL CHECK (btrim(current_time_zone) <> '' AND char_length(current_time_zone) <= 255),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CHECK (created_at <= updated_at)
);

CREATE TABLE public.user_time_zone_history_models (
  user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  effective_at timestamptz NOT NULL,
  time_zone text NOT NULL CHECK (btrim(time_zone) <> '' AND char_length(time_zone) <= 255),
  PRIMARY KEY (user_id, effective_at)
);

CREATE FUNCTION public.enforce_complete_account_activation() RETURNS trigger AS $$
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

CREATE TRIGGER user_models_complete_account_activation
BEFORE UPDATE OF status ON public.user_models
FOR EACH ROW EXECUTE FUNCTION public.enforce_complete_account_activation();

CREATE FUNCTION public.enforce_active_onboarding_owner() RETURNS trigger AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM public.user_models
    WHERE id = NEW.user_id AND status = 'active'
  ) THEN
    RAISE EXCEPTION 'account activation owner must be active at transaction commit';
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER user_account_activation_owner_active
AFTER INSERT OR UPDATE ON public.user_account_activation_models
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.enforce_active_onboarding_owner();

CREATE CONSTRAINT TRIGGER user_policy_acceptance_owner_active
AFTER INSERT OR UPDATE ON public.user_policy_acceptance_models
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.enforce_active_onboarding_owner();

CREATE CONSTRAINT TRIGGER user_preference_owner_active
AFTER INSERT OR UPDATE ON public.user_preference_models
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.enforce_active_onboarding_owner();

CREATE CONSTRAINT TRIGGER user_time_zone_history_owner_active
AFTER INSERT OR UPDATE ON public.user_time_zone_history_models
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.enforce_active_onboarding_owner();

REVOKE ALL ON FUNCTION public.enforce_complete_account_activation() FROM PUBLIC;
REVOKE ALL ON FUNCTION public.enforce_active_onboarding_owner() FROM PUBLIC;

REVOKE ALL ON public.user_account_activation_models FROM app;
GRANT SELECT, INSERT ON public.user_account_activation_models TO app;
REVOKE ALL ON public.user_policy_acceptance_models FROM app;
GRANT SELECT, INSERT ON public.user_policy_acceptance_models TO app;
REVOKE ALL ON public.user_time_zone_history_models FROM app;
GRANT SELECT, INSERT ON public.user_time_zone_history_models TO app;
REVOKE ALL ON public.user_preference_models FROM app;
GRANT SELECT, INSERT ON public.user_preference_models TO app;

COMMIT;
