BEGIN;

CREATE TABLE public.social_interaction_setting_models (
  user_id text PRIMARY KEY REFERENCES public.user_models(id) ON DELETE CASCADE,
  comments_enabled boolean NOT NULL DEFAULT true,
  reactions_enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  CHECK (updated_at >= created_at)
);

INSERT INTO public.social_interaction_setting_models (user_id, created_at, updated_at)
SELECT id, created_at, created_at FROM public.user_models;

CREATE TABLE public.social_interaction_setting_replay_models (
  actor_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation = 'social.interaction_settings.update'),
  idempotency_key text NOT NULL CHECK (idempotency_key = btrim(idempotency_key) AND idempotency_key <> ''),
  request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
  comments_enabled boolean NOT NULL,
  reactions_enabled boolean NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (actor_user_id, operation, idempotency_key)
);

ALTER TABLE public.notification_models
  ADD COLUMN interaction_disabled_reason text,
  ADD CONSTRAINT notification_models_interaction_disabled_reason_check CHECK (
    interaction_disabled_reason IS NULL
    OR (
      interaction_disabled_reason IN ('comments', 'reactions')
      AND deleted_at IS NOT NULL
      AND social_feed_event_id IS NOT NULL
      AND kind IN ('practice_reaction', 'practice_comment', 'comment_heart')
      AND (
        (interaction_disabled_reason = 'reactions' AND kind = 'practice_reaction')
        OR (interaction_disabled_reason = 'comments' AND kind IN ('practice_comment', 'comment_heart'))
      )
    )
  );

REVOKE ALL ON public.social_interaction_setting_models FROM app;
GRANT SELECT, INSERT, UPDATE ON public.social_interaction_setting_models TO app;
REVOKE ALL ON public.social_interaction_setting_replay_models FROM app;
GRANT SELECT, INSERT ON public.social_interaction_setting_replay_models TO app;
GRANT DELETE ON public.notification_push_outbox_models TO app;
GRANT DELETE ON public.notification_push_delivery_models TO app;

COMMIT;
