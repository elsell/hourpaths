package dto

import "time"

type PracticeCommentInput struct {
	Text string `json:"text" required:"true" minLength:"1" maxLength:"2000"`
}

type PracticeCommentEditInput struct {
	Text            string `json:"text" required:"true" minLength:"1" maxLength:"2000"`
	ExpectedVersion int64  `json:"expectedVersion" required:"true" minimum:"1"`
}

type PracticeComment struct {
	ID           string    `json:"id" required:"true"`
	EventID      string    `json:"eventId" required:"true"`
	AuthorUserID string    `json:"authorUserId" required:"true"`
	Text         string    `json:"text" required:"true" maxLength:"2000"`
	Version      int64     `json:"version" required:"true" minimum:"1"`
	CreatedAt    time.Time `json:"createdAt" required:"true" format:"date-time"`
	UpdatedAt    time.Time `json:"updatedAt" required:"true" format:"date-time"`
	Edited       bool      `json:"edited" required:"true"`
}

type PracticeCommentItem struct {
	Comment         PracticeComment `json:"comment" required:"true"`
	Author          PublicProfile   `json:"author" required:"true"`
	HeartCount      int64           `json:"heartCount" required:"true" minimum:"0"`
	HeartedByViewer bool            `json:"heartedByViewer" required:"true"`
}

type CommentHeartSummary struct {
	CommentID       string `json:"commentId" required:"true"`
	HeartCount      int64  `json:"heartCount" required:"true" minimum:"0"`
	HeartedByViewer bool   `json:"heartedByViewer" required:"true"`
}

type CommentVersion struct {
	CommentID string    `json:"commentId" required:"true"`
	Text      string    `json:"text" required:"true" maxLength:"2000"`
	Version   int64     `json:"version" required:"true" minimum:"1"`
	CreatedAt time.Time `json:"createdAt" required:"true" format:"date-time"`
}
