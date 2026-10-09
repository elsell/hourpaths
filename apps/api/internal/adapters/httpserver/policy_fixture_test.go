package httpserver

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
)

// Existing route suites exercise their own domain boundaries with current
// policy acceptance. The policy-renewal suite exercises real admission.
type acceptedPolicyFixture struct{}

func (acceptedPolicyFixture) AdmitPolicyUse(context.Context, string) error { return nil }
func newHTTPTestServer(a app.App, domains []string, options Options) (http.Handler, huma.API) {
	if options.PolicyAdmission == nil {
		options.PolicyAdmission = acceptedPolicyFixture{}
	}
	return New(a, domains, options)
}
