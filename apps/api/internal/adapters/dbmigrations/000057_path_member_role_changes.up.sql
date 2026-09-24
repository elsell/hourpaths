BEGIN;

ALTER TABLE public.notification_models
  DROP CONSTRAINT notification_models_kind_check,
  DROP CONSTRAINT notification_models_presentation_kind_check,
  DROP CONSTRAINT notification_models_subject_check,
  DROP CONSTRAINT notification_models_offered_role_subject_check,
  ADD CONSTRAINT notification_models_kind_check CHECK (
    kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'path_member_left', 'path_member_removed',
      'path_member_role_changed', 'new_follower', 'follow_request_received',
      'follow_request_accepted', 'practice_reaction', 'practice_comment', 'comment_heart'
    )
  ),
  ADD CONSTRAINT notification_models_presentation_kind_check CHECK (
    (kind IN ('path_invitation_received', 'path_ownership_transfer_received', 'follow_request_received')
      AND presentation_class = 'actionable')
    OR (kind IN (
      'path_invitation_accepted', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled',
      'path_deleted', 'path_member_left', 'path_member_removed',
      'path_member_role_changed', 'new_follower', 'follow_request_accepted',
      'practice_reaction', 'practice_comment', 'comment_heart'
    ) AND presentation_class = 'informational')
  ),
  ADD CONSTRAINT notification_models_subject_check CHECK (
    (kind = 'path_deleted'
      AND path_id IS NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL)
    OR (kind = 'path_member_left'
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IS NULL)
    OR (kind IN ('path_member_removed', 'path_member_role_changed')
      AND path_id IS NOT NULL AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL AND offered_role IN ('participant', 'supporter'))
    OR (kind IN (
      'path_invitation_received', 'path_invitation_accepted',
      'path_ownership_transfer_received', 'path_ownership_transfer_accepted',
      'path_ownership_transfer_declined', 'path_ownership_transfer_canceled'
    ) AND path_id IS NOT NULL AND num_nonnulls(path_invitation_id, path_ownership_transfer_id) = 1
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind = 'new_follower' AND path_id IS NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NOT NULL AND social_feed_event_id IS NULL
      AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind IN ('follow_request_received', 'follow_request_accepted') AND path_id IS NULL
      AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NOT NULL AND follow_subject_user_id IS NOT NULL
      AND social_feed_event_id IS NULL AND reaction_type IS NULL AND comment_id IS NULL)
    OR (kind = 'practice_reaction' AND path_id IS NOT NULL AND path_invitation_id IS NULL
      AND path_ownership_transfer_id IS NULL AND follow_request_id IS NULL
      AND follow_subject_user_id IS NULL AND social_feed_event_id IS NOT NULL
      AND reaction_type IS NOT NULL AND comment_id IS NULL)
    OR (kind IN ('practice_comment', 'comment_heart') AND path_id IS NOT NULL
      AND path_invitation_id IS NULL AND path_ownership_transfer_id IS NULL
      AND follow_request_id IS NULL AND follow_subject_user_id IS NULL
      AND social_feed_event_id IS NOT NULL AND reaction_type IS NULL AND comment_id IS NOT NULL)
  ),
  ADD CONSTRAINT notification_models_offered_role_subject_check CHECK (
    (offered_role IS NOT NULL) = (path_invitation_id IS NOT NULL OR kind IN ('path_member_removed', 'path_member_role_changed'))
  );

CREATE FUNCTION public.change_path_member_role(
  path_identifier text,
  manager_user_identifier text,
  member_user_identifier text,
  expected_member_role text,
  requested_member_role text
)
RETURNS TABLE(owner_user_id text, previous_role text, resulting_role text, removed_activity_count bigint)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  path_owner text;
  target_role text;
  activity_count bigint := 0;
BEGIN
  IF expected_member_role NOT IN ('participant', 'supporter')
    OR requested_member_role NOT IN ('participant', 'supporter')
    OR expected_member_role = requested_member_role
    OR manager_user_identifier = member_user_identifier THEN RETURN; END IF;

  SELECT path.owner_user_id INTO path_owner
  FROM public.path_models AS path
  WHERE path.id = path_identifier AND path.archived_at IS NULL
  FOR UPDATE;
  IF path_owner IS NULL OR member_user_identifier = path_owner OR NOT (
    manager_user_identifier = path_owner OR EXISTS (
      SELECT 1 FROM public.path_membership_models AS manager_membership
      WHERE manager_membership.path_id = path_identifier
        AND manager_membership.user_id = manager_user_identifier
        AND manager_membership.role = 'administrator'
    )
  ) THEN RETURN; END IF;

  SELECT membership.role INTO target_role
  FROM public.path_membership_models AS membership
  WHERE membership.path_id = path_identifier
    AND membership.user_id = member_user_identifier
    AND membership.role = expected_member_role
    AND membership.role IN ('participant', 'supporter')
  FOR UPDATE;
  IF target_role IS NULL THEN RETURN; END IF;

  IF target_role = 'participant' THEN
    SELECT count(*) INTO activity_count FROM public.recorded_activity_models
    WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.running_timer_models WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.activity_mutation_models WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.social_goal_achievement_models WHERE path_id = path_identifier AND participant_user_id = member_user_identifier;
    DELETE FROM public.recorded_activity_models WHERE path_id = path_identifier AND participant_id = member_user_identifier;
  END IF;

  UPDATE public.path_membership_models
  SET role = requested_member_role
  WHERE path_id = path_identifier AND user_id = member_user_identifier AND role = target_role;

  RETURN QUERY SELECT path_owner, target_role, requested_member_role, activity_count;
END
$$;

REVOKE ALL ON FUNCTION public.change_path_member_role(text, text, text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.change_path_member_role(text, text, text, text, text) TO app;

COMMIT;
