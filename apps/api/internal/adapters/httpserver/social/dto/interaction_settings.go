package dto

type InteractionSettings struct {
	CommentsEnabled  bool `json:"commentsEnabled" required:"true"`
	ReactionsEnabled bool `json:"reactionsEnabled" required:"true"`
}
