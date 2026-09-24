BEGIN;

CREATE TABLE public.social_practice_comment_models (
  id text PRIMARY KEY,
  social_feed_event_id text NOT NULL REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE,
  author_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  body text NOT NULL CHECK (body <> '' AND char_length(body) <= 2000),
  version bigint NOT NULL CHECK (version >= 1),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CHECK (updated_at >= created_at)
);
CREATE INDEX social_practice_comment_models_event_page_idx
  ON public.social_practice_comment_models(social_feed_event_id, created_at, id);

CREATE TABLE public.social_practice_comment_revision_models (
  comment_id text NOT NULL REFERENCES public.social_practice_comment_models(id) ON DELETE CASCADE,
  version bigint NOT NULL CHECK (version >= 1),
  body text NOT NULL CHECK (body <> '' AND char_length(body) <= 2000),
  changed_at timestamptz NOT NULL,
  PRIMARY KEY (comment_id, version)
);

CREATE TABLE public.social_practice_comment_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (
    operation IN (
      'social.practice_comment.create',
      'social.practice_comment.edit',
      'social.practice_comment.delete'
    )
  ),
  idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND idempotency_key <> ''),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  comment_id text NOT NULL,
  social_feed_event_id text NOT NULL,
  author_user_id text NOT NULL,
  result_body text NOT NULL,
  result_version bigint NOT NULL CHECK (result_version >= 1),
  result_created_at timestamptz NOT NULL,
  result_updated_at timestamptz NOT NULL,
  result_deleted boolean NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key),
  CHECK (result_updated_at >= result_created_at)
);

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  DROP CONSTRAINT notification_models_channel_check,
  ADD COLUMN comment_id text REFERENCES public.social_practice_comment_models(id) ON DELETE CASCADE,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_received', 'follow_request_accepted',
      'practice_reaction', 'practice_comment'
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
      'practice_reaction', 'practice_comment'
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
    (kind = 'practice_comment'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NULL AND comment_id IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK (
    (path_invitation_id IS NULL) = (offered_role IS NULL)
  ),
  ADD CONSTRAINT notification_models_channel_check CHECK (
    channel IN ('path_access', 'following', 'reactions', 'comments')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following')
      OR (kind = 'practice_reaction' AND channel = 'reactions')
      OR (kind = 'practice_comment' AND channel = 'comments'))
  );

CREATE UNIQUE INDEX notification_models_active_practice_comment_idx
  ON public.notification_models(comment_id, recipient_user_id)
  WHERE kind = 'practice_comment' AND deleted_at IS NULL;

REVOKE ALL ON public.social_practice_comment_models FROM app;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.social_practice_comment_models TO app;
REVOKE ALL ON public.social_practice_comment_revision_models FROM app;
GRANT SELECT, INSERT ON public.social_practice_comment_revision_models TO app;
REVOKE ALL ON public.social_practice_comment_replay_models FROM app;
GRANT SELECT, INSERT ON public.social_practice_comment_replay_models TO app;
REVOKE ALL ON public.social_feed_event_models FROM app;
GRANT SELECT, INSERT ON public.social_feed_event_models TO app;

COMMIT;
