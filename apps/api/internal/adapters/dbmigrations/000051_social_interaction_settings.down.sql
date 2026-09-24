BEGIN;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM public.social_interaction_setting_replay_models)
    OR EXISTS (SELECT 1 FROM public.notification_models WHERE interaction_disabled_reason IS NOT NULL)
    OR EXISTS (
      SELECT 1 FROM public.social_interaction_setting_models
      WHERE NOT comments_enabled OR NOT reactions_enabled
    ) THEN
    RAISE EXCEPTION 'cannot remove social interaction settings while configured data exists';
  END IF;
END $$;

DROP TABLE public.social_interaction_setting_replay_models;
DROP TABLE public.social_interaction_setting_models;
ALTER TABLE public.notification_models DROP COLUMN interaction_disabled_reason;
REVOKE DELETE ON public.notification_push_outbox_models FROM app;
REVOKE DELETE ON public.notification_push_delivery_models FROM app;

COMMIT;
