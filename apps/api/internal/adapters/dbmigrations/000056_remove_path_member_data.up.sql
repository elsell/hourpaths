BEGIN;

ALTER TABLE public.path_membership_models
  ADD COLUMN joined_at timestamptz;

-- Reconstruct the current membership incarnation from durable lifecycle
-- evidence. The latest accepted invitation distinguishes leave/rejoin cycles
-- regardless of later continuous role changes. Ownership transfer never
-- advances joined_at: participant->creator and creator->administrator both
-- retain the same membership. Its earliest accepted initiator is used only to
-- recover the original creator lineage, whose membership began at Path
-- creation. Rows with neither invitation nor proven original-creator history
-- are fenced at this migration's single backfill-statement cutover instead of being
-- backdated and accepting queued work from an unknown prior incarnation.
WITH cutover AS (
  SELECT statement_timestamp() AS occurred_at
), current_incarnation AS (
  SELECT
    membership.path_id,
    membership.user_id,
    COALESCE(
      (
        SELECT max(invitation.accepted_at)
        FROM public.path_invitation_models AS invitation
        WHERE invitation.path_id = membership.path_id
          AND invitation.recipient_user_id = membership.user_id
          AND invitation.accepted_at IS NOT NULL
      ),
      CASE WHEN membership.user_id = COALESCE(
        (
          SELECT transfer.initiator_user_id
          FROM public.path_ownership_transfer_models AS transfer
          WHERE transfer.path_id = membership.path_id
            AND transfer.accepted_at IS NOT NULL
          ORDER BY transfer.accepted_at, transfer.created_at, transfer.id
          LIMIT 1
        ),
        path.owner_user_id
      ) THEN path.created_at END,
      cutover.occurred_at
    ) AS joined_at
  FROM public.path_membership_models AS membership
  JOIN public.path_models AS path ON path.id = membership.path_id
  CROSS JOIN cutover
)
UPDATE public.path_membership_models AS membership
SET joined_at = current_incarnation.joined_at
FROM current_incarnation
WHERE current_incarnation.path_id = membership.path_id
  AND current_incarnation.user_id = membership.user_id;
ALTER TABLE public.path_membership_models
  ALTER COLUMN joined_at SET NOT NULL,
  ALTER COLUMN joined_at SET DEFAULT CURRENT_TIMESTAMP;

CREATE FUNCTION public.remove_path_member_data(
  path_identifier text,
  manager_user_identifier text,
  member_user_identifier text,
  expected_member_role text
)
RETURNS TABLE(owner_user_id text, removed_role text, removed_activity_count bigint)
LANGUAGE plpgsql
SECURITY DEFINER
SET search_path = pg_catalog, public
AS $$
DECLARE
  path_owner text;
  target_role text;
  activity_count bigint;
BEGIN
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

  activity_count := 0;
  IF target_role = 'participant' THEN
    SELECT count(*) INTO activity_count FROM public.recorded_activity_models
    WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.running_timer_models WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.activity_mutation_models WHERE path_id = path_identifier AND participant_id = member_user_identifier;
    DELETE FROM public.social_goal_achievement_models WHERE path_id = path_identifier AND participant_user_id = member_user_identifier;
    DELETE FROM public.recorded_activity_models WHERE path_id = path_identifier AND participant_id = member_user_identifier;
  END IF;
  DELETE FROM public.path_membership_models
  WHERE path_id = path_identifier AND user_id = member_user_identifier AND role = target_role;

  RETURN QUERY SELECT path_owner, target_role, activity_count;
END
$$;

REVOKE ALL ON FUNCTION public.remove_path_member_data(text, text, text, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION public.remove_path_member_data(text, text, text, text) TO app;

COMMIT;
