BEGIN;

CREATE TABLE public.path_visibility_replay_models (
  principal_id text NOT NULL REFERENCES public.user_models(id) ON DELETE CASCADE,
  operation text NOT NULL CHECK (operation='path.visibility.set'),
  key text NOT NULL,
  result jsonb NOT NULL,
  created_at timestamptz NOT NULL,
  PRIMARY KEY (principal_id,operation,key)
);
GRANT SELECT, INSERT ON public.path_visibility_replay_models TO app;

ALTER TABLE public.audit_event_models DROP CONSTRAINT audit_event_models_action_check;
ALTER TABLE public.audit_event_models ADD CONSTRAINT audit_event_models_action_check CHECK (action IN (
  'user.provisioned','user.deactivated','user.profile_synchronized','user.viewed','session.created','session.revoked',
  'resource.listed','resource.viewed','resource.created','resource.updated','resource.deleted','resource.access_denied',
  'authorization.relationship_applied','authorization.relationship_failed','authorization.dead_letters_listed','authorization.dead_letter_requeued',
  'invitation.created','invitation.listed','invitation.revoked','invitation.consumed','duplicate_email_recovery.declined',
  'user.onboarding_completed','activity.timer_started','activity.timer_stopped','path_invitation.created','path_invitation.listed',
  'path_invitation.accepted','path_invitation.rejected','path_invitation.canceled','path.visibility_changed',
  'path_ownership_transfer.created','path_ownership_transfer.listed','path_ownership_transfer.accepted','path_ownership_transfer.declined','path_ownership_transfer.canceled'
));

ALTER TABLE public.notification_models
  ADD COLUMN path_visibility text CHECK (path_visibility IN ('private','followers','public')),
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  ADD CONSTRAINT notification_models_kind_check CHECK (kind IN (
    'path_invitation_received','path_invitation_accepted','path_ownership_transfer_received','path_ownership_transfer_accepted',
    'path_ownership_transfer_declined','path_ownership_transfer_canceled','path_deleted','path_member_left','path_member_removed',
    'path_member_role_changed','path_visibility_changed','new_follower','follow_request_received','follow_request_accepted',
    'practice_reaction','practice_comment','comment_heart','nudge_received'
  )),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received','path_ownership_transfer_received','follow_request_received') AND presentation_class='actionable')
    OR (kind NOT IN ('path_invitation_received','path_ownership_transfer_received','follow_request_received') AND presentation_class='informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind='path_deleted' AND num_nonnulls(path_id,path_invitation_id,path_ownership_transfer_id,follow_request_id,follow_subject_user_id,social_feed_event_id,reaction_type,comment_id,offered_role,nudge_id,path_visibility)=0)
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
  );

COMMIT;
