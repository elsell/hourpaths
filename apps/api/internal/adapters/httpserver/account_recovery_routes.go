package httpserver

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
)

func registerAccountRecoveryRoutes(api huma.API, application app.App) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "begin-account-recovery", Method: http.MethodPost, Path: "/v1/onboarding/duplicate-email-recovery", Summary: "Begin explicit dual-identity recovery for an unfinished enrollment", Security: security}, func(ctx context.Context, input *MeInput) (*beginIdentityLinkOutput, error) {
		challenge, err := application.BeginAccountRecovery(ctx, input.Authorization)
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &beginIdentityLinkOutput{}
		out.Body.Data = identityLinkChallengeDTO{ID: challenge.ID, Provider: string(challenge.Provider), Nonce: challenge.Nonce, ExpiresAt: challenge.ExpiresAt}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "complete-account-recovery", Method: http.MethodPost, Path: "/v1/onboarding/duplicate-email-recovery/complete", Summary: "Complete explicit recovery and atomically enter the retained account", Security: security}, func(ctx context.Context, input *finishIdentityLinkInput) (*SessionExchangeOutput, error) {
		session, err := application.FinishAccountRecovery(ctx, input.Authorization, input.Body.ChallengeID, input.Body.IdentityToken)
		if err != nil {
			return nil, mapError(err, false)
		}
		out := &SessionExchangeOutput{}
		out.Body.Data = SessionExchangeData{Token: session.Token, ExpiresAt: session.ExpiresAt, NextAction: "home"}
		return out, nil
	})
}
