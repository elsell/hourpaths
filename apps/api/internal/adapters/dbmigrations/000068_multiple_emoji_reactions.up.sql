BEGIN;
ALTER TABLE public.social_practice_reaction_models
  DROP CONSTRAINT social_practice_reaction_models_pkey,
  DROP CONSTRAINT social_practice_reaction_models_reaction_type_check,
  ADD PRIMARY KEY (social_feed_event_id, actor_user_id, reaction_type),
  ADD CONSTRAINT social_practice_reaction_models_reaction_type_check CHECK (char_length(reaction_type) BETWEEN 1 AND 32);
ALTER TABLE public.social_practice_reaction_replay_models
  DROP CONSTRAINT social_practice_reaction_replay_models_operation_check,
  ADD CONSTRAINT social_practice_reaction_replay_models_operation_check CHECK (operation IN ('social.practice_reaction.set', 'social.practice_reaction.remove', 'social.practice_reaction.emoji_add', 'social.practice_reaction.emoji_remove'));
ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_reaction_type_check,
  ADD CONSTRAINT notification_models_reaction_type_check CHECK (reaction_type IS NULL OR char_length(reaction_type) BETWEEN 1 AND 32);
COMMIT;
