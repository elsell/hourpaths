package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
	"time"
)

type policyRenewalReviewDTO struct {
	Required    bool                   `json:"required"`
	ReviewToken string                 `json:"reviewToken"`
	Policies    onboardingPolicySetDTO `json:"policies"`
}
type policyRenewalReviewOutput struct {
	Body struct {
		Data policyRenewalReviewDTO `json:"data"`
	}
}
type policyRenewalInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           struct {
		ReviewToken                 string `json:"reviewToken" required:"true" minLength:"1" maxLength:"4096"`
		TermsAccepted               bool   `json:"termsAccepted" required:"true"`
		PrivacyAcknowledged         bool   `json:"privacyAcknowledged" required:"true"`
		CommunityGuidelinesAccepted bool   `json:"communityGuidelinesAccepted" required:"true"`
	}
}
type policyRenewalDTO struct {
	TermsVersion               string    `json:"termsVersion"`
	PrivacyPolicyVersion       string    `json:"privacyPolicyVersion"`
	CommunityGuidelinesVersion string    `json:"communityGuidelinesVersion"`
	AcceptedAt                 time.Time `json:"acceptedAt"`
}
type policyRenewalOutput struct {
	Body struct {
		Data policyRenewalDTO `json:"data"`
	}
}

func registerPolicyRenewalRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "review-current-policies", Method: http.MethodGet, Path: "/v1/me/policies", Summary: "Review policies and acceptance required for the current account", Security: security}, func(ctx context.Context, in *MeInput) (*policyRenewalReviewOutput, error) {
		review, err := application.ReviewCurrentPolicies(ctx, in.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &policyRenewalReviewOutput{}
		out.Body.Data.Required = review.Required
		out.Body.Data.ReviewToken = review.ReviewToken
		p := review.Policies
		out.Body.Data.Policies = onboardingPolicySetDTO{TermsOfService: onboardingPolicyReferenceDTO{Version: p.TermsOfService.Version, URL: p.TermsOfService.URL}, PrivacyPolicy: onboardingPolicyReferenceDTO{Version: p.PrivacyPolicy.Version, URL: p.PrivacyPolicy.URL}, CommunityGuidelines: onboardingPolicyReferenceDTO{Version: p.CommunityGuidelines.Version, URL: p.CommunityGuidelines.URL}, SupportURL: p.SupportURL}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "accept-current-policies", Method: http.MethodPost, Path: "/v1/me/policies", Summary: "Accept the reviewed policy versions for the current account", Security: security}, func(ctx context.Context, in *policyRenewalInput) (*policyRenewalOutput, error) {
		result, err := application.AcceptCurrentPolicies(ctx, in.Authorization, in.IdempotencyKey, app.PolicyRenewalInput{ReviewToken: in.Body.ReviewToken, TermsAccepted: in.Body.TermsAccepted, PrivacyAcknowledged: in.Body.PrivacyAcknowledged, GuidelinesAccepted: in.Body.CommunityGuidelinesAccepted})
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &policyRenewalOutput{}
		out.Body.Data.TermsVersion = result.Policies.TermsOfService
		out.Body.Data.PrivacyPolicyVersion = result.Policies.PrivacyPolicy
		out.Body.Data.CommunityGuidelinesVersion = result.Policies.CommunityGuidelines
		out.Body.Data.AcceptedAt = result.AcceptedAt
		return out, nil
	})
}
