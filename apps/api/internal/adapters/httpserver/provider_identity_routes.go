package httpserver

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
)

type providerIdentityInput struct {
	Authorization string `header:"Authorization"`
}
type linkedProviderDTO struct {
	Provider  string `json:"provider" enum:"google,apple"`
	CanUnlink bool   `json:"canUnlink"`
}
type providerIdentityOutput struct {
	Body struct {
		Data []linkedProviderDTO `json:"data"`
	}
}
type beginIdentityLinkInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Provider string `json:"provider" enum:"google,apple"`
	}
}
type identityLinkChallengeDTO struct {
	ID        string    `json:"id"`
	Provider  string    `json:"provider" enum:"google,apple"`
	Nonce     string    `json:"nonce"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type beginIdentityLinkOutput struct {
	Body struct {
		Data identityLinkChallengeDTO `json:"data"`
	}
}
type finishIdentityLinkInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		ChallengeID   string `json:"challengeId" minLength:"1" maxLength:"64"`
		IdentityToken string `json:"identityToken" minLength:"1" maxLength:"32768"`
	}
}
type unlinkIdentityInput struct {
	Authorization string `header:"Authorization"`
	Provider      string `path:"provider" enum:"google,apple"`
	Body          struct {
		ReviewedUserID string `json:"reviewedUserId" minLength:"1" maxLength:"128"`
	}
}

func registerProviderIdentityRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "list-linked-providers", Method: http.MethodGet, Path: "/v1/me/identities", Summary: "List own linked sign-in providers", Security: security}, func(ctx context.Context, input *providerIdentityInput) (*providerIdentityOutput, error) {
		providers, err := application.LinkedProviders(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		output := &providerIdentityOutput{}
		output.Body.Data = make([]linkedProviderDTO, 0, len(providers))
		for _, provider := range providers {
			output.Body.Data = append(output.Body.Data, linkedProviderDTO{Provider: string(provider), CanUnlink: len(providers) > 1})
		}
		return output, nil
	})
	huma.Register(api, huma.Operation{OperationID: "begin-identity-link", Method: http.MethodPost, Path: "/v1/me/identities/link", Summary: "Begin account-bound provider authentication", Security: security}, func(ctx context.Context, input *beginIdentityLinkInput) (*beginIdentityLinkOutput, error) {
		challenge, err := application.BeginIdentityLink(ctx, input.Authorization, identity.Provider(input.Body.Provider))
		if err != nil {
			return nil, mapError(err, false)
		}
		output := &beginIdentityLinkOutput{}
		output.Body.Data.ID = challenge.ID
		output.Body.Data.Provider = string(challenge.Provider)
		output.Body.Data.Nonce = challenge.Nonce
		output.Body.Data.ExpiresAt = challenge.ExpiresAt
		return output, nil
	})
	huma.Register(api, huma.Operation{OperationID: "finish-identity-link", Method: http.MethodPost, Path: "/v1/me/identities/link/complete", Summary: "Link the verified provider without replacing the current session", Security: security}, func(ctx context.Context, input *finishIdentityLinkInput) (*NoContentOutput, error) {
		if err := application.FinishIdentityLink(ctx, input.Authorization, input.Body.ChallengeID, input.Body.IdentityToken); err != nil {
			return nil, mapError(err, false)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
	huma.Register(api, huma.Operation{OperationID: "unlink-provider", Method: http.MethodDelete, Path: "/v1/me/identities/{provider}", Summary: "Unlink a provider while retaining another sign-in method", Security: security}, func(ctx context.Context, input *unlinkIdentityInput) (*NoContentOutput, error) {
		if err := application.UnlinkIdentity(ctx, input.Authorization, input.Body.ReviewedUserID, identity.Provider(input.Provider)); err != nil {
			return nil, mapError(err, false)
		}
		return &NoContentOutput{Status: http.StatusNoContent}, nil
	})
}
