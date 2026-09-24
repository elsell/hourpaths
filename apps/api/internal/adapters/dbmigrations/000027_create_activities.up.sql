BEGIN;

ALTER TABLE audit_event_models DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK (action IN ('user.provisioned', 'user.deactivated', 'user.profile_synchronized', 'user.viewed', 'session.created', 'session.revoked', 'resource.listed', 'resource.viewed', 'resource.created', 'resource.updated', 'resource.deleted', 'resource.access_denied', 'authorization.relationship_applied', 'authorization.relationship_failed', 'authorization.dead_letters_listed', 'authorization.dead_letter_requeued', 'invitation.created', 'invitation.listed', 'invitation.revoked', 'invitation.consumed', 'duplicate_email_recovery.declined', 'user.onboarding_completed', 'activity.timer_started', 'activity.timer_stopped'));

CREATE TABLE public.running_timer_models (
  id text PRIMARY KEY,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  started_at timestamptz NOT NULL,
  occurrence_time_zone text NOT NULL CHECK (
    occurrence_time_zone = btrim(occurrence_time_zone)
    AND occurrence_time_zone <> ''
    AND occurrence_time_zone <> 'Local'
  ),
  UNIQUE (participant_id, path_id)
);
CREATE INDEX running_timer_models_path_idx
  ON public.running_timer_models(path_id, participant_id);

CREATE TABLE public.recorded_activity_models (
  id text PRIMARY KEY,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  started_at timestamptz NOT NULL,
  ended_at timestamptz NOT NULL,
  occurrence_time_zone text NOT NULL CHECK (
    occurrence_time_zone = btrim(occurrence_time_zone)
    AND occurrence_time_zone <> ''
    AND occurrence_time_zone <> 'Local'
  ),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CHECK (ended_at >= started_at + interval '1 second'),
  CHECK (updated_at >= created_at)
);
CREATE INDEX recorded_activity_models_participant_path_time_idx
  ON public.recorded_activity_models(participant_id, path_id, started_at, id);

CREATE TABLE public.activity_mutation_models (
  participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation IN ('activity.timer.start', 'activity.timer.stop')),
  key text NOT NULL CHECK (key = btrim(key) AND key <> ''),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  timer_id text NOT NULL,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  result_started_at timestamptz,
  result_time_zone text NOT NULL DEFAULT '',
  result_activity_id text,
  result_ended_at timestamptz,
  result_activity_saved boolean NOT NULL DEFAULT false,
  result_created_at timestamptz,
  result_updated_at timestamptz,
  PRIMARY KEY (participant_id, operation, key),
  CHECK (
    (operation = 'activity.timer.start' AND result_started_at IS NOT NULL AND result_time_zone <> '' AND NOT result_activity_saved)
    OR operation = 'activity.timer.stop'
  ),
  CHECK (
    NOT result_activity_saved
    OR (
      result_activity_id IS NOT NULL
      AND result_started_at IS NOT NULL
      AND result_ended_at IS NOT NULL
      AND result_time_zone <> ''
      AND result_created_at IS NOT NULL
      AND result_updated_at IS NOT NULL
    )
  )
);

REVOKE ALL ON public.running_timer_models FROM app;
GRANT SELECT, INSERT, DELETE ON public.running_timer_models TO app;
REVOKE ALL ON public.recorded_activity_models FROM app;
GRANT SELECT, INSERT ON public.recorded_activity_models TO app;
REVOKE ALL ON public.activity_mutation_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.activity_mutation_models TO app;

COMMIT;
