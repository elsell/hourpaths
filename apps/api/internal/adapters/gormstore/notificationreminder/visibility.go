package notificationreminder

const PathAccessPredicate = `EXISTS (SELECT 1 FROM user_models reminder_user WHERE reminder_user.id = reminder_receipt.participant_id AND reminder_user.status = 'active')
 AND EXISTS (SELECT 1 FROM path_membership_models reminder_member WHERE reminder_member.path_id = reminder_path.id AND reminder_member.user_id = reminder_receipt.participant_id AND reminder_member.role IN ('participant','administrator'))
 AND NOT EXISTS (SELECT 1 FROM block_models reminder_block WHERE (reminder_block.blocker_user_id = reminder_receipt.participant_id AND reminder_block.blocked_user_id = reminder_path.owner_user_id) OR (reminder_block.blocker_user_id = reminder_path.owner_user_id AND reminder_block.blocked_user_id = reminder_receipt.participant_id))`

const VisiblePredicate = `(notification_models.kind <> 'goal_practice_reminder' OR (notification_models.actor_user_id = notification_models.recipient_user_id AND EXISTS (
 SELECT 1 FROM goal_reminder_receipt_models reminder_receipt JOIN path_models reminder_path ON reminder_path.id = reminder_receipt.path_id
 WHERE reminder_receipt.notification_id = notification_models.id AND reminder_receipt.participant_id = notification_models.recipient_user_id AND ` + PathAccessPredicate + `)))`
