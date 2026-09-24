package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type ProfilePage struct {
	Profiles []domain.PublicProfile
	HasMore  bool
	LastRank int
}

type ProfileRepository interface {
	Search(context.Context, string, string, int, string, int) (ProfilePage, error)
	GetByUsername(context.Context, string, string) (domain.PublicProfile, error)
}

type Dependencies struct {
	Auth                      ports.Authenticator
	Profiles                  ProfileRepository
	Feed                      FeedRepository
	ActiveFollowing           ActiveFollowingRepository
	Relationships             RelationshipRepository
	Blocks                    BlockRepository
	Authorizer                ports.Authorizer
	AuthorizationOutbox       ports.AuthorizationOutbox
	AuthorizationStatus       AuthorizationChangeStatusReader
	AuthorizationSerializer   ports.AuthorizationSerializer
	Audits                    ports.Audits
	AuditRateLimiter          ports.AuditRateLimiter
	RelationshipRateLimiter   RelationshipRateLimiter
	ReactionRateLimiter       RelationshipRateLimiter
	Reactions                 ReactionRepository
	CommentRateLimiter        RelationshipRateLimiter
	Comments                  CommentRepository
	InteractionSettings       InteractionSettingsRepository
	Nudges                    NudgeRepository
	NudgeNotificationChannels NudgeNotificationChannelRepository
	NudgeRateLimiter          RelationshipRateLimiter
	Clock                     ports.Clock
	NewID                     func() string
	AuthorizationWorker       string
	AuthorizationLease        time.Duration
	CursorSigningKey          []byte
}

type Service struct{ Dependencies }

var errInvalidDependencies = errors.New("social profile service dependencies are invalid")

func New(dependencies Dependencies) *Service { return &Service{Dependencies: dependencies} }

func (service *Service) authenticate(ctx context.Context, authorization string) (ports.Principal, error) {
	if service.Auth == nil {
		return ports.Principal{}, errInvalidDependencies
	}
	principal, err := service.Auth.Authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, err
	}
	if principal.UserID == "" || len(principal.Scopes) != 1 || principal.Scopes[0] != "api:user" {
		return ports.Principal{}, platformapp.ErrUnauthenticated
	}
	return principal, nil
}

func (service *Service) ready(principal string) error {
	if service.Profiles == nil || service.Audits == nil || service.AuditRateLimiter == nil || service.Clock == nil || len(service.CursorSigningKey) < 32 {
		return errInvalidDependencies
	}
	if !service.AuditRateLimiter.Allow(principal, service.Clock.Now().UTC()) {
		return platformapp.ErrRateLimited
	}
	return nil
}

func (service *Service) Search(ctx context.Context, authorization, rawQuery, cursor string, limit int) ([]domain.PublicProfile, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	query, err := domain.NormalizeProfileQuery(rawQuery)
	if err != nil || limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if err := service.ready(principal.UserID); err != nil {
		return nil, "", err
	}
	after, afterRank := "", -1
	if cursor != "" {
		claims, decodeErr := decodeCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || claims.Viewer != principal.UserID || claims.Query != query {
			return nil, "", ports.ErrInvalidArgument
		}
		after = claims.AfterUsername
		afterRank = claims.AfterRank
	}
	page, err := service.Profiles.Search(ctx, principal.UserID, query, afterRank, after, limit)
	if err != nil {
		return nil, "", err
	}
	for _, profile := range page.Profiles {
		if !profile.Valid() {
			return nil, "", errInvalidDependencies
		}
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "profile", "search", audit.Succeeded)
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return nil, "", err
	}
	next := ""
	if page.HasMore {
		if len(page.Profiles) == 0 {
			return nil, "", errInvalidDependencies
		}
		if page.LastRank < 0 || page.LastRank > 3 {
			return nil, "", errInvalidDependencies
		}
		next, err = encodeCursor(service.CursorSigningKey, cursorClaims{Version: 1, Viewer: principal.UserID, Query: query, AfterRank: page.LastRank, AfterUsername: strings.ToLower(page.Profiles[len(page.Profiles)-1].Username)})
		if err != nil {
			return nil, "", err
		}
	}
	return page.Profiles, next, nil
}

func (service *Service) Get(ctx context.Context, authorization, rawUsername string) (domain.PublicProfile, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return domain.PublicProfile{}, err
	}
	username, err := domain.NormalizeUsername(rawUsername)
	if err != nil {
		return domain.PublicProfile{}, ports.ErrInvalidArgument
	}
	if err := service.ready(principal.UserID); err != nil {
		return domain.PublicProfile{}, err
	}
	profile, err := service.Profiles.GetByUsername(ctx, principal.UserID, username)
	if err != nil {
		if errors.Is(err, ports.ErrNotFound) {
			denial := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "profile", "hidden", audit.Denied)
			if auditErr := service.Audits.AppendAuditEvent(ctx, denial); auditErr != nil {
				return domain.PublicProfile{}, auditErr
			}
		}
		return domain.PublicProfile{}, err
	}
	if !profile.Valid() {
		return domain.PublicProfile{}, errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "profile", profile.ID, audit.Succeeded)
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return domain.PublicProfile{}, err
	}
	return profile, nil
}

type cursorClaims struct {
	Version       int    `json:"v"`
	Viewer        string `json:"viewer"`
	Query         string `json:"query"`
	AfterRank     int    `json:"afterRank"`
	AfterUsername string `json:"afterUsername"`
}

func encodeCursor(key []byte, claims cursorClaims) (string, error) {
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(append(body, mac.Sum(nil)...)), nil
}

func decodeCursor(key []byte, value string) (cursorClaims, error) {
	var claims cursorClaims
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) <= sha256.Size || len(value) > 4096 {
		return claims, ports.ErrInvalidArgument
	}
	body, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(body, &claims) != nil || claims.Version != 1 || claims.Viewer == "" || claims.Query == "" || claims.AfterRank < 0 || claims.AfterRank > 3 || claims.AfterUsername == "" {
		return cursorClaims{}, ports.ErrInvalidArgument
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !hmac.Equal(body, canonical) {
		return cursorClaims{}, ports.ErrInvalidArgument
	}
	return claims, nil
}
