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

type nudgePreferenceInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId" minLength:"1" maxLength:"128"`
}

type nudgePreferenceUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId" minLength:"1" maxLength:"128"`
	Body           dto.NudgeAudiencePreferenceInput
}

type nudgePreferenceOutput struct {
	Body struct {
		Data dto.NudgeAudiencePreference `json:"data"`
	}
}

type nudgeEligibilityInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId" minLength:"1" maxLength:"128"`
	UserID        string `path:"userId" minLength:"1" maxLength:"128"`
}

type nudgeEligibilityOutput struct {
	Body struct {
		Data dto.NudgeEligibility `json:"data"`
	}
}

type nudgeCreationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId" minLength:"1" maxLength:"128"`
	UserID         string `path:"userId" minLength:"1" maxLength:"128"`
	Body           dto.NudgeInput
}

type nudgeOutput struct {
	Status int
	Body   struct {
		Data dto.Nudge `json:"data"`
	}
}

type nudgeNotificationChannelInput struct {
	Authorization string `header:"Authorization"`
}

type nudgeNotificationChannelUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           dto.NudgeNotificationChannelPreferenceInput
}

type nudgeNotificationChannelOutput struct {
	Body struct {
		Data dto.NudgeNotificationChannelPreference `json:"data"`
	}
}

func registerNudgeRoutes(api huma.API, service Service, security []map[string][]string) {
	huma.Register(api, huma.Operation{OperationID: "get-nudge-notification-channel", Method: http.MethodGet, Path: "/v1/me/notification-channels/nudges", Summary: "Get the viewer's nudge notification channel", Security: security}, func(ctx context.Context, input *nudgeNotificationChannelInput) (*nudgeNotificationChannelOutput, error) {
		preference, err := service.GetNudgeNotificationChannel(ctx, input.Authorization)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapNudgeNotificationChannel(preference), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-nudge-notification-channel", Method: http.MethodPut, Path: "/v1/me/notification-channels/nudges", Summary: "Update the viewer's nudge notification channel", Security: security}, func(ctx context.Context, input *nudgeNotificationChannelUpdateInput) (*nudgeNotificationChannelOutput, error) {
		preference, err := service.UpdateNudgeNotificationChannel(ctx, input.Authorization, input.IdempotencyKey, input.Body.ExpectedRevision, input.Body.Enabled)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapNudgeNotificationChannel(preference), nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-path-nudge-preference", Method: http.MethodGet, Path: "/v1/paths/{pathId}/nudge-preference", Summary: "Get the viewer's nudge audience for a path", Security: security}, func(ctx context.Context, input *nudgePreferenceInput) (*nudgePreferenceOutput, error) {
		preference, err := service.GetNudgeAudience(ctx, input.Authorization, input.PathID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapNudgePreference(preference), nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-path-nudge-preference", Method: http.MethodPut, Path: "/v1/paths/{pathId}/nudge-preference", Summary: "Update the viewer's nudge audience for a path", Security: security}, func(ctx context.Context, input *nudgePreferenceUpdateInput) (*nudgePreferenceOutput, error) {
		preference, err := service.UpdateNudgeAudience(ctx, input.Authorization, input.PathID, input.IdempotencyKey, input.Body.ExpectedRevision, domain.NudgeAudience(input.Body.Audience))
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapNudgePreference(preference), nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-path-member-nudge-eligibility", Method: http.MethodGet, Path: "/v1/paths/{pathId}/members/{userId}/nudge-eligibility", Summary: "Check whether the viewer may nudge a path participant", Security: security}, func(ctx context.Context, input *nudgeEligibilityInput) (*nudgeEligibilityOutput, error) {
		eligibility, err := service.GetNudgeEligibility(ctx, input.Authorization, input.PathID, input.UserID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapNudgeEligibility(eligibility), nil
	})
	huma.Register(api, huma.Operation{OperationID: "send-path-member-nudge", Method: http.MethodPost, Path: "/v1/paths/{pathId}/members/{userId}/nudges", Summary: "Send a preset nudge to a path participant", DefaultStatus: http.StatusCreated, Security: security}, func(ctx context.Context, input *nudgeCreationInput) (*nudgeOutput, error) {
		content := domain.NudgeContent{Kind: domain.NudgeContentKind(input.Body.Content.Kind), Preset: domain.NudgePreset(input.Body.Content.Preset)}
		nudge, err := service.SendNudge(ctx, input.Authorization, input.PathID, input.UserID, content, input.IdempotencyKey)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		return mapNudge(nudge), nil
	})
}

func mapNudgeNotificationChannel(preference application.NudgeNotificationChannelPreference) *nudgeNotificationChannelOutput {
	out := &nudgeNotificationChannelOutput{}
	out.Body.Data = dto.NudgeNotificationChannelPreference{Channel: application.NudgeNotificationChannel, Enabled: preference.Enabled, Revision: preference.Revision}
	return out
}

func mapNudgePreference(preference application.NudgeAudiencePreference) *nudgePreferenceOutput {
	out := &nudgePreferenceOutput{}
	out.Body.Data = dto.NudgeAudiencePreference{PathID: preference.PathID, UserID: preference.UserID, Audience: string(preference.Audience), Revision: preference.Revision}
	return out
}

func mapNudgeEligibility(eligibility application.NudgeEligibility) *nudgeEligibilityOutput {
	out := &nudgeEligibilityOutput{}
	out.Body.Data = dto.NudgeEligibility{PathID: eligibility.PathID, RecipientUserID: eligibility.RecipientUserID, Eligible: eligibility.Eligible, Reason: string(eligibility.Reason)}
	return out
}

func mapNudge(nudge domain.Nudge) *nudgeOutput {
	out := &nudgeOutput{Status: http.StatusCreated}
	out.Body.Data = dto.Nudge{
		ID: nudge.ID, SenderUserID: nudge.SenderID, RecipientUserID: nudge.RecipientID, PathID: nudge.PathID,
		Content: dto.NudgeContent{Kind: string(nudge.Content.Kind), Preset: string(nudge.Content.Preset)}, SentAt: nudge.SentAt,
	}
	return out
}
