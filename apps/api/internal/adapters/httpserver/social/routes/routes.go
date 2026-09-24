package routes

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/dto"
	application "github.com/elsell/hour-paths/apps/api/internal/app/social"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
)

type Service interface {
	GetNudgeNotificationChannel(context.Context, string) (application.NudgeNotificationChannelPreference, error)
	UpdateNudgeNotificationChannel(context.Context, string, string, int64, bool) (application.NudgeNotificationChannelPreference, error)
	GetNudgeAudience(context.Context, string, string) (application.NudgeAudiencePreference, error)
	UpdateNudgeAudience(context.Context, string, string, string, int64, domain.NudgeAudience) (application.NudgeAudiencePreference, error)
	GetNudgeEligibility(context.Context, string, string, string) (application.NudgeEligibility, error)
	SendNudge(context.Context, string, string, string, domain.NudgeContent, string) (domain.Nudge, error)
	GetInteractionSettings(context.Context, string) (application.InteractionSettings, error)
	UpdateInteractionSettings(context.Context, string, string, application.InteractionSettings) (application.InteractionSettings, error)
	ListPracticeFeed(context.Context, string, string, int) ([]application.PracticeFeedItem, string, error)
	GetPracticeFeedEvent(context.Context, string, string) (application.PracticeFeedItem, error)
	ListActiveFollowing(context.Context, string, string, int) ([]application.ActiveFollowingCandidate, string, error)
	SetPracticeReaction(context.Context, string, string, domain.Reaction, string) (domain.ReactionSummary, error)
	RemovePracticeReaction(context.Context, string, string, string) (domain.ReactionSummary, error)
	ListPracticeComments(context.Context, string, string, string, int) ([]application.PracticeCommentItem, string, error)
	ListCommentHistory(context.Context, string, string, string, string, int) ([]domain.CommentVersion, string, error)
	CreatePracticeComment(context.Context, string, string, string, string) (domain.Comment, error)
	EditPracticeComment(context.Context, string, string, string, string, int64, string) (domain.Comment, error)
	DeletePracticeComment(context.Context, string, string, string, string) (application.CommentDeleteResult, error)
	SetPracticeCommentHeart(context.Context, string, string, string, string) (application.CommentHeartSummary, error)
	RemovePracticeCommentHeart(context.Context, string, string, string, string) (application.CommentHeartSummary, error)
	ListPracticeCommentHearts(context.Context, string, string, string, string, int) ([]domain.PublicProfile, string, error)
	Search(context.Context, string, string, string, int) ([]domain.PublicProfile, string, error)
	Get(context.Context, string, string) (domain.PublicProfile, error)
	Follow(context.Context, string, string, string) (application.RelationshipResult, error)
	CancelRequest(context.Context, string, string, string) (application.RelationshipResult, error)
	Unfollow(context.Context, string, string, string) (application.RelationshipResult, error)
	AcceptRequest(context.Context, string, string, string) (application.ReviewResult, error)
	RejectRequest(context.Context, string, string, string) (application.ReviewResult, error)
	ListIncomingRequests(context.Context, string, string, int) ([]domain.FollowRequest, string, error)
	ReviewBlock(context.Context, string, string) (application.BlockReview, error)
	BlockUser(context.Context, string, string, string, application.BlockReviewAcknowledgement) (application.BlockMutationResult, error)
	ListBlockedAccounts(context.Context, string, string, int) ([]domain.BlockedAccount, string, error)
	UnblockUser(context.Context, string, string, string) (application.BlockMutationResult, error)
}

type interactionSettingsInput struct {
	Authorization string `header:"Authorization"`
}
type interactionSettingsUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           dto.InteractionSettings
}
type interactionSettingsOutput struct {
	Body struct {
		Data dto.InteractionSettings `json:"data"`
	}
}

type searchInput struct {
	Authorization string `header:"Authorization"`
	Query         string `query:"query" required:"true" minLength:"2" maxLength:"100"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type profileInput struct {
	Authorization string `header:"Authorization"`
	Username      string `path:"username" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
}
type profileMutationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Username       string `path:"username" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
}
type requestMutationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	RequestID      string `path:"requestId" minLength:"1" maxLength:"128"`
}
type followRequestsInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type blockReviewInput struct {
	Authorization string `header:"Authorization"`
	Username      string `path:"username" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
}
type blockMutationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Username       string `path:"username" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
	Body           struct {
		Acknowledgement dto.BlockReviewAcknowledgement `json:"acknowledgement"`
	}
}
type blockedAccountsInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type unblockMutationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	UserID         string `path:"userId" minLength:"1" maxLength:"128"`
}
type practiceFeedInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type practiceFeedEventInput struct {
	Authorization string `header:"Authorization"`
	EventID       string `path:"eventId" minLength:"1" maxLength:"128"`
}
type activeFollowingInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type practiceReactionInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	EventID        string `path:"eventId" minLength:"1" maxLength:"128"`
	Body           dto.PracticeReactionInput
}
type practiceReactionRemovalInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	EventID        string `path:"eventId" minLength:"1" maxLength:"128"`
}
type practiceCommentsInput struct {
	Authorization string `header:"Authorization"`
	EventID       string `path:"eventId" minLength:"1" maxLength:"128"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type practiceCommentCreationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	EventID        string `path:"eventId" minLength:"1" maxLength:"128"`
	Body           dto.PracticeCommentInput
}
type practiceCommentEditInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	EventID        string `path:"eventId" minLength:"1" maxLength:"128"`
	CommentID      string `path:"commentId" minLength:"1" maxLength:"128"`
	Body           dto.PracticeCommentEditInput
}
type practiceCommentRemovalInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	EventID        string `path:"eventId" minLength:"1" maxLength:"128"`
	CommentID      string `path:"commentId" minLength:"1" maxLength:"128"`
}
type commentHistoryInput struct {
	Authorization string `header:"Authorization"`
	EventID       string `path:"eventId" minLength:"1" maxLength:"128"`
	CommentID     string `path:"commentId" minLength:"1" maxLength:"128"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type practiceCommentHeartInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	EventID        string `path:"eventId" minLength:"1" maxLength:"128"`
	CommentID      string `path:"commentId" minLength:"1" maxLength:"128"`
}
type practiceCommentHeartsInput struct {
	Authorization string `header:"Authorization"`
	EventID       string `path:"eventId" minLength:"1" maxLength:"128"`
	CommentID     string `path:"commentId" minLength:"1" maxLength:"128"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type searchOutput struct {
	Body struct {
		Data []dto.PublicProfile `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type profileOutput struct {
	Body struct {
		Data dto.PublicProfile `json:"data"`
	}
}
type relationshipData struct {
	Profile   dto.PublicProfile `json:"profile"`
	RequestID string            `json:"requestId,omitempty"`
}
type relationshipOutput struct {
	Body struct {
		Data relationshipData `json:"data"`
	}
}
type reviewData struct {
	Request  dto.FollowRequest `json:"request"`
	Decision string            `json:"decision" enum:"accepted,rejected"`
}
type reviewOutput struct {
	Body struct {
		Data reviewData `json:"data"`
	}
}
type followRequestsOutput struct {
	Body struct {
		Data []dto.FollowRequest `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type blockReviewData struct {
	Target          dto.BlockTarget                `json:"target"`
	SharedPaths     []dto.BlockSharedPath          `json:"sharedPaths" nullable:"false"`
	Acknowledgement dto.BlockReviewAcknowledgement `json:"acknowledgement"`
}
type blockReviewOutput struct {
	Body struct {
		Data blockReviewData `json:"data"`
	}
}
type blockMutationData struct {
	Target  dto.BlockTarget `json:"target"`
	Blocked bool            `json:"blocked" required:"true"`
}
type blockMutationOutput struct {
	Body struct {
		Data blockMutationData `json:"data"`
	}
}
type blockedAccountsOutput struct {
	Body struct {
		Data []dto.BlockedAccount `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type practiceFeedData struct {
	Items []dto.PracticeFeedItem `json:"items" nullable:"false"`
}
type practiceFeedOutput struct {
	Body struct {
		Data practiceFeedData `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type practiceFeedEventOutput struct {
	Body struct {
		Data dto.PracticeFeedItem `json:"data"`
	}
}
type activeFollowingData struct {
	Items []dto.ActiveFollowingItem `json:"items" nullable:"false"`
}
type activeFollowingOutput struct {
	Body struct {
		Data activeFollowingData `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type practiceReactionOutput struct {
	Body struct {
		Data dto.PracticeReactionSummary `json:"data"`
	}
}
type practiceCommentListData struct {
	Items []dto.PracticeCommentItem `json:"items" nullable:"false"`
}
type practiceCommentListOutput struct {
	Body struct {
		Data practiceCommentListData `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type practiceCommentOutput struct {
	Status int
	Body   struct {
		Data dto.PracticeComment `json:"data"`
	}
}
type commentHistoryData struct {
	Versions []dto.CommentVersion `json:"versions" nullable:"false"`
}
type commentHistoryOutput struct {
	Body struct {
		Data commentHistoryData `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}
type practiceCommentHeartOutput struct {
	Body struct {
		Data dto.CommentHeartSummary `json:"data"`
	}
}
type practiceCommentHeartRosterData struct {
	Items []dto.PublicProfile `json:"items" nullable:"false"`
}
type practiceCommentHeartRosterOutput struct {
	Body struct {
		Data practiceCommentHeartRosterData `json:"data"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}

func Register(api huma.API, service Service) {
	security := []map[string][]string{{"oidc": {}}}
	registerNudgeRoutes(api, service, security)
	huma.Register(api, huma.Operation{OperationID: "get-interaction-settings", Method: http.MethodGet, Path: "/v1/me/interaction-settings", Summary: "Get the viewer's interaction settings", Security: security}, func(ctx context.Context, input *interactionSettingsInput) (*interactionSettingsOutput, error) {
		settings, err := service.GetInteractionSettings(ctx, input.Authorization)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapInteractionSettings(settings), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-interaction-settings", Method: http.MethodPut, Path: "/v1/me/interaction-settings", Summary: "Update the viewer's interaction settings", Security: security}, func(ctx context.Context, input *interactionSettingsUpdateInput) (*interactionSettingsOutput, error) {
		settings, err := service.UpdateInteractionSettings(ctx, input.Authorization, input.IdempotencyKey, application.InteractionSettings{CommentsEnabled: input.Body.CommentsEnabled, ReactionsEnabled: input.Body.ReactionsEnabled})
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapInteractionSettings(settings), nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-practice-comments", Method: http.MethodGet, Path: "/v1/social/feed/{eventId}/comments", Summary: "List comments on a practice event", Security: security}, func(ctx context.Context, input *practiceCommentsInput) (*practiceCommentListOutput, error) {
		items, cursor, err := service.ListPracticeComments(ctx, input.Authorization, input.EventID, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &practiceCommentListOutput{}
		out.Body.Data.Items = make([]dto.PracticeCommentItem, 0, len(items))
		out.Body.Meta.NextCursor = cursor
		for _, item := range items {
			out.Body.Data.Items = append(out.Body.Data.Items, mapPracticeCommentItem(item))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "create-practice-comment", Method: http.MethodPost, Path: "/v1/social/feed/{eventId}/comments", Summary: "Comment on a practice event", DefaultStatus: http.StatusCreated, Security: security}, func(ctx context.Context, input *practiceCommentCreationInput) (*practiceCommentOutput, error) {
		comment, err := service.CreatePracticeComment(ctx, input.Authorization, input.EventID, input.Body.Text, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapPracticeCommentOutput(comment, http.StatusCreated), nil
	})
	huma.Register(api, huma.Operation{OperationID: "edit-practice-comment", Method: http.MethodPatch, Path: "/v1/social/feed/{eventId}/comments/{commentId}", Summary: "Edit the viewer's practice-event comment", Security: security}, func(ctx context.Context, input *practiceCommentEditInput) (*practiceCommentOutput, error) {
		comment, err := service.EditPracticeComment(ctx, input.Authorization, input.EventID, input.CommentID, input.Body.Text, input.Body.ExpectedVersion, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapPracticeCommentOutput(comment, http.StatusOK), nil
	})
	huma.Register(api, huma.Operation{OperationID: "delete-practice-comment", Method: http.MethodDelete, Path: "/v1/social/feed/{eventId}/comments/{commentId}", Summary: "Delete a practice-event comment", Security: security}, func(ctx context.Context, input *practiceCommentRemovalInput) (*shared.NoContentOutput, error) {
		if _, err := service.DeletePracticeComment(ctx, input.Authorization, input.EventID, input.CommentID, input.IdempotencyKey); err != nil {
			return nil, shared.MapError(err, true)
		}
		return &shared.NoContentOutput{Status: http.StatusNoContent}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-practice-comment-history", Method: http.MethodGet, Path: "/v1/social/feed/{eventId}/comments/{commentId}/history", Summary: "List immutable versions of a practice-event comment", Security: security}, func(ctx context.Context, input *commentHistoryInput) (*commentHistoryOutput, error) {
		versions, cursor, err := service.ListCommentHistory(ctx, input.Authorization, input.EventID, input.CommentID, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &commentHistoryOutput{}
		out.Body.Data.Versions = make([]dto.CommentVersion, 0, len(versions))
		out.Body.Meta.NextCursor = cursor
		for _, version := range versions {
			out.Body.Data.Versions = append(out.Body.Data.Versions, mapCommentVersion(version))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "set-practice-comment-heart", Method: http.MethodPut, Path: "/v1/social/feed/{eventId}/comments/{commentId}/heart", Summary: "Heart a practice-event comment", Security: security}, func(ctx context.Context, input *practiceCommentHeartInput) (*practiceCommentHeartOutput, error) {
		summary, err := service.SetPracticeCommentHeart(ctx, input.Authorization, input.EventID, input.CommentID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapPracticeCommentHeartOutput(summary), nil
	})
	huma.Register(api, huma.Operation{OperationID: "remove-practice-comment-heart", Method: http.MethodDelete, Path: "/v1/social/feed/{eventId}/comments/{commentId}/heart", Summary: "Remove the viewer's heart from a practice-event comment", Security: security}, func(ctx context.Context, input *practiceCommentHeartInput) (*practiceCommentHeartOutput, error) {
		summary, err := service.RemovePracticeCommentHeart(ctx, input.Authorization, input.EventID, input.CommentID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapPracticeCommentHeartOutput(summary), nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-practice-comment-hearts", Method: http.MethodGet, Path: "/v1/social/feed/{eventId}/comments/{commentId}/hearts", Summary: "List people who hearted a practice-event comment", Security: security}, func(ctx context.Context, input *practiceCommentHeartsInput) (*practiceCommentHeartRosterOutput, error) {
		profiles, cursor, err := service.ListPracticeCommentHearts(ctx, input.Authorization, input.EventID, input.CommentID, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &practiceCommentHeartRosterOutput{}
		out.Body.Data.Items = make([]dto.PublicProfile, 0, len(profiles))
		for _, profile := range profiles {
			out.Body.Data.Items = append(out.Body.Data.Items, mapProfile(profile))
		}
		out.Body.Meta.NextCursor = cursor
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "set-practice-reaction", Method: http.MethodPut, Path: "/v1/social/feed/{eventId}/reaction", Summary: "Set a reaction on a practice event", Security: security}, func(ctx context.Context, input *practiceReactionInput) (*practiceReactionOutput, error) {
		summary, err := service.SetPracticeReaction(ctx, input.Authorization, input.EventID, domain.Reaction(input.Body.Reaction), input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapPracticeReactionOutput(summary), nil
	})
	huma.Register(api, huma.Operation{OperationID: "remove-practice-reaction", Method: http.MethodDelete, Path: "/v1/social/feed/{eventId}/reaction", Summary: "Remove the viewer's reaction from a practice event", Security: security}, func(ctx context.Context, input *practiceReactionRemovalInput) (*practiceReactionOutput, error) {
		summary, err := service.RemovePracticeReaction(ctx, input.Authorization, input.EventID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapPracticeReactionOutput(summary), nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-active-following", Method: http.MethodGet, Path: "/v1/social/feed/active", Summary: "List followed profiles currently tracking time", Security: security}, func(ctx context.Context, input *activeFollowingInput) (*activeFollowingOutput, error) {
		items, cursor, err := service.ListActiveFollowing(ctx, input.Authorization, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &activeFollowingOutput{}
		out.Body.Data.Items = make([]dto.ActiveFollowingItem, 0, len(items))
		out.Body.Meta.NextCursor = cursor
		for _, item := range items {
			out.Body.Data.Items = append(out.Body.Data.Items, mapActiveFollowingItem(item))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-practice-feed", Method: http.MethodGet, Path: "/v1/social/feed", Summary: "List chronological feed events", Security: security}, func(ctx context.Context, input *practiceFeedInput) (*practiceFeedOutput, error) {
		items, cursor, err := service.ListPracticeFeed(ctx, input.Authorization, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &practiceFeedOutput{}
		out.Body.Data.Items = make([]dto.PracticeFeedItem, 0, len(items))
		out.Body.Meta.NextCursor = cursor
		for _, item := range items {
			out.Body.Data.Items = append(out.Body.Data.Items, mapPracticeFeedItem(item))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-practice-feed-event", Method: http.MethodGet, Path: "/v1/social/feed/{eventId}", Summary: "Get an accessible feed event", Security: security}, func(ctx context.Context, input *practiceFeedEventInput) (*practiceFeedEventOutput, error) {
		item, err := service.GetPracticeFeedEvent(ctx, input.Authorization, input.EventID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &practiceFeedEventOutput{}
		out.Body.Data = mapPracticeFeedItem(item)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "search-profiles", Method: http.MethodGet, Path: "/v1/profiles", Summary: "Search active discoverable profiles", Security: security}, func(ctx context.Context, input *searchInput) (*searchOutput, error) {
		profiles, cursor, err := service.Search(ctx, input.Authorization, input.Query, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &searchOutput{}
		out.Body.Data = make([]dto.PublicProfile, 0, len(profiles))
		out.Body.Meta.NextCursor = cursor
		for _, profile := range profiles {
			out.Body.Data = append(out.Body.Data, mapProfile(profile))
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-profile", Method: http.MethodGet, Path: "/v1/profiles/{username}", Summary: "Get a discoverable public profile", Security: security}, func(ctx context.Context, input *profileInput) (*profileOutput, error) {
		profile, err := service.Get(ctx, input.Authorization, input.Username)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &profileOutput{}
		out.Body.Data = mapProfile(profile)
		return out, nil
	})
	registerTargetMutation(api, security, "follow-profile", http.MethodPost, "/v1/profiles/{username}/follow", "Follow a profile or request access", func(ctx context.Context, input *profileMutationInput) (application.RelationshipResult, error) {
		return service.Follow(ctx, input.Authorization, input.Username, input.IdempotencyKey)
	})
	registerTargetMutation(api, security, "cancel-follow-request", http.MethodDelete, "/v1/profiles/{username}/follow-request", "Cancel an outgoing follow request", func(ctx context.Context, input *profileMutationInput) (application.RelationshipResult, error) {
		return service.CancelRequest(ctx, input.Authorization, input.Username, input.IdempotencyKey)
	})
	registerTargetMutation(api, security, "unfollow-profile", http.MethodDelete, "/v1/profiles/{username}/follow", "Stop following a profile", func(ctx context.Context, input *profileMutationInput) (application.RelationshipResult, error) {
		return service.Unfollow(ctx, input.Authorization, input.Username, input.IdempotencyKey)
	})
	huma.Register(api, huma.Operation{OperationID: "list-follow-requests", Method: http.MethodGet, Path: "/v1/follow-requests", Summary: "List incoming follow requests", Security: security}, func(ctx context.Context, input *followRequestsInput) (*followRequestsOutput, error) {
		requests, cursor, err := service.ListIncomingRequests(ctx, input.Authorization, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &followRequestsOutput{}
		out.Body.Data = make([]dto.FollowRequest, 0, len(requests))
		out.Body.Meta.NextCursor = cursor
		for _, request := range requests {
			out.Body.Data = append(out.Body.Data, mapFollowRequest(request))
		}
		return out, nil
	})
	registerRequestReview(api, security, "accept-follow-request", "/v1/follow-requests/{requestId}/accept", "Accept an incoming follow request", service.AcceptRequest)
	registerRequestReview(api, security, "reject-follow-request", "/v1/follow-requests/{requestId}/reject", "Reject an incoming follow request", service.RejectRequest)
	huma.Register(api, huma.Operation{OperationID: "review-profile-block", Method: http.MethodGet, Path: "/v1/profiles/{username}/block-review", Summary: "Review the effects of blocking a profile", Security: security}, func(ctx context.Context, input *blockReviewInput) (*blockReviewOutput, error) {
		review, err := service.ReviewBlock(ctx, input.Authorization, input.Username)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &blockReviewOutput{}
		out.Body.Data.Target = mapBlockTarget(review.Target)
		out.Body.Data.SharedPaths = make([]dto.BlockSharedPath, 0, len(review.SharedPaths))
		for _, path := range review.SharedPaths {
			out.Body.Data.SharedPaths = append(out.Body.Data.SharedPaths, dto.BlockSharedPath{ID: path.ID, Name: path.Name})
		}
		out.Body.Data.Acknowledgement = dto.BlockReviewAcknowledgement{Version: review.Acknowledgement.Version, Token: review.Acknowledgement.Token, ExpiresAt: review.Acknowledgement.ExpiresAt}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "block-profile", Method: http.MethodPost, Path: "/v1/profiles/{username}/block", Summary: "Block a profile", Security: security}, func(ctx context.Context, input *blockMutationInput) (*blockMutationOutput, error) {
		result, err := service.BlockUser(ctx, input.Authorization, input.Username, input.IdempotencyKey, application.BlockReviewAcknowledgement{Version: input.Body.Acknowledgement.Version, Token: input.Body.Acknowledgement.Token})
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapBlockMutation(result), nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-blocked-accounts", Method: http.MethodGet, Path: "/v1/blocked-accounts", Summary: "List accounts blocked by the viewer", Security: security}, func(ctx context.Context, input *blockedAccountsInput) (*blockedAccountsOutput, error) {
		accounts, cursor, err := service.ListBlockedAccounts(ctx, input.Authorization, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &blockedAccountsOutput{}
		out.Body.Data = make([]dto.BlockedAccount, 0, len(accounts))
		out.Body.Meta.NextCursor = cursor
		for _, account := range accounts {
			out.Body.Data = append(out.Body.Data, dto.BlockedAccount{BlockTarget: mapBlockTarget(account.Target), BlockedAt: account.BlockedAt})
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "unblock-account", Method: http.MethodDelete, Path: "/v1/blocked-accounts/{userId}", Summary: "Unblock an account", Security: security}, func(ctx context.Context, input *unblockMutationInput) (*blockMutationOutput, error) {
		result, err := service.UnblockUser(ctx, input.Authorization, input.UserID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapBlockMutation(result), nil
	})
}

func mapBlockTarget(target domain.BlockTarget) dto.BlockTarget {
	return dto.BlockTarget{UserID: target.UserID, Username: target.Username, DisplayName: target.DisplayName}
}

func mapBlockMutation(result application.BlockMutationResult) *blockMutationOutput {
	out := &blockMutationOutput{}
	out.Body.Data.Target = mapBlockTarget(result.Target)
	out.Body.Data.Blocked = result.Blocked
	return out
}

func mapInteractionSettings(settings application.InteractionSettings) *interactionSettingsOutput {
	out := &interactionSettingsOutput{}
	out.Body.Data = dto.InteractionSettings{CommentsEnabled: settings.CommentsEnabled, ReactionsEnabled: settings.ReactionsEnabled}
	return out
}

type targetMutation func(context.Context, *profileMutationInput) (application.RelationshipResult, error)

func registerTargetMutation(api huma.API, security []map[string][]string, operationID, method, path, summary string, mutate targetMutation) {
	huma.Register(api, huma.Operation{OperationID: operationID, Method: method, Path: path, Summary: summary, Security: security}, func(ctx context.Context, input *profileMutationInput) (*relationshipOutput, error) {
		result, err := mutate(ctx, input)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &relationshipOutput{}
		out.Body.Data.Profile = mapProfile(result.Target)
		out.Body.Data.RequestID = result.RequestID
		return out, nil
	})
}

type requestReview func(context.Context, string, string, string) (application.ReviewResult, error)

func registerRequestReview(api huma.API, security []map[string][]string, operationID, path, summary string, review requestReview) {
	huma.Register(api, huma.Operation{OperationID: operationID, Method: http.MethodPost, Path: path, Summary: summary, Security: security}, func(ctx context.Context, input *requestMutationInput) (*reviewOutput, error) {
		result, err := review(ctx, input.Authorization, input.RequestID, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &reviewOutput{}
		out.Body.Data.Request = mapFollowRequest(result.Request)
		out.Body.Data.Decision = string(result.Decision)
		return out, nil
	})
}

func mapProfile(profile domain.PublicProfile) dto.PublicProfile {
	return dto.PublicProfile{ID: profile.ID, Username: profile.Username, DisplayName: profile.DisplayName, ProfilePictureURL: profile.ProfilePictureURL, Description: profile.Description, FollowerCount: profile.FollowerCount, FollowingCount: profile.FollowingCount, Relationship: string(profile.Relationship)}
}

func mapFollowRequest(request domain.FollowRequest) dto.FollowRequest {
	return dto.FollowRequest{ID: request.ID, Requester: mapProfile(request.Requester), CreatedAt: request.CreatedAt}
}

func mapPracticeFeedItem(item application.PracticeFeedItem) dto.PracticeFeedItem {
	var picture *string
	if item.ProfilePictureURL != "" {
		value := item.ProfilePictureURL
		picture = &value
	}
	result := dto.PracticeFeedItem{
		ID: item.ID, Type: string(item.Type), PublishedAt: item.PublishedAt,
		Participant:      dto.PracticeFeedParticipant{UserID: item.ParticipantID, Username: item.Username, DisplayName: item.DisplayName, ProfilePictureURL: picture},
		Path:             dto.PracticeFeedPath{ID: item.PathID, Name: item.PathName},
		Reactions:        mapPracticeReactionCounts(item.Reactions.Counts),
		ViewerReaction:   mapViewerReaction(item.Reactions.ViewerReaction),
		CommentsEnabled:  item.CommentsEnabled,
		ReactionsEnabled: item.ReactionsEnabled,
	}
	if item.Type == application.FeedEventPracticeSession {
		result.Activity = &dto.PracticeFeedActivity{ID: item.ActivityID, DurationSeconds: item.DurationSeconds, Edited: item.Edited}
	}
	if item.Type == application.FeedEventGoalAchievement && item.Achievement != nil {
		result.Achievement = &dto.GoalAchievement{Kind: string(item.Achievement.Kind), TargetSeconds: item.Achievement.TargetSeconds}
		if item.Achievement.Kind == application.AchievementInterval {
			startedAt, endedAt := item.Achievement.IntervalStartedAt, item.Achievement.IntervalEndedAt
			result.Achievement.IntervalStartedAt, result.Achievement.IntervalEndedAt = &startedAt, &endedAt
		}
	}
	return result
}

func mapPracticeReactionOutput(summary domain.ReactionSummary) *practiceReactionOutput {
	out := &practiceReactionOutput{}
	out.Body.Data = dto.PracticeReactionSummary{Reactions: mapPracticeReactionCounts(summary.Counts), ViewerReaction: mapViewerReaction(summary.ViewerReaction)}
	return out
}

func mapPracticeComment(comment domain.Comment) dto.PracticeComment {
	return dto.PracticeComment{ID: comment.ID, EventID: comment.EventID, AuthorUserID: comment.AuthorID, Text: comment.Text, Version: comment.Version, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt, Edited: comment.Edited()}
}

func mapPracticeCommentItem(item application.PracticeCommentItem) dto.PracticeCommentItem {
	return dto.PracticeCommentItem{Comment: mapPracticeComment(item.Comment), Author: mapProfile(item.Author), HeartCount: item.HeartCount, HeartedByViewer: item.HeartedByViewer}
}

func mapPracticeCommentHeartOutput(summary application.CommentHeartSummary) *practiceCommentHeartOutput {
	out := &practiceCommentHeartOutput{}
	out.Body.Data = dto.CommentHeartSummary{CommentID: summary.CommentID, HeartCount: summary.HeartCount, HeartedByViewer: summary.HeartedByViewer}
	return out
}

func mapPracticeCommentOutput(comment domain.Comment, status int) *practiceCommentOutput {
	out := &practiceCommentOutput{Status: status}
	out.Body.Data = mapPracticeComment(comment)
	return out
}

func mapCommentVersion(version domain.CommentVersion) dto.CommentVersion {
	return dto.CommentVersion{CommentID: version.CommentID, Text: version.Text, Version: version.Version, CreatedAt: version.CreatedAt}
}

func mapPracticeReactionCounts(counts domain.ReactionCounts) dto.PracticeReactionCounts {
	return dto.PracticeReactionCounts{Heart: counts.Heart, Applause: counts.Applause, Fire: counts.Fire, Strong: counts.Strong, Celebrate: counts.Celebrate}
}

func mapViewerReaction(reaction domain.Reaction) *string {
	if reaction == "" {
		return nil
	}
	value := string(reaction)
	return &value
}

func mapActiveFollowingItem(item application.ActiveFollowingCandidate) dto.ActiveFollowingItem {
	var picture *string
	if item.ProfilePictureURL != "" {
		value := item.ProfilePictureURL
		picture = &value
	}
	out := dto.ActiveFollowingItem{
		Participant: dto.PracticeFeedParticipant{UserID: item.ParticipantID, Username: item.Username, DisplayName: item.DisplayName, ProfilePictureURL: picture},
		Timers:      make([]dto.ActiveFollowingTimer, 0, len(item.Timers)),
	}
	for _, timer := range item.Timers {
		out.Timers = append(out.Timers, dto.ActiveFollowingTimer{ID: timer.ID, Path: dto.PracticeFeedPath{ID: timer.PathID, Name: timer.PathName}, StartedAt: timer.StartedAt})
	}
	return out
}
