package activitystore

import "gorm.io/gorm"

const protectedPathReadSQL = `EXISTS (
  SELECT 1 FROM path_models protected_path
  WHERE protected_path.id = ? AND (
    protected_path.owner_user_id = ? OR
    EXISTS (SELECT 1 FROM path_membership_models protected_member
      WHERE protected_member.path_id = protected_path.id AND protected_member.user_id = ?) OR (
      NOT EXISTS (SELECT 1 FROM block_models protected_block
        WHERE (protected_block.blocker_user_id = ? AND protected_block.blocked_user_id = protected_path.owner_user_id)
           OR (protected_block.blocker_user_id = protected_path.owner_user_id AND protected_block.blocked_user_id = ?))
      AND (protected_path.visibility = 'public' OR (
        protected_path.visibility = 'followers' AND EXISTS (
          SELECT 1 FROM follow_models protected_follow
          WHERE protected_follow.follower_user_id = ?
            AND protected_follow.following_user_id = protected_path.owner_user_id
        )
      ))
    )
  )
)`

func scopeProtectedPathRead(query *gorm.DB, viewerID, pathID string) *gorm.DB {
	return query.Where(protectedPathReadSQL, pathID, viewerID, viewerID, viewerID, viewerID, viewerID)
}

const currentRecordedActivitySourceSQL = `EXISTS (
  SELECT 1 FROM path_models current_source_path
  WHERE current_source_path.id = recorded_activity_models.path_id
    AND (current_source_path.owner_user_id = recorded_activity_models.participant_id OR EXISTS (
      SELECT 1 FROM path_membership_models current_source_member
      WHERE current_source_member.path_id = recorded_activity_models.path_id
        AND current_source_member.user_id = recorded_activity_models.participant_id
        AND current_source_member.role IN ('administrator', 'participant')
    ))
)`

const currentActivityAliasSourceSQL = `EXISTS (
  SELECT 1 FROM path_models current_source_path
  WHERE current_source_path.id = activity.path_id
    AND (current_source_path.owner_user_id = activity.participant_id OR EXISTS (
      SELECT 1 FROM path_membership_models current_source_member
      WHERE current_source_member.path_id = activity.path_id
        AND current_source_member.user_id = activity.participant_id
        AND current_source_member.role IN ('administrator', 'participant')
    ))
)`

const currentRevisionActivitySourceSQL = `EXISTS (
  SELECT 1 FROM path_models current_source_path
  WHERE current_source_path.id = current.path_id
    AND (current_source_path.owner_user_id = current.participant_id OR EXISTS (
      SELECT 1 FROM path_membership_models current_source_member
      WHERE current_source_member.path_id = current.path_id
        AND current_source_member.user_id = current.participant_id
        AND current_source_member.role IN ('administrator', 'participant')
    ))
)`

func scopeCurrentActivitySource(query *gorm.DB, source string) *gorm.DB {
	switch source {
	case "recorded_activity_models":
		return query.Where(currentRecordedActivitySourceSQL)
	case "activity":
		return query.Where(currentActivityAliasSourceSQL)
	case "current":
		return query.Where(currentRevisionActivitySourceSQL)
	default:
		return query.Where("FALSE")
	}
}

const recordedActivityBlockScopeSQL = `(
  NOT EXISTS (SELECT 1 FROM block_models participant_block
    WHERE (participant_block.blocker_user_id = ? AND participant_block.blocked_user_id = recorded_activity_models.participant_id)
       OR (participant_block.blocker_user_id = recorded_activity_models.participant_id AND participant_block.blocked_user_id = ?))
  OR (EXISTS (SELECT 1 FROM path_membership_models viewer_member
        WHERE viewer_member.path_id = recorded_activity_models.path_id AND viewer_member.user_id = ?)
      AND EXISTS (SELECT 1 FROM path_membership_models source_member
        WHERE source_member.path_id = recorded_activity_models.path_id AND source_member.user_id = recorded_activity_models.participant_id))
)`

const activityAliasBlockScopeSQL = `(
  NOT EXISTS (SELECT 1 FROM block_models participant_block
    WHERE (participant_block.blocker_user_id = ? AND participant_block.blocked_user_id = activity.participant_id)
       OR (participant_block.blocker_user_id = activity.participant_id AND participant_block.blocked_user_id = ?))
  OR (EXISTS (SELECT 1 FROM path_membership_models viewer_member
        WHERE viewer_member.path_id = activity.path_id AND viewer_member.user_id = ?)
      AND EXISTS (SELECT 1 FROM path_membership_models source_member
        WHERE source_member.path_id = activity.path_id AND source_member.user_id = activity.participant_id))
)`

const currentRevisionBlockScopeSQL = `(
  NOT EXISTS (SELECT 1 FROM block_models participant_block
    WHERE (participant_block.blocker_user_id = ? AND participant_block.blocked_user_id = current.participant_id)
       OR (participant_block.blocker_user_id = current.participant_id AND participant_block.blocked_user_id = ?))
  OR (EXISTS (SELECT 1 FROM path_membership_models viewer_member
        WHERE viewer_member.path_id = current.path_id AND viewer_member.user_id = ?)
      AND EXISTS (SELECT 1 FROM path_membership_models source_member
        WHERE source_member.path_id = current.path_id AND source_member.user_id = current.participant_id))
)`

func scopeActivityParticipantBlock(query *gorm.DB, source, viewerID string) *gorm.DB {
	switch source {
	case "recorded_activity_models":
		return query.Where(recordedActivityBlockScopeSQL, viewerID, viewerID, viewerID)
	case "activity":
		return query.Where(activityAliasBlockScopeSQL, viewerID, viewerID, viewerID)
	case "current":
		return query.Where(currentRevisionBlockScopeSQL, viewerID, viewerID, viewerID)
	default:
		return query.Where("FALSE")
	}
}

func scopeReadableActivity(query *gorm.DB, source, viewerID, pathID string) *gorm.DB {
	query = scopeCurrentActivitySource(query, source)
	query = scopeActivityParticipantBlock(query, source, viewerID)
	return scopeProtectedPathRead(query, viewerID, pathID)
}
