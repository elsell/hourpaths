package main

import (
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	attentionroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/adminattention"
	routes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/support"
	attention "github.com/elsell/hour-paths/apps/api/internal/app/adminattention"
	app "github.com/elsell/hour-paths/apps/api/internal/app/support"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func supportRegistration(store *gormstore.Store, auth ports.Authenticator, authorizer ports.Authorizer, clock ports.Clock, limiter ports.AuditRateLimiter) func(huma.API) {
	s := &app.Service{Repository: gormstore.SupportRepository{DB: store.DB}, Auth: auth, Authorizer: authorizer, Clock: clock, Limiter: limiter}
	return func(api huma.API) {
		routes.Register(api, s)
		attentionroutes.Register(api, &attention.Service{Administrator: s, Repository: gormstore.AdminAttentionRepository{DB: store.DB}, Clock: clock})
	}
}
