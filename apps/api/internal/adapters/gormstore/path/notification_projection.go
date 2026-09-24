package pathstore

import "gorm.io/gorm"

const notificationPairVisiblePredicate = `NOT EXISTS (SELECT 1 FROM block_models notification_block
  WHERE (notification_block.blocker_user_id = notification_models.recipient_user_id AND
         notification_block.blocked_user_id = notification_models.actor_user_id)
     OR (notification_block.blocker_user_id = notification_models.actor_user_id AND
         notification_block.blocked_user_id = notification_models.recipient_user_id))`

const visibleNotificationPredicate = `notification_models.deleted_at IS NULL AND
  ` + notificationPairVisiblePredicate + ` AND
  (notification_models.kind <> 'path_invitation_received' OR
    (path_invitation_models.accepted_at IS NULL AND
     path_invitation_models.rejected_at IS NULL AND
     path_invitation_models.canceled_at IS NULL)) AND
  (notification_models.kind <> 'path_ownership_transfer_received' OR
    (path_ownership_transfer_models.accepted_at IS NULL AND
     path_ownership_transfer_models.declined_at IS NULL AND
     path_ownership_transfer_models.canceled_at IS NULL AND
     path_ownership_transfer_models.expired_at IS NULL AND
     path_ownership_transfer_models.expires_at > CURRENT_TIMESTAMP)) AND
  (notification_models.kind <> 'follow_request_received' OR
    (follow_request_models.accepted_at IS NULL AND
     follow_request_models.rejected_at IS NULL AND
     follow_request_models.canceled_at IS NULL))`

func notificationProjectionQuery(tx *gorm.DB) *gorm.DB {
	return joinNotificationSubjects(tx.Table("notification_models")).
		Select(`notification_models.id,
      notification_models.recipient_user_id,
      notification_models.actor_user_id,
      notification_models.path_id,
      COALESCE(notification_models.path_invitation_id, '') AS path_invitation_id,
      COALESCE(notification_models.path_ownership_transfer_id, '') AS path_ownership_transfer_id,
      COALESCE(notification_models.follow_request_id, '') AS follow_request_id,
      COALESCE(notification_models.follow_subject_user_id, '') AS follow_subject_user_id,
      COALESCE(notification_models.social_feed_event_id, '') AS social_feed_event_id,
      COALESCE(notification_models.reaction_type, '') AS reaction_type,
      COALESCE(notification_models.comment_id, '') AS comment_id,
      COALESCE(notification_models.nudge_id, '') AS nudge_id,
      COALESCE(notification_models.path_visibility, '') AS path_visibility,
      notification_models.kind,
      notification_models.presentation_class,
      notification_models.channel,
      COALESCE(notification_models.offered_role, '') AS offered_role,
      notification_models.created_at,
      notification_models.read_at,
      notification_models.deleted_at,
      notification_models.interaction_disabled_reason,
      COALESCE(path_models.name, notification_models.path_name_snapshot, '') AS path_name,
      notification_models.path_name_snapshot,
      notification_models.actor_username_snapshot,
      notification_models.actor_display_name_snapshot,
      notification_actor.id AS actor_id,
      notification_actor.username AS actor_username,
      notification_actor.display_name AS actor_name,
      COALESCE(path_invitation_models.path_id, '') AS invitation_path_id,
      COALESCE(path_invitation_models.inviter_user_id, '') AS invitation_inviter_id,
      COALESCE(path_invitation_models.recipient_user_id, '') AS invitation_recipient_id,
      COALESCE(path_invitation_models.offered_role, '') AS invitation_offered_role,
      path_invitation_models.created_at AS invitation_created_at,
      path_invitation_models.accepted_at AS invitation_accepted_at,
      COALESCE(path_ownership_transfer_models.path_id, '') AS transfer_path_id,
      COALESCE(path_ownership_transfer_models.initiator_user_id, '') AS transfer_initiator_id,
      COALESCE(path_ownership_transfer_models.recipient_user_id, '') AS transfer_recipient_id,
      path_ownership_transfer_models.created_at AS transfer_created_at,
      path_ownership_transfer_models.expires_at AS transfer_expires_at,
      path_ownership_transfer_models.accepted_at AS transfer_accepted_at,
      path_ownership_transfer_models.declined_at AS transfer_declined_at,
      path_ownership_transfer_models.canceled_at AS transfer_canceled_at,
      path_ownership_transfer_models.expired_at AS transfer_expired_at,
      COALESCE(follow_request_models.requester_user_id, '') AS follow_requester_id,
      COALESCE(follow_request_models.target_user_id, '') AS follow_target_id,
      follow_request_models.created_at AS follow_request_created_at,
      follow_request_models.accepted_at AS follow_request_accepted_at,
      follow_request_models.rejected_at AS follow_request_rejected_at,
      follow_request_models.canceled_at AS follow_request_canceled_at,
      COALESCE(social_practice_comment_models.social_feed_event_id, '') AS comment_event_id,
      COALESCE(social_practice_comment_models.author_user_id, '') AS comment_author_id,
      COALESCE(notification_event.participant_user_id, '') AS event_owner_id,
      event_owner.username AS event_owner_username,
      COALESCE(event_owner.display_name, '') AS event_owner_name,
      COALESCE(social_nudge_models.id, '') AS nudge_record_id,
      COALESCE(social_nudge_models.sender_user_id, '') AS nudge_sender_id,
      COALESCE(social_nudge_models.recipient_user_id, '') AS nudge_recipient_id,
      COALESCE(social_nudge_models.path_id, '') AS nudge_path_id,
      COALESCE(social_nudge_models.content_kind, '') AS nudge_content_kind,
      COALESCE(social_nudge_models.preset, '') AS nudge_preset,
      social_nudge_models.sent_at AS nudge_sent_at`)
}

func joinNotificationSubjects(query *gorm.DB) *gorm.DB {
	return query.
		Joins("LEFT JOIN path_models ON path_models.id = notification_models.path_id").
		Joins("LEFT JOIN path_invitation_models ON path_invitation_models.id = notification_models.path_invitation_id").
		Joins("LEFT JOIN path_ownership_transfer_models ON path_ownership_transfer_models.id = notification_models.path_ownership_transfer_id").
		Joins("LEFT JOIN follow_request_models ON follow_request_models.id = notification_models.follow_request_id").
		Joins("LEFT JOIN social_practice_comment_models ON social_practice_comment_models.id = notification_models.comment_id").
		Joins("LEFT JOIN social_nudge_models ON social_nudge_models.id = notification_models.nudge_id").
		Joins("LEFT JOIN social_feed_event_models notification_event ON notification_event.id = notification_models.social_feed_event_id").
		Joins("LEFT JOIN user_models event_owner ON event_owner.id = notification_event.participant_user_id AND event_owner.status = 'active'").
		Joins("JOIN user_models AS notification_actor ON notification_actor.id = notification_models.actor_user_id")
}
