BEGIN;

CREATE TABLE public.path_nudge_preference_models (
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  audience text NOT NULL CHECK (audience IN ('nobody', 'path_members', 'followers', 'everyone')),
  revision bigint NOT NULL CHECK (revision > 0),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (path_id, user_id),
  CHECK (updated_at >= created_at)
);

CREATE FUNCTION public.enforce_path_nudge_preference_participant() RETURNS trigger AS $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM public.path_models AS path
    WHERE path.id = NEW.path_id AND (
      path.owner_user_id = NEW.user_id OR EXISTS (
        SELECT 1 FROM public.path_membership_models AS membership
        WHERE membership.path_id = NEW.path_id AND membership.user_id = NEW.user_id
          AND membership.role IN ('administrator', 'participant')
      )
    )
  ) THEN
    RAISE EXCEPTION 'nudge preference owner must be a tracking Path participant';
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER path_nudge_preference_owner_participant
AFTER INSERT OR UPDATE ON public.path_nudge_preference_models
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION public.enforce_path_nudge_preference_participant();
REVOKE ALL ON FUNCTION public.enforce_path_nudge_preference_participant() FROM PUBLIC;

CREATE FUNCTION public.remove_ineligible_path_nudge_preference() RETURNS trigger AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    DELETE FROM public.path_nudge_preference_models AS preference
    WHERE preference.path_id = OLD.path_id AND preference.user_id = OLD.user_id
      AND NOT EXISTS (
        SELECT 1 FROM public.path_models AS path
        WHERE path.id = OLD.path_id AND path.owner_user_id = OLD.user_id
      );
    RETURN OLD;
  END IF;
  IF NEW.role NOT IN ('administrator', 'participant') THEN
    DELETE FROM public.path_nudge_preference_models AS preference
    WHERE preference.path_id = OLD.path_id AND preference.user_id = OLD.user_id
      AND NOT EXISTS (
        SELECT 1 FROM public.path_models AS path
        WHERE path.id = OLD.path_id AND path.owner_user_id = OLD.user_id
      );
  END IF;
  RETURN NEW;
END
$$ LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public;

CREATE TRIGGER path_membership_remove_ineligible_nudge_preference
AFTER DELETE OR UPDATE OF role ON public.path_membership_models
FOR EACH ROW EXECUTE FUNCTION public.remove_ineligible_path_nudge_preference();
REVOKE ALL ON FUNCTION public.remove_ineligible_path_nudge_preference() FROM PUBLIC;

CREATE TABLE public.path_nudge_preference_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation = 'social.path_nudge_preference.update'),
  idempotency_key text NOT NULL CHECK (
    idempotency_key = btrim(idempotency_key)
    AND char_length(idempotency_key) BETWEEN 16 AND 128
  ),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  result_audience text NOT NULL CHECK (result_audience IN ('nobody', 'path_members', 'followers', 'everyone')),
  result_revision bigint NOT NULL CHECK (result_revision > 0),
  result_updated_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key)
);

CREATE TABLE public.notification_channel_preference_models (
  user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  channel text NOT NULL CHECK (channel IN (
    'following', 'path_access', 'tracking_activity', 'achievements',
    'comments', 'reactions', 'comment_hearts', 'nudges',
    'goal_reminders', 'timer_health'
  )),
  enabled boolean NOT NULL,
  revision bigint NOT NULL CHECK (revision > 0),
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  PRIMARY KEY (user_id, channel),
  CHECK (updated_at >= created_at)
);

CREATE TABLE public.notification_channel_preference_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation = 'notification.channel_preference.update'),
  idempotency_key text NOT NULL CHECK (
    idempotency_key = btrim(idempotency_key)
    AND char_length(idempotency_key) BETWEEN 16 AND 128
  ),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  channel text NOT NULL CHECK (channel IN (
    'following', 'path_access', 'tracking_activity', 'achievements',
    'comments', 'reactions', 'comment_hearts', 'nudges',
    'goal_reminders', 'timer_health'
  )),
  result_enabled boolean NOT NULL,
  result_revision bigint NOT NULL CHECK (result_revision > 0),
  result_updated_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key)
);

CREATE TABLE public.social_nudge_models (
  id text PRIMARY KEY,
  sender_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  recipient_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  content_kind text NOT NULL CHECK (content_kind = 'preset'),
  preset text NOT NULL CHECK (preset IN (
    'you_have_got_this', 'lets_go', 'little_progress_counts',
    'keep_it_going', 'time_to_work'
  )),
  sent_at timestamptz NOT NULL,
  interval_started_at timestamptz,
  interval_ended_at timestamptz,
  CHECK (sender_user_id <> recipient_user_id),
  CHECK ((interval_started_at IS NULL) = (interval_ended_at IS NULL)),
  CHECK (interval_ended_at IS NULL OR interval_ended_at > interval_started_at)
);
CREATE UNIQUE INDEX social_nudge_models_interval_limit_idx
  ON public.social_nudge_models(sender_user_id, recipient_user_id, path_id, interval_started_at)
  WHERE interval_started_at IS NOT NULL;
CREATE INDEX social_nudge_models_rolling_limit_idx
  ON public.social_nudge_models(sender_user_id, recipient_user_id, path_id, sent_at DESC)
  WHERE interval_started_at IS NULL;

CREATE TABLE public.social_nudge_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation = 'social.nudge.send'),
  idempotency_key text NOT NULL CHECK (
    idempotency_key = btrim(idempotency_key)
    AND char_length(idempotency_key) BETWEEN 16 AND 128
  ),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  nudge_id text NOT NULL REFERENCES public.social_nudge_models(id) ON DELETE CASCADE,
  recipient_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
  preset text NOT NULL CHECK (preset IN (
    'you_have_got_this', 'lets_go', 'little_progress_counts',
    'keep_it_going', 'time_to_work'
  )),
  result_notification_created boolean NOT NULL,
  result_sent_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key)
);

ALTER TABLE public.notification_models
  ADD COLUMN nudge_id text REFERENCES public.social_nudge_models(id) ON DELETE CASCADE,
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_channel_check,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'path_member_left', 'path_member_removed',
      'path_member_role_changed', 'new_follower',
      'follow_request_received', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart', 'nudge_received'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received', 'follow_request_received')
      AND presentation_class = 'actionable')
    OR (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'path_member_left', 'path_member_removed',
      'path_member_role_changed', 'new_follower', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart', 'nudge_received'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted'
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL AND nudge_id IS NULL)
    OR (kind = 'path_member_left'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL AND nudge_id IS NULL)
    OR (kind = 'path_member_removed'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IN ('participant', 'supporter')
      AND nudge_id IS NULL)
    OR (kind = 'path_member_role_changed'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IN ('participant', 'supporter', 'administrator')
      AND nudge_id IS NULL)
    OR (kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND nudge_id IS NULL)
    OR (kind = 'new_follower' AND path_id IS NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NOT NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL AND nudge_id IS NULL)
    OR (kind IN ('follow_request_received', 'follow_request_accepted') AND path_id IS NULL
      AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL AND nudge_id IS NULL)
    OR (kind = 'practice_reaction' AND path_id IS NOT NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NULL AND social_feed_event_id IS NOT NULL
      AND reaction_type IS NOT NULL AND comment_id IS NULL AND offered_role IS NULL AND nudge_id IS NULL)
    OR (kind IN ('practice_comment', 'comment_heart') AND path_id IS NOT NULL
      AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NULL AND comment_id IS NOT NULL
      AND offered_role IS NULL AND nudge_id IS NULL)
    OR (kind = 'nudge_received' AND path_id IS NOT NULL
      AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL
      AND offered_role IS NULL AND nudge_id IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_channel_check CHECK (
    channel IN ('path_access', 'following', 'reactions', 'comments', 'comment_hearts', 'nudges')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following')
      OR (kind = 'practice_reaction' AND channel = 'reactions')
      OR (kind = 'practice_comment' AND channel = 'comments')
      OR (kind = 'comment_heart' AND channel = 'comment_hearts')
      OR (kind = 'nudge_received' AND channel = 'nudges'))
  );
CREATE UNIQUE INDEX notification_models_nudge_recipient_idx
  ON public.notification_models(nudge_id, recipient_user_id)
  WHERE kind = 'nudge_received';

REVOKE ALL ON public.path_nudge_preference_models FROM app;
GRANT SELECT, INSERT, UPDATE (audience, revision, updated_at)
  ON public.path_nudge_preference_models TO app;
REVOKE ALL ON public.path_nudge_preference_replay_models FROM app;
GRANT SELECT, INSERT ON public.path_nudge_preference_replay_models TO app;
REVOKE ALL ON public.notification_channel_preference_models FROM app;
GRANT SELECT, INSERT, UPDATE (enabled, revision, updated_at)
  ON public.notification_channel_preference_models TO app;
REVOKE ALL ON public.notification_channel_preference_replay_models FROM app;
GRANT SELECT, INSERT ON public.notification_channel_preference_replay_models TO app;
REVOKE ALL ON public.social_nudge_models FROM app;
GRANT SELECT, INSERT ON public.social_nudge_models TO app;
REVOKE ALL ON public.social_nudge_replay_models FROM app;
GRANT SELECT, INSERT ON public.social_nudge_replay_models TO app;

COMMIT;
