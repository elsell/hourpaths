BEGIN;

ALTER TABLE public.user_models
  ADD COLUMN description text,
  ADD COLUMN profile_picture_url text,
  ADD CONSTRAINT user_models_description_check CHECK (
    description IS NULL OR (description = btrim(description) AND description <> '' AND char_length(description) <= 500)
  ),
  ADD CONSTRAINT user_models_profile_picture_url_check CHECK (
    profile_picture_url IS NULL OR (profile_picture_url = btrim(profile_picture_url) AND profile_picture_url <> '')
  );

CREATE TABLE public.follow_models (
  follower_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  following_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (follower_user_id, following_user_id),
  CHECK (follower_user_id <> following_user_id)
);
CREATE INDEX follow_models_following_idx ON public.follow_models(following_user_id, follower_user_id);

CREATE TABLE public.block_models (
  blocker_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  blocked_user_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (blocker_user_id, blocked_user_id),
  CHECK (blocker_user_id <> blocked_user_id)
);
CREATE INDEX block_models_blocked_idx ON public.block_models(blocked_user_id, blocker_user_id);

REVOKE ALL ON public.follow_models FROM app;
GRANT SELECT ON public.follow_models TO app;
REVOKE ALL ON public.block_models FROM app;
GRANT SELECT ON public.block_models TO app;

COMMIT;
