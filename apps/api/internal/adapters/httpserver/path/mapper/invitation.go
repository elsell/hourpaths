package mapper

import (
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func InvitationRecipient(value pathapp.InvitationRecipientReview) dto.PathInvitationRecipient {
	return dto.PathInvitationRecipient{
		UserID: value.UserID, Username: value.Username, DisplayName: value.DisplayName,
	}
}

func Invitation(value domain.Invitation) dto.PathInvitation {
	result := dto.PathInvitation{
		ID:              string(value.ID),
		PathID:          string(value.PathID),
		InviterUserID:   value.InviterUserID,
		RecipientUserID: value.RecipientUserID,
		OfferedRole:     string(value.OfferedRole),
		CreatedAt:       value.CreatedAt,
	}
	if !value.AcceptedAt.IsZero() {
		acceptedAt := value.AcceptedAt
		result.AcceptedAt = &acceptedAt
	}
	return result
}

func InvitationRejection(value pathapp.RejectInvitationResult) dto.PathInvitationRejection {
	return dto.PathInvitationRejection{
		InvitationID: string(value.Invitation.ID),
		RejectedAt:   value.Invitation.RejectedAt,
		UnreadCount:  value.UnreadCount,
	}
}

func PendingInvitation(value pathapp.PendingInvitation) dto.PendingPathInvitation {
	result := dto.PendingPathInvitation{
		Invitation: Invitation(value.Invitation),
		PathName:   value.PathName,
		Inviter: dto.PathInvitationPublicIdentity{
			UserID: value.Inviter.UserID, Username: value.Inviter.Username,
			DisplayName: value.Inviter.DisplayName,
		},
	}
	if value.Warning != nil {
		result.Warning = &dto.PathInvitationVisibilityWarning{
			PathVisibility:      value.Warning.PathVisibility,
			HasRetainedActivity: value.Warning.HasRetainedActivity,
		}
	}
	return result
}

func ManagedInvitation(value pathapp.ManagedInvitation) dto.ManagedPathInvitation {
	return dto.ManagedPathInvitation{Invitation: Invitation(value.Invitation), Inviter: publicIdentity(value.Inviter), Recipient: publicIdentity(value.Recipient)}
}

func InvitationCancellation(value pathapp.CancelInvitationResult) dto.PathInvitationCancellation {
	return dto.PathInvitationCancellation{InvitationID: string(value.Invitation.ID), CanceledAt: value.Invitation.CanceledAt}
}

func publicIdentity(value pathapp.InvitationPublicIdentity) dto.PathInvitationPublicIdentity {
	return dto.PathInvitationPublicIdentity{UserID: value.UserID, Username: value.Username, DisplayName: value.DisplayName}
}

func InvitationNotification(value pathapp.InvitationNotificationProjection) dto.PathInvitationNotification {
	result := dto.PathInvitationNotification{
		ID: value.ID, Type: string(value.Kind), Presentation: string(value.Presentation),
		Read: value.Read, CreatedAt: value.CreatedAt,
		Actor: dto.PathInvitationPublicIdentity{
			UserID: value.Actor.UserID, Username: value.Actor.Username, DisplayName: value.Actor.DisplayName,
		},
		InteractionDisabled: string(value.InteractionDisabled),
	}
	switch value.Kind {
	case pathapp.NotificationPathDeleted:
		result.PathName = value.PathName
	case pathapp.NotificationPracticeReaction:
		result.PathID = string(value.PathID)
		result.PathName = value.PathName
		result.SocialFeedEventID = value.SocialFeedEventID
		if value.InteractionDisabled == "" {
			result.Reaction = string(value.Reaction)
		}
	case pathapp.NotificationPracticeComment, pathapp.NotificationCommentHeart:
		result.PathID = string(value.PathID)
		result.PathName = value.PathName
		result.SocialFeedEventID = value.SocialFeedEventID
		if value.InteractionDisabled == "" {
			result.CommentID = value.CommentID
		}
	case pathapp.NotificationNudgeReceived:
		result.PathID = string(value.PathID)
		result.PathName = value.PathName
		result.Content = &dto.NudgeNotificationContent{
			Kind: string(value.NudgeContent.Kind), Preset: string(value.NudgeContent.Preset),
		}
	case pathapp.NotificationPathVisibilityChanged:
		result.PathID = string(value.PathID)
		result.PathName = value.PathName
		result.PathVisibility = value.PathVisibility
	default:
		result.PathID = string(value.PathID)
		result.PathName = value.PathName
		result.InvitationID = string(value.InvitationID)
		result.OfferedRole = string(value.OfferedRole)
		result.OwnershipTransferID = string(value.OwnershipTransferID)
		result.FollowRequestID = value.FollowRequestID
	}
	return result
}
