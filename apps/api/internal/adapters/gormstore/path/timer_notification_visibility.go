package pathstore

// History remains after subscription changes, but never grants access to a Path
// or another person's activity. Read permission is also checked by the service.
const timerNotificationVisiblePredicate = `(notification_models.kind <> 'timer_started' OR (
  path_models.id IS NOT NULL
  AND EXISTS (SELECT 1 FROM user_models recipient WHERE recipient.id = notification_models.recipient_user_id AND recipient.status = 'active')
  AND EXISTS (SELECT 1 FROM user_models starter WHERE starter.id = notification_models.actor_user_id AND starter.status = 'active')
  AND NOT EXISTS (SELECT 1 FROM block_models owner_block
    WHERE (owner_block.blocker_user_id = notification_models.recipient_user_id AND owner_block.blocked_user_id = path_models.owner_user_id)
      OR (owner_block.blocker_user_id = path_models.owner_user_id AND owner_block.blocked_user_id = notification_models.recipient_user_id))
  AND (EXISTS (SELECT 1 FROM follow_models follow WHERE follow.follower_user_id = notification_models.recipient_user_id AND follow.following_user_id = notification_models.actor_user_id)
    OR (path_models.owner_user_id = notification_models.recipient_user_id OR EXISTS (SELECT 1 FROM path_membership_models member WHERE member.path_id = path_models.id AND member.user_id = notification_models.recipient_user_id AND member.role IN ('administrator','participant'))))
  AND (path_models.visibility = 'public' OR path_models.owner_user_id = notification_models.recipient_user_id
    OR EXISTS (SELECT 1 FROM path_membership_models access WHERE access.path_id = path_models.id AND access.user_id = notification_models.recipient_user_id)
    OR (path_models.visibility = 'followers' AND EXISTS (SELECT 1 FROM follow_models owner_follow WHERE owner_follow.follower_user_id = notification_models.recipient_user_id AND owner_follow.following_user_id = path_models.owner_user_id)))
))`
