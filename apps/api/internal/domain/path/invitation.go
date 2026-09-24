package path

import (
	"strings"
	"time"
)

type InvitationID string

type MembershipRole string

const (
	RoleParticipant   MembershipRole = "participant"
	RoleSupporter     MembershipRole = "supporter"
	RoleAdministrator MembershipRole = "administrator"
)

type Invitation struct {
	ID              InvitationID
	PathID          ID
	InviterUserID   string
	RecipientUserID string
	OfferedRole     MembershipRole
	CreatedAt       time.Time
	AcceptedAt      time.Time
	RejectedAt      time.Time
	CanceledAt      time.Time
}

func NewInvitation(
	id InvitationID,
	pathID ID,
	inviterUserID string,
	recipientUserID string,
	offeredRole MembershipRole,
	createdAt time.Time,
) (Invitation, error) {
	inviterUserID = strings.TrimSpace(inviterUserID)
	recipientUserID = strings.TrimSpace(recipientUserID)
	if id == "" ||
		pathID == "" ||
		inviterUserID == "" ||
		recipientUserID == "" ||
		inviterUserID == recipientUserID ||
		!offeredRole.Valid() ||
		createdAt.IsZero() {
		return Invitation{}, ErrInvalidFields
	}
	return Invitation{
		ID:              id,
		PathID:          pathID,
		InviterUserID:   inviterUserID,
		RecipientUserID: recipientUserID,
		OfferedRole:     offeredRole,
		CreatedAt:       createdAt,
	}, nil
}

func (role MembershipRole) Valid() bool {
	return role == RoleParticipant || role == RoleSupporter
}

func (role MembershipRole) ValidPathMemberRole() bool {
	return role.Valid() || role == RoleAdministrator
}

func (invitation Invitation) Pending() bool {
	return invitation.AcceptedAt.IsZero() && invitation.RejectedAt.IsZero() && invitation.CanceledAt.IsZero()
}

func (invitation Invitation) Cancel(canceledAt time.Time) (Invitation, error) {
	if !invitation.Pending() || canceledAt.IsZero() || canceledAt.Before(invitation.CreatedAt) {
		return Invitation{}, ErrInvitationUnavailable
	}
	invitation.CanceledAt = canceledAt
	return invitation, nil
}

func (invitation Invitation) Reject(recipientUserID string, rejectedAt time.Time) (Invitation, error) {
	if !invitation.Pending() ||
		strings.TrimSpace(recipientUserID) != invitation.RecipientUserID ||
		rejectedAt.IsZero() ||
		rejectedAt.Before(invitation.CreatedAt) {
		return Invitation{}, ErrInvitationUnavailable
	}
	invitation.RejectedAt = rejectedAt
	return invitation, nil
}

func (invitation Invitation) Accept(recipientUserID string, acceptedAt time.Time) (Invitation, error) {
	if !invitation.Pending() ||
		strings.TrimSpace(recipientUserID) != invitation.RecipientUserID ||
		acceptedAt.IsZero() ||
		acceptedAt.Before(invitation.CreatedAt) {
		return Invitation{}, ErrInvitationUnavailable
	}
	invitation.AcceptedAt = acceptedAt
	return invitation, nil
}
