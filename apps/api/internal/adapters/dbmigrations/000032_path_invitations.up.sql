BEGIN;

ALTER TABLE public.audit_event_models
  DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE public.audit_event_models
  ADD CONSTRAINT audit_event_models_action_check CHECK (
    action IN (
      'user.provisioned',
      'user.deactivated',
      'user.profile_synchronized',
      'user.viewed',
      'session.created',
      'session.revoked',
      'resource.listed',
      'resource.viewed',
      'resource.created',
      'resource.updated',
      'resource.deleted',
      'resource.access_denied',
      'authorization.relationship_applied',
      'authorization.relationship_failed',
      'authorization.dead_letters_listed',
      'authorization.dead_letter_requeued',
      'invitation.created',
      'invitation.listed',
      'invitation.revoked',
      'invitation.consumed',
      'duplicate_email_recovery.declined',
      'user.onboarding_completed',
      'activity.timer_started',
      'activity.timer_stopped',
      'path_invitation.created',
      'path_invitation.listed',
      'path_invitation.accepted'
    )
  );

CREATE TABLE public.path_invitation_models (
  id text PRIMARY KEY,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  inviter_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  recipient_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  offered_role text NOT NULL CHECK (
    offered_role IN ('participant', 'supporter')
  ),
  authorization_change_id text UNIQUE REFERENCES public.authorization_outbox_models(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL,
  accepted_at timestamptz,
  rejected_at timestamptz,
  canceled_at timestamptz,
  CHECK (inviter_user_id <> recipient_user_id),
  CHECK (num_nonnulls(accepted_at, rejected_at, canceled_at) <= 1),
  CHECK ((accepted_at IS NULL) = (authorization_change_id IS NULL)),
  CHECK (accepted_at IS NULL OR accepted_at >= created_at),
  CHECK (rejected_at IS NULL OR rejected_at >= created_at),
  CHECK (canceled_at IS NULL OR canceled_at >= created_at)
);

CREATE UNIQUE INDEX path_invitation_models_one_pending_idx
  ON public.path_invitation_models(path_id, recipient_user_id)
  WHERE accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL;
CREATE INDEX path_invitation_models_recipient_pending_idx
  ON public.path_invitation_models(recipient_user_id, created_at DESC, id)
  WHERE accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL;

CREATE TABLE public.notification_models (
  id text PRIMARY KEY,
  recipient_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  path_invitation_id text NOT NULL REFERENCES public.path_invitation_models(id) ON DELETE CASCADE,
  kind text NOT NULL CHECK (
    kind IN ('path_invitation_received', 'path_invitation_accepted')
  ),
  presentation_class text NOT NULL CHECK (
    presentation_class IN ('actionable', 'informational')
  ),
  channel text NOT NULL CHECK (channel = 'path_access'),
  offered_role text NOT NULL CHECK (
    offered_role IN ('participant', 'supporter')
  ),
  created_at timestamptz NOT NULL,
  read_at timestamptz,
  deleted_at timestamptz,
  CHECK (read_at IS NULL OR read_at >= created_at),
  CHECK (deleted_at IS NULL OR deleted_at >= created_at),
  CHECK (
    (kind = 'path_invitation_received' AND presentation_class = 'actionable')
    OR
    (kind = 'path_invitation_accepted' AND presentation_class = 'informational')
  )
);

CREATE UNIQUE INDEX notification_models_invitation_kind_recipient_idx
  ON public.notification_models(path_invitation_id, kind, recipient_user_id);
CREATE INDEX notification_models_recipient_section_idx
  ON public.notification_models(recipient_user_id, presentation_class, created_at DESC, id)
  WHERE deleted_at IS NULL;

CREATE TABLE public.notification_push_outbox_models (
  notification_id text PRIMARY KEY REFERENCES public.notification_models(id) ON DELETE CASCADE,
  attempts integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
  locked_by text,
  locked_until timestamptz,
  delivered_at timestamptz,
  suppressed_at timestamptz,
  permanently_failed_at timestamptz,
  failure_code text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  CHECK (num_nonnulls(delivered_at, suppressed_at, permanently_failed_at) <= 1),
  CHECK (delivered_at IS NULL OR delivered_at >= created_at),
  CHECK (suppressed_at IS NULL OR suppressed_at >= created_at),
  CHECK (permanently_failed_at IS NULL OR permanently_failed_at >= created_at),
  CHECK ((locked_by IS NULL) = (locked_until IS NULL))
);

CREATE INDEX notification_push_outbox_models_claim_idx
  ON public.notification_push_outbox_models(created_at, notification_id)
  WHERE delivered_at IS NULL
    AND suppressed_at IS NULL
    AND permanently_failed_at IS NULL;

REVOKE ALL ON public.path_invitation_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.path_invitation_models TO app;
REVOKE ALL ON public.notification_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.notification_models TO app;
REVOKE ALL ON public.notification_push_outbox_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.notification_push_outbox_models TO app;

COMMIT;
