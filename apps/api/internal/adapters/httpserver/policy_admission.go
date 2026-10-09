package httpserver

import (
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
)

func policyReviewExempt(operation string) bool {
	switch operation {
	// Session lifecycle and minimal identity remain available so clients can
	// review policies, switch accounts, or explicitly sign out.
	case "exchange-session", "revoke-session", "refresh-session", "get-me", "delete-push-installation",
		"get-onboarding-profile", "activate-onboarding", "decline-duplicate-email-recovery", "begin-account-recovery", "complete-account-recovery",
		"review-current-policies", "accept-current-policies", "delete-account", "confirm-account-deletion-receipt", "deactivate-me",
		"export-own-activities", "export-own-profile", "export-own-paths", "check-notification-ownership", "list-own-running-timers", "get-path-timer", "stop-path-timer":
		return true
	default:
		return false
	}
}
func installPolicyAdmission(api huma.API, admission app.PolicyAdmission) {
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		operation := ctx.Operation()
		if len(operation.Security) == 0 || policyReviewExempt(operation.OperationID) {
			next(ctx)
			return
		}
		if err := admission.AdmitPolicyUse(ctx.Context(), ctx.Header("Authorization")); err != nil {
			failure := mapError(err, false)
			status := http.StatusInternalServerError
			if classified, ok := failure.(huma.StatusError); ok {
				status = classified.GetStatus()
			}
			ctx.SetHeader("Content-Type", "application/problem+json")
			ctx.SetStatus(status)
			_ = json.NewEncoder(ctx.BodyWriter()).Encode(failure)
			return
		}
		next(ctx)
	})
}
