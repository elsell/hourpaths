BEGIN;

CREATE TABLE public.current_policy_set_models (
  singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
  revision bigint NOT NULL CHECK (revision > 0),
  terms_version text NOT NULL CHECK (btrim(terms_version) <> '' AND char_length(terms_version) <= 128),
  privacy_policy_version text NOT NULL CHECK (btrim(privacy_policy_version) <> '' AND char_length(privacy_policy_version) <= 128),
  community_guidelines_version text NOT NULL CHECK (btrim(community_guidelines_version) <> '' AND char_length(community_guidelines_version) <= 128),
  terms_url text NOT NULL CHECK (btrim(terms_url) <> '' AND char_length(terms_url) <= 2048),
  privacy_policy_url text NOT NULL CHECK (btrim(privacy_policy_url) <> '' AND char_length(privacy_policy_url) <= 2048),
  community_guidelines_url text NOT NULL CHECK (btrim(community_guidelines_url) <> '' AND char_length(community_guidelines_url) <= 2048),
  support_url text NOT NULL CHECK (btrim(support_url) <> '' AND char_length(support_url) <= 2048),
  updated_at timestamptz NOT NULL
);

ALTER TABLE public.user_account_activation_models
  ADD COLUMN policy_set_revision bigint NULL
  CONSTRAINT user_account_activation_models_policy_set_revision_check
  CHECK (policy_set_revision IS NULL OR policy_set_revision > 0);

CREATE OR REPLACE FUNCTION public.enforce_complete_account_activation() RETURNS trigger AS $$
BEGIN
  IF OLD.status = 'provisional' AND NEW.status = 'active' THEN
    IF NEW.username IS NULL
      OR btrim(NEW.display_name) = ''
      OR NEW.profile_visibility IS NULL
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_account_activation_models activation
        JOIN public.current_policy_set_models authority
          ON authority.singleton
          AND activation.policy_set_revision = authority.revision
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
        JOIN public.current_policy_set_models authority
          ON authority.singleton
          AND activation.policy_set_revision = authority.revision
        WHERE acceptance.user_id = NEW.id
          AND acceptance.policy = 'terms'
          AND acceptance.version = authority.terms_version
          AND acceptance.acknowledgement = 'accepted'
          AND acceptance.accepted_at <= activation.completed_at
      )
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_policy_acceptance_models acceptance
        JOIN public.user_account_activation_models activation ON activation.user_id = acceptance.user_id
        JOIN public.current_policy_set_models authority
          ON authority.singleton
          AND activation.policy_set_revision = authority.revision
        WHERE acceptance.user_id = NEW.id
          AND acceptance.policy = 'privacy'
          AND acceptance.version = authority.privacy_policy_version
          AND acceptance.acknowledgement = 'acknowledged'
          AND acceptance.accepted_at <= activation.completed_at
      )
      OR NOT EXISTS (
        SELECT 1
        FROM public.user_policy_acceptance_models acceptance
        JOIN public.user_account_activation_models activation ON activation.user_id = acceptance.user_id
        JOIN public.current_policy_set_models authority
          ON authority.singleton
          AND activation.policy_set_revision = authority.revision
        WHERE acceptance.user_id = NEW.id
          AND acceptance.policy = 'community_guidelines'
          AND acceptance.version = authority.community_guidelines_version
          AND acceptance.acknowledgement = 'accepted'
          AND acceptance.accepted_at <= activation.completed_at
      )
    THEN
      RAISE EXCEPTION 'provisional account activation requires a complete onboarding aggregate matching the current policy authority';
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

REVOKE ALL ON public.current_policy_set_models FROM app;
GRANT SELECT ON public.current_policy_set_models TO app;

COMMIT;
