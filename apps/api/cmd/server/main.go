package main

import (
	"context"
	"fmt"
	"github.com/danielgtaylor/huma/v2"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore"
	pathstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/path"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	pathroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/routes"
	socialroutes "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/social/routes"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/observability"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/oidcauth"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/ratelimit"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/sessionauth"
	spice "github.com/elsell/hour-paths/apps/api/internal/adapters/spicedb"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/config"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/generated"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

type socialRelationshipLimiter struct{ limiter *ratelimit.Limiter }

func (limiter socialRelationshipLimiter) Allow(actor, target string, now time.Time) bool {
	return limiter.limiter != nil && limiter.limiter.Allow(socialRelationshipRateLimitKey(actor, target), now)
}

func socialRelationshipRateLimitKey(actor, target string) string {
	return fmt.Sprintf("%d:%s%d:%s", len(actor), actor, len(target), target)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration invalid", "error", err)
		os.Exit(1)
	}
	store, err := gormstore.Open("postgres", cfg.DatabaseDSN)
	if err != nil {
		panic(err)
	}
	if err := store.ConfigurePool(cfg.DatabaseMaxOpenConns, cfg.DatabaseMaxIdleConns, time.Duration(cfg.DatabaseConnMaxLifetime)*time.Second, time.Duration(cfg.DatabaseConnMaxIdleTime)*time.Second); err != nil {
		panic(err)
	}
	verifier, err := oidcauth.New(ctx, cfg.OIDCIssuer, cfg.OIDCBackchannelURL, cfg.OIDCAudiences, cfg.OIDCInsecure)
	if err != nil {
		panic(err)
	}
	authorizationURL, tokenURL := verifier.Endpoints()
	discoveryURL := cfg.OIDCIssuer
	if cfg.OIDCBackchannelURL != "" {
		discoveryURL = cfg.OIDCBackchannelURL
	}
	authorizer, err := spice.New(cfg.SpiceDBEndpoint, cfg.SpiceDBToken, cfg.SpiceDBInsecure)
	if err != nil {
		panic(err)
	}
	clock := systemClock{}
	auditLimiter, err := ratelimit.New(store.DB, "audit", cfg.AuditEventsPerMinute, time.Minute, cfg.AuditLimiterPrincipals)
	if err != nil {
		panic(err)
	}
	requestLimiter, err := ratelimit.New(store.DB, "request", cfg.RequestsPerMinute, time.Minute, cfg.RequestLimiterSources)
	if err != nil {
		panic(err)
	}
	pushInstallations, pushWorker, err := newPushRuntime(store.DB, cfg, clock)
	if err != nil {
		panic(err)
	}
	sessions := sessionauth.New(store, clock)
	metrics := observability.NewMetrics(cfg.MetricsBearerToken)
	probe := observability.Fanout{observability.StructuredLog{Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil))}, metrics}
	if cfg.OTLPHTTPEndpoint != "" {
		telemetry, err := observability.NewOpenTelemetry(ctx, cfg.OTLPHTTPEndpoint, cfg.OTelServiceName, cfg.OTLPInsecure)
		if err != nil {
			panic(err)
		}
		probe = append(probe, telemetry)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = telemetry.Shutdown(shutdownCtx)
		}()
	}
	invitedEmails := make(map[string]struct{}, len(cfg.AccountInvitedEmails))
	for _, email := range cfg.AccountInvitedEmails {
		invitedEmails[email] = struct{}{}
	}
	invitationAdmins := make(map[string]struct{}, len(cfg.InvitationAdminIdentities))
	invitationAdminUsers := make(map[string]struct{}, len(cfg.InvitationAdminIdentities))
	for _, configured := range cfg.InvitationAdminIdentities {
		invitationAdmins[configured] = struct{}{}
		separator := strings.LastIndex(configured, "#")
		invitationAdminUsers[identity.UserID(configured[:separator], configured[separator+1:])] = struct{}{}
	}
	application := app.App{Auth: sessions, IdentityVerifier: verifier, Sessions: sessions, OnboardingActivator: sessions, PolicyAuthority: store, SessionTTL: time.Duration(cfg.SessionTTLMinutes) * time.Minute, SessionAbsoluteTTL: time.Duration(cfg.SessionAbsoluteTTLMinutes) * time.Minute, AuthorizationMaxAttempts: cfg.AuthorizationMaxAttempts, AllowAccountProvisioning: cfg.AccountProvisioningMode == "open", InvitedEmails: invitedEmails, InvitationAdmins: invitationAdmins, InvitationAdminUsers: invitationAdminUsers, Invitations: store, PushInstallations: pushInstallations, AllowAccountDeactivation: cfg.AccountSelfDeactivationEnabled, Users: store, TimeZonePreferences: store, DuplicateAccountHints: store, DuplicateAccountRecoveryDeclines: store, UsernameSuggestions: store, Authorizer: authorizer, Resources: store, Audits: store, AuditRateLimiter: auditLimiter, AuthorizationOutbox: store, AuthorizationBatchOutbox: store, AuthorizationSerializer: store, RelationshipWriter: authorizer, Clock: clock, Dependencies: []ports.HealthChecker{store, gormstore.PolicyAuthorityHealth{Authority: store}, authorizer}, CursorSigningKey: []byte(cfg.CursorSigningKey), Probe: probe}
	authorizationWorker := uuid.NewString()
	go reconcile(ctx, application, authorizationWorker)
	go reconcilePush(ctx, pushWorker)
	registrations := generated.Registrations(generated.Dependencies{DB: store.DB, Auth: sessions, Profiles: store, Authorizer: authorizer, AuthorizationOutbox: store, AuthorizationSerializer: store, Audits: store, AuditRateLimiter: auditLimiter, Clock: clock, Probe: probe, NewID: uuid.NewString, AuthorizationWorker: uuid.NewString(), AuthorizationLease: 30 * time.Second, CursorSigningKey: []byte(cfg.CursorSigningKey)})
	socialRelationships := gormstore.NewSocialRelationshipRepository(store.DB, uuid.NewString, authorizationWorker, 30*time.Second)
	socialFeed := gormstore.NewSocialFeedRepository(store.DB)
	socialService := socialapp.New(socialapp.Dependencies{
		Auth: sessions, Profiles: gormstore.NewSocialProfileRepository(store.DB), Feed: socialFeed, ActiveFollowing: socialFeed, Relationships: socialRelationships,
		Blocks:    socialRelationships,
		Reactions: socialFeed, ReactionRateLimiter: socialRelationshipLimiter{limiter: auditLimiter},
		Comments: socialFeed, CommentRateLimiter: socialRelationshipLimiter{limiter: auditLimiter},
		InteractionSettings: socialFeed,
		Nudges:              socialFeed, NudgeRateLimiter: socialRelationshipLimiter{limiter: auditLimiter},
		NudgeNotificationChannels: gormstore.NewNudgeNotificationChannelRepository(store.DB),
		Authorizer:                authorizer, AuthorizationOutbox: store, AuthorizationStatus: socialRelationships,
		AuthorizationSerializer: store, AuthorizationWorker: authorizationWorker, AuthorizationLease: 30 * time.Second,
		Audits: store, AuditRateLimiter: auditLimiter, RelationshipRateLimiter: socialRelationshipLimiter{limiter: auditLimiter},
		Clock: clock, NewID: uuid.NewString, CursorSigningKey: []byte(cfg.CursorSigningKey),
	})
	registrations = append(registrations, func(api huma.API) { socialroutes.Register(api, socialService) })
	transferService := pathapp.NewOwnershipTransferService(pathapp.OwnershipTransferDependencies{
		Auth: sessions, Paths: pathstore.New(store.DB), Profiles: store, Authorizer: authorizer,
		AuthorizationReconciler: application, AuthorizationWorker: authorizationWorker,
		Transfers: pathstore.NewOwnershipTransferRepository(store.DB), Audits: store,
		AuditRateLimiter: auditLimiter, Clock: clock, NewID: uuid.NewString,
		Lifetime: cfg.OwnershipTransferExpiration, CursorSigningKey: []byte(cfg.CursorSigningKey),
	})
	registrations = append(registrations, func(api huma.API) { pathroutes.RegisterOwnershipTransfers(api, transferService) })
	handler, _ := httpserver.New(application, generated.Domains, httpserver.Options{OIDCIssuer: cfg.OIDCIssuer, OIDCDiscoveryURL: discoveryURL, OIDCAuthorizationURL: authorizationURL, OIDCTokenURL: tokenURL, OIDCDocsClientID: cfg.OIDCDocsClientID, OIDCDocsRedirectURI: cfg.OIDCDocsRedirectURI, PublicBaseURL: cfg.PublicBaseURL, CORSAllowedOrigins: cfg.CORSAllowedOrigins, TrustedProxyCIDRs: cfg.TrustedProxyCIDRs, DisableDocs: cfg.ScalarDocsDisabled, RequestRateLimiter: requestLimiter, Clock: clock, Probe: probe, MetricsHandler: metrics, DomainRegistrations: registrations})
	server := &http.Server{Addr: cfg.HTTPAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
func reconcile(ctx context.Context, application app.App, worker string) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := application.ReconcileAuthorization(ctx, worker, 25); err != nil {
			slog.Error("authorization reconciliation failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
