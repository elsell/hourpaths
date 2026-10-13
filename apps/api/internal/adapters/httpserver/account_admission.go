package httpserver

import (
	"encoding/json"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
)

func accountRestrictionExempt(operation string) bool {
	switch operation {
	case "exchange-session", "revoke-session", "refresh-session", "get-me", "delete-push-installation",
		"get-onboarding-profile", "activate-onboarding", "decline-duplicate-email-recovery", "begin-account-recovery", "complete-account-recovery",
		"review-current-policies", "accept-current-policies", "delete-account", "confirm-account-deletion-receipt", "deactivate-me",
		"export-own-activities", "export-own-profile", "export-own-paths",
		"get-account-restriction", "list-enforcement-notices", "get-enforcement-notice", "submit-enforcement-appeal":
		return true
	default:
		return false
	}
}

// Exempt operations still authenticate and authorize in their own application
// services. Restriction admission supplements, and never replaces, those checks.
func installAccountAdmission(api huma.API, admission app.AccountAdmission) {
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		operation := ctx.Operation()
		if len(operation.Security) == 0 || accountRestrictionExempt(operation.OperationID) {
			next(ctx)
			return
		}
		if err := admission.AdmitAccountUse(ctx.Context(), ctx.Header("Authorization")); err != nil {
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
