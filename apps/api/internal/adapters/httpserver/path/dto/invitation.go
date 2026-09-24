package dto

import "time"

type PathInvitationCreate struct {
	Username                string `json:"username" required:"true" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
	ExpectedRecipientUserID string `json:"expectedRecipientUserId" required:"true" minLength:"1"`
	OfferedRole             string `json:"offeredRole" required:"true" enum:"participant,supporter"`
}

type PathInvitationRecipient struct {
	UserID      string `json:"userId" required:"true"`
	Username    string `json:"username" required:"true"`
	DisplayName string `json:"displayName" required:"true"`
}

type PathInvitation struct {
	ID              string     `json:"id" required:"true"`
	PathID          string     `json:"pathId" required:"true"`
	InviterUserID   string     `json:"inviterUserId" required:"true"`
	RecipientUserID string     `json:"recipientUserId" required:"true"`
	OfferedRole     string     `json:"offeredRole" required:"true" enum:"participant,supporter"`
	CreatedAt       time.Time  `json:"createdAt" required:"true"`
	AcceptedAt      *time.Time `json:"acceptedAt,omitempty"`
}

type PendingPathInvitation struct {
	Invitation PathInvitation                   `json:"invitation" required:"true"`
	PathName   string                           `json:"pathName" required:"true"`
	Inviter    PathInvitationPublicIdentity     `json:"inviter" required:"true"`
	Warning    *PathInvitationVisibilityWarning `json:"warning,omitempty"`
}

type ManagedPathInvitation struct {
	Invitation PathInvitation               `json:"invitation" required:"true"`
	Inviter    PathInvitationPublicIdentity `json:"inviter" required:"true"`
	Recipient  PathInvitationPublicIdentity `json:"recipient" required:"true"`
}

type PathInvitationCancellation struct {
	InvitationID string    `json:"invitationId" required:"true"`
	CanceledAt   time.Time `json:"canceledAt" required:"true"`
}

type PathInvitationVisibilityWarning struct {
	PathVisibility      string `json:"pathVisibility" required:"true" enum:"followers,public"`
	HasRetainedActivity bool   `json:"hasRetainedActivity" required:"true"`
}

type PathInvitationVisibilityWarningAcknowledgement struct {
	PathVisibility string `json:"pathVisibility" required:"true" enum:"followers,public"`
}

type PathInvitationAccept struct {
	VisibilityWarningAcknowledgement *PathInvitationVisibilityWarningAcknowledgement `json:"visibilityWarningAcknowledgement,omitempty"`
}

type PathInvitationRejection struct {
	InvitationID string    `json:"invitationId" required:"true"`
	RejectedAt   time.Time `json:"rejectedAt" required:"true"`
	UnreadCount  int64     `json:"unreadCount" required:"true" minimum:"0"`
}

type PathInvitationPublicIdentity struct {
	UserID      string `json:"userId" required:"true"`
	Username    string `json:"username" required:"true"`
	DisplayName string `json:"displayName" required:"true"`
}

type PathInvitationNotification struct {
	ID                  string                       `json:"id" required:"true"`
	Type                string                       `json:"type" required:"true" enum:"path_invitation_received,path_invitation_accepted,path_ownership_transfer_received,path_ownership_transfer_accepted,path_ownership_transfer_declined,path_ownership_transfer_canceled,path_deleted,path_member_left,path_member_removed,path_member_role_changed,path_visibility_changed,new_follower,follow_request_received,follow_request_accepted,practice_reaction,practice_comment,comment_heart,nudge_received"`
	Presentation        string                       `json:"presentation" required:"true" enum:"actionable,informational"`
	Read                bool                         `json:"read" required:"true"`
	CreatedAt           time.Time                    `json:"createdAt" required:"true"`
	Actor               PathInvitationPublicIdentity `json:"actor" required:"true"`
	PathID              string                       `json:"pathId,omitempty"`
	PathName            string                       `json:"pathName,omitempty"`
	PathVisibility      string                       `json:"pathVisibility,omitempty" enum:"private,followers,public"`
	InvitationID        string                       `json:"invitationId,omitempty"`
	OfferedRole         string                       `json:"offeredRole,omitempty" enum:"participant,supporter,administrator"`
	OwnershipTransferID string                       `json:"ownershipTransferId,omitempty"`
	FollowRequestID     string                       `json:"followRequestId,omitempty"`
	SocialFeedEventID   string                       `json:"socialFeedEventId,omitempty"`
	Reaction            string                       `json:"reaction,omitempty" enum:"heart,applause,fire,strong,celebrate"`
	CommentID           string                       `json:"commentId,omitempty"`
	InteractionDisabled string                       `json:"interactionDisabled,omitempty" enum:"comments,reactions"`
	Content             *NudgeNotificationContent    `json:"content,omitempty"`
}

type NudgeNotificationContent struct {
	Kind   string `json:"kind" required:"true" enum:"preset"`
	Preset string `json:"preset" required:"true" enum:"you_have_got_this,lets_go,little_progress_counts,keep_it_going,time_to_work"`
}

type NotificationMutationResult struct {
	UnreadCount int64 `json:"unreadCount" required:"true" minimum:"0"`
}

type NotificationListMeta struct {
	NextCursor  string `json:"nextCursor,omitempty"`
	UnreadCount int64  `json:"unreadCount" required:"true" minimum:"0"`
}
