package httpserver

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

type onboardingProfileDTO struct {
	Email              string                 `json:"email"`
	DisplayName        string                 `json:"displayName"`
	UsernameSuggestion string                 `json:"usernameSuggestion"`
	PolicyReviewToken  string                 `json:"policyReviewToken"`
	Policies           onboardingPolicySetDTO `json:"policies"`
}

type onboardingPolicyReferenceDTO struct {
	Version string `json:"version"`
	URL     string `json:"url" format:"uri"`
}

type onboardingPolicySetDTO struct {
	TermsOfService      onboardingPolicyReferenceDTO `json:"termsOfService"`
	PrivacyPolicy       onboardingPolicyReferenceDTO `json:"privacyPolicy"`
	CommunityGuidelines onboardingPolicyReferenceDTO `json:"communityGuidelines"`
	SupportURL          string                       `json:"supportUrl" format:"uri"`
}

type onboardingProfileOutput struct {
	Body struct {
		Data onboardingProfileDTO `json:"data"`
	}
}

type onboardingActivationInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Username                    string `json:"username" required:"true" minLength:"3" maxLength:"64"`
		DisplayName                 string `json:"displayName" required:"true" minLength:"1"`
		ProfileVisibility           string `json:"profileVisibility" required:"true" enum:"public,private"`
		TimeZone                    string `json:"timeZone" required:"true" minLength:"1"`
		FirstDayOfWeek              int    `json:"firstDayOfWeek" required:"true" minimum:"1" maximum:"7"`
		AtLeast16                   bool   `json:"atLeast16" required:"true"`
		TermsAccepted               bool   `json:"termsAccepted" required:"true"`
		PrivacyAcknowledged         bool   `json:"privacyAcknowledged" required:"true"`
		CommunityGuidelinesAccepted bool   `json:"communityGuidelinesAccepted" required:"true"`
		PolicyReviewToken           string `json:"policyReviewToken" required:"true" minLength:"1" maxLength:"4096"`
	}
}

func registerOnboardingRoutes(api huma.API, application app.App) {
	huma.Register(api, huma.Operation{
		OperationID: "get-onboarding-profile",
		Method:      http.MethodGet,
		Path:        "/v1/onboarding",
		Summary:     "Get the authenticated provisional user's private onboarding profile seeds",
		Security:    []map[string][]string{{"oidc": {}}},
	}, func(ctx context.Context, input *MeInput) (*onboardingProfileOutput, error) {
		profile, err := application.OnboardingProfile(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &onboardingProfileOutput{}
		out.Body.Data = onboardingProfileDTO{
			Email: profile.Email, DisplayName: profile.DisplayName, UsernameSuggestion: profile.UsernameSuggestion, PolicyReviewToken: profile.PolicyReviewToken,
			Policies: onboardingPolicySetDTO{
				TermsOfService:      onboardingPolicyReferenceDTO{Version: profile.Policies.TermsOfService.Version, URL: profile.Policies.TermsOfService.URL},
				PrivacyPolicy:       onboardingPolicyReferenceDTO{Version: profile.Policies.PrivacyPolicy.Version, URL: profile.Policies.PrivacyPolicy.URL},
				CommunityGuidelines: onboardingPolicyReferenceDTO{Version: profile.Policies.CommunityGuidelines.Version, URL: profile.Policies.CommunityGuidelines.URL},
				SupportURL:          profile.Policies.SupportURL,
			},
		}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "activate-onboarding",
		Method:      http.MethodPost,
		Path:        "/v1/onboarding/activation",
		Summary:     "Complete onboarding and atomically issue the first active application session",
		Security:    []map[string][]string{{"oidc": {}}},
	}, func(ctx context.Context, input *onboardingActivationInput) (*SessionExchangeOutput, error) {
		session, err := application.ActivateOnboarding(ctx, input.Authorization, app.OnboardingActivationInput{
			Username: input.Body.Username, DisplayName: input.Body.DisplayName,
			ProfileVisibility: identity.ProfileVisibility(input.Body.ProfileVisibility), TimeZone: identity.IANATimeZone(input.Body.TimeZone), FirstDayOfWeek: identity.FirstDayOfWeek(input.Body.FirstDayOfWeek),
			AtLeast16: input.Body.AtLeast16, TermsAccepted: input.Body.TermsAccepted, PrivacyAcknowledged: input.Body.PrivacyAcknowledged, CommunityGuidelinesAccepted: input.Body.CommunityGuidelinesAccepted,
			PolicyReviewToken: input.Body.PolicyReviewToken,
		})
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &SessionExchangeOutput{}
		out.Body.Data = SessionExchangeData{Token: session.Token, ExpiresAt: session.ExpiresAt, NextAction: "home"}
		return out, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "decline-duplicate-email-recovery",
		Method:      http.MethodPost,
		Path:        "/v1/onboarding/duplicate-email-recovery/decline",
		Summary:     "Decline the duplicate-email recovery offer and continue provisional onboarding",
		Security:    []map[string][]string{{"oidc": {}}},
	}, func(ctx context.Context, input *MeInput) (*NoContentOutput, error) {
		if err := application.DeclineDuplicateEmailRecovery(ctx, input.Authorization); err != nil {
			return nil, mapError(err, false)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
}
