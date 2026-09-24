package dto

import "time"

type BlockTarget struct {
	UserID      string `json:"userId" required:"true"`
	Username    string `json:"username" required:"true"`
	DisplayName string `json:"displayName" required:"true"`
}

type BlockSharedPath struct {
	ID   string `json:"id" required:"true"`
	Name string `json:"name" required:"true"`
}

type BlockReviewAcknowledgement struct {
	Version   int       `json:"version" required:"true" minimum:"1" maximum:"1"`
	Token     string    `json:"token" required:"true" minLength:"1" maxLength:"8192"`
	ExpiresAt time.Time `json:"expiresAt" required:"true" format:"date-time"`
}

type BlockedAccount struct {
	BlockTarget
	BlockedAt time.Time `json:"blockedAt" required:"true" format:"date-time"`
}
