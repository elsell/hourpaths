package dto

import "time"

type OwnershipTransferCandidate struct {
	UserID        string `json:"userId" required:"true"`
	Username      string `json:"username" required:"true"`
	DisplayName   string `json:"displayName" required:"true"`
	Administrator bool   `json:"administrator" required:"true"`
}

type OwnershipTransferCreate struct {
	ReservationToken string `json:"reservationToken" required:"true" minLength:"1" maxLength:"4096"`
}

type OwnershipTransferReviewRequest struct {
	RecipientUserID string `json:"recipientUserId" required:"true" minLength:"1"`
}

type OwnershipTransferPublicIdentity struct {
	UserID      string `json:"userId" required:"true"`
	Username    string `json:"username" required:"true"`
	DisplayName string `json:"displayName" required:"true"`
}

type OwnershipTransferReview struct {
	Recipient        OwnershipTransferPublicIdentity `json:"recipient" required:"true"`
	ReviewedAt       time.Time                       `json:"reviewedAt" required:"true"`
	ExpiresAt        time.Time                       `json:"expiresAt" required:"true"`
	ReservationToken string                          `json:"reservationToken" required:"true"`
	ViewerTimeZone   string                          `json:"viewerTimeZone" required:"true"`
}

type OwnershipTransfer struct {
	ID              string     `json:"id" required:"true"`
	PathID          string     `json:"pathId" required:"true"`
	CreatorUserID   string     `json:"creatorUserId" required:"true"`
	RecipientUserID string     `json:"recipientUserId" required:"true"`
	ReviewedAt      time.Time  `json:"reviewedAt" required:"true"`
	CreatedAt       time.Time  `json:"createdAt" required:"true"`
	ExpiresAt       time.Time  `json:"expiresAt" required:"true"`
	State           string     `json:"state" required:"true" enum:"pending,accepted,declined,canceled"`
	AcceptedAt      *time.Time `json:"acceptedAt,omitempty"`
	DeclinedAt      *time.Time `json:"declinedAt,omitempty"`
	CanceledAt      *time.Time `json:"canceledAt,omitempty"`
}

type OwnershipTransferResult struct {
	Transfer        OwnershipTransfer               `json:"transfer" required:"true"`
	Path            *PathItem                       `json:"path,omitempty"`
	Replayed        bool                            `json:"replayed" required:"true"`
	Counterpart     OwnershipTransferPublicIdentity `json:"counterpart" required:"true"`
	CounterpartRole string                          `json:"counterpartRole" required:"true" enum:"creator,recipient"`
	ViewerTimeZone  string                          `json:"viewerTimeZone" required:"true"`
}
