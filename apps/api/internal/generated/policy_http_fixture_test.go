package generated_test

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"net/http"
)

// These domain-boundary fixtures represent accounts with current acceptance.
type acceptedPolicyHTTPFixture struct{}

func (acceptedPolicyHTTPFixture) AdmitPolicyUse(context.Context, string) error { return nil }
func newPolicyAcceptedHTTPTestServer(a app.App, domains []string, options httpserver.Options) (http.Handler, huma.API) {
	options.PolicyAdmission = acceptedPolicyHTTPFixture{}
	return httpserver.New(a, domains, options)
}
