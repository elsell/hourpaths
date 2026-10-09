BEGIN;
ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_channel_check,
  ADD CONSTRAINT notification_models_kind_check CHECK (kind IN (
    'path_invitation_received','path_invitation_accepted','path_ownership_transfer_received','path_ownership_transfer_accepted',
    'path_ownership_transfer_declined','path_ownership_transfer_canceled','path_deleted','path_member_left','path_member_removed',
    'path_member_role_changed','path_visibility_changed','new_follower','follow_request_received','follow_request_accepted',
    'practice_reaction','practice_comment','comment_heart','nudge_received','timer_started','long_timer_running','goal_no_longer_achievable','goal_practice_reminder','interval_goal_achieved','overall_target_achieved'
  )),
  ADD CONSTRAINT notification_models_subject_check CHECK ((timer_id IS NULL AND (
    (kind='goal_practice_reminder' AND recipient_user_id=actor_user_id AND num_nonnulls(path_id,path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0) OR
    (kind='path_deleted' AND num_nonnulls(path_id,path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)
    OR (kind='goal_no_longer_achievable' AND recipient_user_id=actor_user_id AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)
    OR (kind='path_member_left' AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)
    OR (kind='path_member_removed' AND path_id IS NOT NULL AND offered_role IN ('participant','supporter') AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,nudge_id,path_visibility)=0)
    OR (kind='path_member_role_changed' AND path_id IS NOT NULL AND offered_role IN ('participant','supporter','administrator') AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,nudge_id,path_visibility)=0)
    OR (kind='path_visibility_changed' AND path_id IS NOT NULL AND path_visibility IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id)=0)
    OR (kind IN ('path_invitation_received','path_invitation_accepted','path_ownership_transfer_received','path_ownership_transfer_accepted','path_ownership_transfer_declined','path_ownership_transfer_canceled') AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id)=1 AND num_nonnulls(follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,nudge_id,path_visibility)=0)
    OR (kind='new_follower' AND follow_subject_user_id IS NOT NULL AND num_nonnulls(path_id,path_invitation_id,path_ownership_transfer_id,follow_request_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)
    OR (kind IN ('follow_request_received','follow_request_accepted') AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL AND num_nonnulls(path_id,path_invitation_id,path_ownership_transfer_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)
    OR (kind='practice_reaction' AND path_id IS NOT NULL AND social_feed_event_id IS NOT NULL AND reaction_type IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,comment_id,offered_role,nudge_id,path_visibility)=0)
    OR (kind IN ('practice_comment','comment_heart') AND path_id IS NOT NULL AND social_feed_event_id IS NOT NULL AND comment_id IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,reaction_type,offered_role,nudge_id,path_visibility)=0)
    OR (kind='nudge_received' AND path_id IS NOT NULL AND nudge_id IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,path_visibility)=0)
    OR (kind IN ('interval_goal_achieved','overall_target_achieved') AND path_id IS NOT NULL AND social_feed_event_id IS NOT NULL AND recipient_user_id = actor_user_id AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)
  )) OR (kind IN ('timer_started','long_timer_running') AND (kind <> 'long_timer_running' OR recipient_user_id = actor_user_id) AND timer_id IS NOT NULL AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)),
  ADD CONSTRAINT notification_models_channel_check CHECK ((
    channel IN ('path_access', 'following', 'reactions', 'comments', 'comment_hearts', 'nudges')
    AND ((kind LIKE 'path_%' AND channel = 'path_access')
      OR (kind IN ('new_follower', 'follow_request_received', 'follow_request_accepted') AND channel = 'following')
      OR (kind = 'practice_reaction' AND channel = 'reactions')
      OR (kind = 'practice_comment' AND channel = 'comments')
      OR (kind = 'comment_heart' AND channel = 'comment_hearts')
      OR (kind = 'nudge_received' AND channel = 'nudges'))
  ) OR (kind = 'timer_started' AND channel = 'tracking_activity')
    OR (kind = 'long_timer_running' AND channel = 'timer_health')
    OR (kind IN ('goal_no_longer_achievable','goal_practice_reminder') AND channel = 'goal_reminders')
    OR (kind IN ('interval_goal_achieved','overall_target_achieved') AND channel = 'achievements'));
CREATE UNIQUE INDEX notification_models_id_recipient_idx ON public.notification_models(id,recipient_user_id);
CREATE TABLE public.goal_reminder_receipt_models (
 participant_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
 path_id text NOT NULL REFERENCES public.path_models(id) ON DELETE CASCADE,
 interval_started_at timestamptz NOT NULL,
 interval_ended_at timestamptz NOT NULL CHECK (interval_ended_at>interval_started_at),
 notification_id text,
 scheduled_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL,
 PRIMARY KEY(participant_id,path_id,interval_started_at,interval_ended_at),
 FOREIGN KEY(notification_id,participant_id) REFERENCES public.notification_models(id,recipient_user_id) ON DELETE SET NULL (notification_id)
);
CREATE INDEX goal_reminder_receipt_notification_idx ON public.goal_reminder_receipt_models(notification_id,participant_id);
REVOKE ALL ON public.goal_reminder_receipt_models FROM app;
GRANT SELECT,INSERT ON public.goal_reminder_receipt_models TO app;
COMMIT;
