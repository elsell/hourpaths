BEGIN;

CREATE TABLE public.social_practice_reaction_models (
  social_feed_event_id text NOT NULL REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE,
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  reaction_type text NOT NULL CHECK (reaction_type IN ('heart', 'applause', 'fire', 'strong', 'celebrate')),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (social_feed_event_id, actor_user_id),
  CHECK (updated_at >= created_at)
);
CREATE INDEX social_practice_reaction_models_event_count_idx
  ON public.social_practice_reaction_models(social_feed_event_id, reaction_type);

CREATE TABLE public.social_practice_reaction_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation IN ('social.practice_reaction.set', 'social.practice_reaction.remove')),
  idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND idempotency_key <> ''),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  social_feed_event_id text NOT NULL REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key)
);

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  DROP CONSTRAINT notification_models_channel_check,
  ADD COLUMN social_feed_event_id text REFERENCES public.social_feed_event_models(id) ON DELETE CASCADE,
  ADD COLUMN reaction_type text CHECK (reaction_type IN ('heart', 'applause', 'fire', 'strong', 'celebrate')),
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_received', 'follow_request_accepted',
      'practice_reaction'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received', 'follow_request_received')
      AND presentation_class = 'actionable')
    OR
    (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'new_follower', 'follow_request_accepted', 'practice_reaction'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted'
      AND path_id IS NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND path_id IS NOT NULL
      AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind = 'new_follower'
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind IN ('follow_request_received', 'follow_request_accepted')
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL)
    OR
    (kind = 'practice_reaction'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK (
    (path_invitation_id IS NULL) = (offered_role IS NULL)
  ),
  ADD CONSTRAINT notification_models_channel_check CHECK (
    channel IN ('path_access', 'following', 'reactions')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following')
      OR (kind = 'practice_reaction' AND channel = 'reactions'))
  );

CREATE UNIQUE INDEX notification_models_active_practice_reaction_idx
  ON public.notification_models(social_feed_event_id, actor_user_id, recipient_user_id)
  WHERE kind = 'practice_reaction' AND deleted_at IS NULL;

REVOKE ALL ON public.social_practice_reaction_models FROM app;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.social_practice_reaction_models TO app;
REVOKE ALL ON public.social_practice_reaction_replay_models FROM app;
GRANT SELECT, INSERT ON public.social_practice_reaction_replay_models TO app;
GRANT DELETE ON public.notification_models TO app;
REVOKE ALL ON public.social_feed_event_models FROM app;
GRANT SELECT, INSERT ON public.social_feed_event_models TO app;

COMMIT;
