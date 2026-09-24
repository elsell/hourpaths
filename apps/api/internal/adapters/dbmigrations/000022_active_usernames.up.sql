BEGIN;

ALTER TABLE public.user_models ADD COLUMN username text NULL;

ALTER TABLE public.user_models
  ADD CONSTRAINT user_models_provisional_username_check
  CHECK (status <> 'provisional' OR username IS NULL),
  ADD CONSTRAINT user_models_username_format_check
  CHECK (username IS NULL OR (
      char_length(username) BETWEEN 3 AND 64
      AND username ~ '^[A-Za-z0-9_.]+$'
    ));

CREATE UNIQUE INDEX user_models_normalized_username_unique_idx
ON public.user_models (translate(username, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz'))
WHERE username IS NOT NULL;

COMMIT;
