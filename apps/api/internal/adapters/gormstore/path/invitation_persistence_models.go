package pathstore

import (
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

type invitationModel struct {
	ID                    string `gorm:"primaryKey"`
	PathID                string
	InviterUserID         string
	RecipientUserID       string
	OfferedRole           string
	AuthorizationChangeID *string
	CreatedAt             time.Time
	AcceptedAt            *time.Time
	RejectedAt            *time.Time
	RejectionUnreadCount  *int64
	CanceledAt            *time.Time
}

func (invitationModel) TableName() string { return "path_invitation_models" }

type InvitationPersistence = invitationModel

type invitationUserModel struct {
	ID                string `gorm:"primaryKey"`
	Username          *string
	DisplayName       string
	ProfileVisibility *identity.ProfileVisibility
	Status            identity.Status
}

func (invitationUserModel) TableName() string { return "user_models" }

type notificationModel struct {
	ID                        string `gorm:"primaryKey"`
	RecipientUserID           string
	ActorUserID               string
	PathID                    string
	PathInvitationID          string
	PathOwnershipTransferID   string
	FollowRequestID           string
	FollowSubjectUserID       string
	SocialFeedEventID         string
	ReactionType              string
	CommentID                 string
	NudgeID                   string
	PathVisibility            string
	Kind                      string
	PresentationClass         string
	Channel                   string
	OfferedRole               string
	CreatedAt                 time.Time
	ReadAt                    *time.Time
	DeletedAt                 *time.Time
	InteractionDisabledReason *string
	PathNameSnapshot          *string
	ActorUsernameSnapshot     *string
	ActorDisplayNameSnapshot  *string
}

func (notificationModel) TableName() string { return "notification_models" }

type NotificationPersistence = notificationModel

type invitationNotificationRow struct {
	NotificationPersistence `gorm:"embedded"`
	PathName                string
	ActorID                 string
	ActorUsername           *string
	ActorName               string
	InvitationPathID        string
	InvitationInviterID     string
	InvitationRecipientID   string
	InvitationOfferedRole   string
	InvitationCreatedAt     *time.Time
	InvitationAcceptedAt    *time.Time
	TransferPathID          string
	TransferInitiatorID     string
	TransferRecipientID     string
	TransferCreatedAt       *time.Time
	TransferExpiresAt       *time.Time
	TransferAcceptedAt      *time.Time
	TransferDeclinedAt      *time.Time
	TransferCanceledAt      *time.Time
	TransferExpiredAt       *time.Time
	FollowRequesterID       string
	FollowTargetID          string
	FollowRequestCreatedAt  *time.Time
	FollowRequestAcceptedAt *time.Time
	FollowRequestRejectedAt *time.Time
	FollowRequestCanceledAt *time.Time
	CommentEventID          string
	CommentAuthorID         string
	EventOwnerID            string
	EventOwnerUsername      *string
	EventOwnerName          string
	NudgeRecordID           string
	NudgeSenderID           string
	NudgeRecipientID        string
	NudgePathID             string
	NudgeContentKind        string
	NudgePreset             string
	NudgeSentAt             *time.Time
}

type notificationPushOutboxModel struct {
	NotificationID string `gorm:"primaryKey"`
	CreatedAt      time.Time
}

func (notificationPushOutboxModel) TableName() string { return "notification_push_outbox_models" }

type authorizationChangeStateModel struct {
	ID             string `gorm:"primaryKey"`
	LockedBy       string
	LockedUntil    *time.Time
	CompletedAt    *time.Time
	DeadLetteredAt *time.Time
	LockActive     bool `gorm:"column:lock_active"`
}

func (authorizationChangeStateModel) TableName() string { return "authorization_outbox_models" }
