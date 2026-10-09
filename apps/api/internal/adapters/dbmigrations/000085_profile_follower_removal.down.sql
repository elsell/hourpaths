BEGIN;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM public.social_relationship_replay_models WHERE operation = 'social.follower.remove') THEN
  RAISE EXCEPTION 'cannot downgrade while follower removal receipts exist';
 END IF;
END $$;
ALTER TABLE public.social_relationship_replay_models
 DROP CONSTRAINT social_relationship_replay_models_operation_check,
 DROP CONSTRAINT social_relationship_replay_models_check1,
 ADD CONSTRAINT social_relationship_replay_models_operation_check CHECK (
  operation IN ('social.follow','social.follow_request.cancel','social.unfollow','social.follow_request.accept','social.follow_request.reject')
 ),
 ADD CONSTRAINT social_relationship_replay_models_check1 CHECK (
  (authorization_change_id IS NOT NULL) =
  (changed AND result_state IN ('none','following') AND operation IN ('social.follow','social.unfollow','social.follow_request.accept'))
 );
COMMIT;
