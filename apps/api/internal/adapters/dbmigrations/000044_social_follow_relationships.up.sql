BEGIN;

ALTER TABLE public.follow_models
  ADD COLUMN activity_notifications_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN authorization_change_id text UNIQUE REFERENCES public.authorization_outbox_models(id) ON DELETE RESTRICT;

CREATE TABLE public.follow_request_models (
  id text PRIMARY KEY,
  requester_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  target_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL,
  accepted_at timestamptz,
  rejected_at timestamptz,
  canceled_at timestamptz,
  CHECK (requester_user_id <> target_user_id),
  CHECK (num_nonnulls(accepted_at, rejected_at, canceled_at) <= 1),
  CHECK (accepted_at IS NULL OR accepted_at >= created_at),
  CHECK (rejected_at IS NULL OR rejected_at >= created_at),
  CHECK (canceled_at IS NULL OR canceled_at >= created_at)
);

CREATE UNIQUE INDEX follow_request_models_one_pending_idx
  ON public.follow_request_models(requester_user_id, target_user_id)
  WHERE accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL;
CREATE INDEX follow_request_models_target_pending_idx
  ON public.follow_request_models(target_user_id, created_at DESC, id DESC)
  WHERE accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL;

CREATE TABLE public.social_relationship_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (
    operation IN ('social.follow', 'social.follow_request.cancel', 'social.unfollow', 'social.follow_request.accept', 'social.follow_request.reject')
  ),
  idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND idempotency_key <> ''),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  target_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  follow_request_id text REFERENCES public.follow_request_models(id) ON DELETE CASCADE,
  authorization_change_id text REFERENCES public.authorization_outbox_models(id) ON DELETE RESTRICT,
  changed boolean NOT NULL,
  result_state text NOT NULL CHECK (result_state IN ('none', 'requested', 'following')),
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key),
  CHECK (result_state <> 'requested' OR follow_request_id IS NOT NULL),
  CHECK ((authorization_change_id IS NOT NULL) =
    (changed AND result_state IN ('none', 'following')
      AND operation IN ('social.follow', 'social.unfollow', 'social.follow_request.accept')))
);

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  DROP CONSTRAINT notification_models_channel_check,
  ADD COLUMN follow_request_id text REFERENCES public.follow_request_models(id) ON DELETE CASCADE,
  ADD COLUMN follow_subject_user_id text REFERENCES public.user_models(id) ON DELETE CASCADE,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted',
      'new_follower', 'follow_request_received', 'follow_request_accepted'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received', 'follow_request_received')
      AND presentation_class = 'actionable')
    OR
    (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_accepted'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted'
      AND path_id IS NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NULL)
    OR
    (kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND path_id IS NOT NULL
      AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL)
    OR
    (kind = 'new_follower'
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NOT NULL)
    OR
    (kind IN ('follow_request_received', 'follow_request_accepted')
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK (
    (path_invitation_id IS NULL) = (offered_role IS NULL)
  ),
  ADD CONSTRAINT notification_models_channel_check CHECK (
    channel IN ('path_access', 'following')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following'))
  );

CREATE UNIQUE INDEX notification_models_follow_request_kind_recipient_idx
  ON public.notification_models(follow_request_id, kind, recipient_user_id)
  WHERE follow_request_id IS NOT NULL;

REVOKE ALL ON public.follow_models FROM app;
GRANT SELECT, INSERT, DELETE ON public.follow_models TO app;
REVOKE ALL ON public.follow_request_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.follow_request_models TO app;
REVOKE ALL ON public.social_relationship_replay_models FROM app;
GRANT SELECT, INSERT ON public.social_relationship_replay_models TO app;

COMMIT;
