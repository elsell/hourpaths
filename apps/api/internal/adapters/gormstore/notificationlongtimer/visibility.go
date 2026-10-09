package notificationlongtimer

const VisiblePredicate = `(notification_models.kind <> 'long_timer_running' OR (
 notification_models.recipient_user_id = notification_models.actor_user_id
 AND path_models.id IS NOT NULL
 AND EXISTS (SELECT 1 FROM user_models recipient WHERE recipient.id = notification_models.recipient_user_id AND recipient.status = 'active')
 AND EXISTS (SELECT 1 FROM path_membership_models member WHERE member.path_id = path_models.id AND member.user_id = notification_models.recipient_user_id AND member.role IN ('administrator','participant'))
 AND NOT EXISTS (SELECT 1 FROM block_models b WHERE (b.blocker_user_id = notification_models.recipient_user_id AND b.blocked_user_id = path_models.owner_user_id) OR (b.blocked_user_id = notification_models.recipient_user_id AND b.blocker_user_id = path_models.owner_user_id))
))`
