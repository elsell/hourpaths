BEGIN;
ALTER TABLE public.social_relationship_replay_models
 DROP CONSTRAINT social_relationship_replay_models_operation_check,
 DROP CONSTRAINT social_relationship_replay_models_check1,
 ADD CONSTRAINT social_relationship_replay_models_operation_check CHECK (
  operation IN ('social.follow','social.follow_request.cancel','social.unfollow','social.follow_request.accept','social.follow_request.reject','social.follower.remove')
 ),
 ADD CONSTRAINT social_relationship_replay_models_check1 CHECK (
  (authorization_change_id IS NOT NULL) =
  (changed AND result_state IN ('none','following') AND operation IN ('social.follow','social.unfollow','social.follow_request.accept','social.follower.remove'))
 );
COMMIT;
