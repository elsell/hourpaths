package routes

import "context"

// Route contract tests use an already accepted policy context; global admission
// has dedicated adversarial HTTP and real-session PostgreSQL coverage.
type acceptedPolicyFixture struct{}

func (acceptedPolicyFixture) AdmitPolicyUse(context.Context, string) error { return nil }
