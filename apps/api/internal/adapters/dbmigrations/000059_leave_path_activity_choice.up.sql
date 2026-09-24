BEGIN;

REVOKE EXECUTE ON FUNCTION public.leave_path_membership(text, text) FROM app;
DROP FUNCTION public.leave_path_membership(text, text);

CREATE FUNCTION public.leave_path_membership(
  path_identifier text,
  member_user_identifier text,
  retain_activity boolean
)
RETURNS TABLE(owner_user_id text, removed_role text, removed_activity_count bigint)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  path_owner text;
  member_role text;
  activity_count bigint := 0;
BEGIN
  SELECT path.owner_user_id, membership.role
  INTO path_owner, member_role
  FROM public.path_models AS path
  JOIN public.path_membership_models AS membership
    ON membership.path_id = path.id
   AND membership.user_id = member_user_identifier
  WHERE path.id = path_identifier
    AND path.archived_at IS NULL
    AND member_user_identifier <> path.owner_user_id
    AND membership.role IN ('administrator', 'participant', 'supporter')
  FOR UPDATE OF path, membership;
  IF path_owner IS NULL OR member_role IS NULL THEN RETURN; END IF;

  IF NOT retain_activity THEN
    SELECT count(*) INTO activity_count
    FROM public.recorded_activity_models
    WHERE path_id = path_identifier AND participant_id = member_user_identifier;

    DELETE FROM public.social_practice_comment_heart_replay_models
    WHERE social_feed_event_id IN (
      SELECT event.id FROM public.social_feed_event_models AS event
      WHERE event.path_id = path_identifier AND event.participant_user_id = member_user_identifier
    );
    DELETE FROM public.social_practice_comment_replay_models
    WHERE social_feed_event_id IN (
      SELECT event.id FROM public.social_feed_event_models AS event
      WHERE event.path_id = path_identifier AND event.participant_user_id = member_user_identifier
    );
    DELETE FROM public.running_timer_models
    WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.activity_mutation_models
    WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.social_goal_achievement_models
    WHERE path_id = path_identifier AND participant_user_id = member_user_identifier;
    DELETE FROM public.recorded_activity_models
    WHERE path_id = path_identifier AND participant_id = member_user_identifier;
  END IF;

  DELETE FROM public.path_membership_models
  WHERE path_id = path_identifier
    AND user_id = member_user_identifier
    AND role = member_role;
  IF NOT FOUND THEN RETURN; END IF;

  RETURN QUERY SELECT path_owner, member_role, activity_count;
END
$$;

REVOKE ALL ON FUNCTION public.leave_path_membership(text, text, boolean) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.leave_path_membership(text, text, boolean) TO app;

COMMIT;
