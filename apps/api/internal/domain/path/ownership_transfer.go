package path

import (
	"strings"
	"time"
)

type OwnershipTransferID string

// OwnershipTransfer records a proposed ownership change without applying role
// changes. Role mutation belongs to the application transaction that accepts it.
type OwnershipTransfer struct {
	ID              OwnershipTransferID
	PathID          ID
	InitiatorUserID string
	RecipientUserID string
	ReviewedAt      time.Time
	CreatedAt       time.Time
	ExpiresAt       time.Time
	AcceptedAt      time.Time
	DeclinedAt      time.Time
	CanceledAt      time.Time
}

func NewOwnershipTransfer(
	id OwnershipTransferID,
	pathID ID,
	initiatorUserID string,
	recipientUserID string,
	createdAt time.Time,
	expiresAt time.Time,
) (OwnershipTransfer, error) {
	return NewReviewedOwnershipTransfer(id, pathID, initiatorUserID, recipientUserID, createdAt, createdAt, expiresAt)
}

func NewReviewedOwnershipTransfer(
	id OwnershipTransferID,
	pathID ID,
	initiatorUserID string,
	recipientUserID string,
	reviewedAt time.Time,
	createdAt time.Time,
	expiresAt time.Time,
) (OwnershipTransfer, error) {
	id = OwnershipTransferID(strings.TrimSpace(string(id)))
	pathID = ID(strings.TrimSpace(string(pathID)))
	initiatorUserID = strings.TrimSpace(initiatorUserID)
	recipientUserID = strings.TrimSpace(recipientUserID)
	if id == "" ||
		pathID == "" ||
		initiatorUserID == "" ||
		recipientUserID == "" ||
		initiatorUserID == recipientUserID ||
		reviewedAt.IsZero() ||
		createdAt.IsZero() ||
		expiresAt.IsZero() ||
		createdAt.Before(reviewedAt) ||
		!expiresAt.After(createdAt) {
		return OwnershipTransfer{}, ErrInvalidFields
	}

	return OwnershipTransfer{
		ID:              id,
		PathID:          pathID,
		InitiatorUserID: initiatorUserID,
		RecipientUserID: recipientUserID,
		ReviewedAt:      reviewedAt,
		CreatedAt:       createdAt,
		ExpiresAt:       expiresAt,
	}, nil
}

func (transfer OwnershipTransfer) Pending(at time.Time) bool {
	return transfer.terminalAt().IsZero() &&
		!at.IsZero() &&
		!at.Before(transfer.CreatedAt) &&
		at.Before(transfer.ExpiresAt)
}

func (transfer OwnershipTransfer) Expired(at time.Time) bool {
	return transfer.terminalAt().IsZero() &&
		!at.IsZero() &&
		!at.Before(transfer.ExpiresAt)
}

func (transfer OwnershipTransfer) Accept(recipientUserID string, acceptedAt time.Time) (OwnershipTransfer, error) {
	if strings.TrimSpace(recipientUserID) != transfer.RecipientUserID || !transfer.Pending(acceptedAt) {
		return OwnershipTransfer{}, ErrOwnershipTransferUnavailable
	}
	transfer.AcceptedAt = acceptedAt
	return transfer, nil
}

func (transfer OwnershipTransfer) Decline(recipientUserID string, declinedAt time.Time) (OwnershipTransfer, error) {
	if strings.TrimSpace(recipientUserID) != transfer.RecipientUserID || !transfer.Pending(declinedAt) {
		return OwnershipTransfer{}, ErrOwnershipTransferUnavailable
	}
	transfer.DeclinedAt = declinedAt
	return transfer, nil
}

func (transfer OwnershipTransfer) Cancel(initiatorUserID string, canceledAt time.Time) (OwnershipTransfer, error) {
	if strings.TrimSpace(initiatorUserID) != transfer.InitiatorUserID || !transfer.Pending(canceledAt) {
		return OwnershipTransfer{}, ErrOwnershipTransferUnavailable
	}
	transfer.CanceledAt = canceledAt
	return transfer, nil
}

func (transfer OwnershipTransfer) terminalAt() time.Time {
	if !transfer.AcceptedAt.IsZero() {
		return transfer.AcceptedAt
	}
	if !transfer.DeclinedAt.IsZero() {
		return transfer.DeclinedAt
	}
	return transfer.CanceledAt
}
