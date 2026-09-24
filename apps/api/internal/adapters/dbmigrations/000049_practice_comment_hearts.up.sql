BEGIN;

CREATE TABLE public.social_practice_comment_heart_models (
  comment_id text NOT NULL REFERENCES public.social_practice_comment_models(id) ON DELETE CASCADE,
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (comment_id, actor_user_id)
);
CREATE INDEX social_practice_comment_heart_models_roster_idx
  ON public.social_practice_comment_heart_models(comment_id, created_at, actor_user_id);

CREATE TABLE public.social_practice_comment_heart_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation IN ('social.practice_comment_heart.set', 'social.practice_comment_heart.remove')),
  idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND idempotency_key <> ''),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  social_feed_event_id text NOT NULL,
  comment_id text NOT NULL,
  result_hearted boolean NOT NULL,
  result_heart_count bigint NOT NULL CHECK (result_heart_count >= 0),
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key),
  CHECK (NOT result_hearted OR result_heart_count > 0)
);

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  DROP CONSTRAINT notification_models_channel_check,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_received', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received', 'follow_request_received')
      AND presentation_class = 'actionable')
    OR
    (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted'
      AND path_id IS NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL)
    OR
    (kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND path_id IS NOT NULL
      AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR
    (kind = 'new_follower'
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR
    (kind IN ('follow_request_received', 'follow_request_accepted')
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR
    (kind = 'practice_reaction'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NOT NULL AND comment_id IS NULL)
    OR
    (kind IN ('practice_comment', 'comment_heart')
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NULL AND comment_id IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK ((path_invitation_id IS NULL) = (offered_role IS NULL)),
  ADD CONSTRAINT notification_models_channel_check CHECK (
    channel IN ('path_access', 'following', 'reactions', 'comments', 'comment_hearts')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following')
      OR (kind = 'practice_reaction' AND channel = 'reactions')
      OR (kind = 'practice_comment' AND channel = 'comments')
      OR (kind = 'comment_heart' AND channel = 'comment_hearts'))
  );

CREATE UNIQUE INDEX notification_models_active_comment_heart_idx
  ON public.notification_models(comment_id, actor_user_id, recipient_user_id)
  WHERE kind = 'comment_heart' AND deleted_at IS NULL;

REVOKE ALL ON public.social_practice_comment_heart_models FROM app;
GRANT SELECT, INSERT, DELETE ON public.social_practice_comment_heart_models TO app;
REVOKE ALL ON public.social_practice_comment_heart_replay_models FROM app;
GRANT SELECT, INSERT ON public.social_practice_comment_heart_replay_models TO app;

COMMIT;
