package main

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	routes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/moderation"
	app "github.com/elsell/hour-paths/apps/api/internal/app/moderation"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
)

func moderationRegistration(store *gormstore.Store, auth ports.Authenticator, authorizer ports.Authorizer, clock ports.Clock, limiter ports.AuditRateLimiter, cursorKey []byte) func(huma.API) {
	service := app.New(app.Dependencies{Auth: auth, Repository: gormstore.NewModerationRepository(store.DB), Authorizer: authorizer, Audits: store, Clock: clock, RateLimiter: limiter, NewID: uuid.NewString})
	enforcements := app.NewEnforcements(app.EnforcementDependencies{Auth: auth, Repository: gormstore.NewModerationRepository(store.DB), Audits: store, Clock: clock, RateLimiter: limiter, CursorSigningKey: cursorKey})
	return func(api huma.API) { routes.Register(api, service); routes.RegisterEnforcements(api, enforcements) }
}
