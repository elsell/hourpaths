BEGIN;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM public.social_practice_reaction_models WHERE reaction_type NOT IN ('heart', 'applause', 'fire', 'strong', 'celebrate'))
    OR EXISTS (SELECT 1 FROM public.social_practice_reaction_models GROUP BY social_feed_event_id, actor_user_id HAVING count(*) > 1)
    OR EXISTS (SELECT 1 FROM public.notification_models WHERE reaction_type NOT IN ('heart', 'applause', 'fire', 'strong', 'celebrate'))
    OR EXISTS (SELECT 1 FROM public.social_practice_reaction_replay_models WHERE operation IN ('social.practice_reaction.emoji_add', 'social.practice_reaction.emoji_remove')) THEN
    RAISE EXCEPTION 'cannot downgrade multiple emoji reactions while incompatible data exists';
  END IF;
END $$;
ALTER TABLE public.social_practice_reaction_models
  DROP CONSTRAINT social_practice_reaction_models_pkey,
  DROP CONSTRAINT social_practice_reaction_models_reaction_type_check,
  ADD PRIMARY KEY (social_feed_event_id, actor_user_id),
  ADD CONSTRAINT social_practice_reaction_models_reaction_type_check CHECK (reaction_type IN ('heart', 'applause', 'fire', 'strong', 'celebrate'));
ALTER TABLE public.social_practice_reaction_replay_models
  DROP CONSTRAINT social_practice_reaction_replay_models_operation_check,
  ADD CONSTRAINT social_practice_reaction_replay_models_operation_check CHECK (operation IN ('social.practice_reaction.set', 'social.practice_reaction.remove'));
ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_reaction_type_check,
  ADD CONSTRAINT notification_models_reaction_type_check CHECK (reaction_type IN ('heart', 'applause', 'fire', 'strong', 'celebrate'));
COMMIT;
