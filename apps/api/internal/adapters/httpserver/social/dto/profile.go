package dto

import "time"

type PublicProfile struct {
	ID                string `json:"id" required:"true"`
	Username          string `json:"username" required:"true"`
	DisplayName       string `json:"displayName" required:"true"`
	ProfilePictureURL string `json:"profilePictureUrl,omitempty" format:"uri"`
	Description       string `json:"description,omitempty" maxLength:"500"`
	FollowerCount     int64  `json:"followerCount" required:"true" minimum:"0"`
	FollowingCount    int64  `json:"followingCount" required:"true" minimum:"0"`
	Relationship      string `json:"relationship" required:"true" enum:"self,none,requested,following"`
}

type FollowRequest struct {
	ID        string        `json:"id" required:"true"`
	Requester PublicProfile `json:"requester" required:"true"`
	CreatedAt time.Time     `json:"createdAt" required:"true" format:"date-time"`
}
