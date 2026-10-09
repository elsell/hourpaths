package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

type ConnectionDirection string

const (
	Followers ConnectionDirection = "followers"
	Following ConnectionDirection = "following"
)

func (d ConnectionDirection) Valid() bool { return d == Followers || d == Following }

type ConnectionPage struct {
	Profiles    []domain.PublicProfile
	HasMore     bool
	LastCreated time.Time
	LastID      string
}
type ConnectionRepository interface {
	ListConnections(context.Context, string, string, ConnectionDirection, ports.PageRequest) (ConnectionPage, error)
	RemoveFollower(context.Context, RelationshipCommand) (RelationshipResult, error)
}

func (service *Service) RemoveFollower(ctx context.Context, authorization, userID, key string) (RelationshipResult, error) {
	return service.mutateTarget(ctx, authorization, userID, key, RemoveFollowerOperation)
}

func (service *Service) ListConnections(ctx context.Context, authorization, rawUsername string, direction ConnectionDirection, cursor string, limit int) ([]domain.PublicProfile, string, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	username, err := domain.NormalizeUsername(rawUsername)
	if err != nil || !direction.Valid() || limit < 1 || limit > 100 {
		return nil, "", ports.ErrInvalidArgument
	}
	if err = service.ready(principal.UserID); err != nil {
		return nil, "", err
	}
	repo, ok := service.Relationships.(ConnectionRepository)
	if !ok || service.Authorizer == nil {
		return nil, "", errInvalidDependencies
	}
	profile, err := service.Profiles.GetByUsername(ctx, principal.UserID, username)
	if err != nil {
		return nil, "", service.relationshipError(ctx, principal.UserID, err, service.Clock.Now().UTC())
	}
	if !profile.Valid() {
		return nil, "", errInvalidDependencies
	}
	if profile.ID != principal.UserID {
		allowed, checkErr := service.Authorizer.Check(ctx, "user", profile.ID, "follower", principal.UserID)
		if checkErr != nil {
			return nil, "", checkErr
		}
		if !allowed {
			return nil, "", service.relationshipError(ctx, principal.UserID, ports.ErrNotFound, service.Clock.Now().UTC())
		}
	}
	pageRequest := ports.PageRequest{Snapshot: service.Clock.Now().UTC().Truncate(time.Microsecond), Limit: limit}
	if cursor != "" {
		claims, decodeErr := decodeConnectionCursor(service.CursorSigningKey, cursor)
		if decodeErr != nil || claims.Viewer != principal.UserID || claims.Owner != profile.ID || claims.Direction != direction {
			return nil, "", ports.ErrInvalidArgument
		}
		pageRequest.Snapshot, pageRequest.AfterCreated, pageRequest.AfterID = claims.Snapshot, claims.AfterCreated, claims.AfterID
	}
	page, err := repo.ListConnections(ctx, principal.UserID, profile.ID, direction, pageRequest)
	if err != nil {
		return nil, "", service.relationshipError(ctx, principal.UserID, err, service.Clock.Now().UTC())
	}
	if len(page.Profiles) > limit {
		return nil, "", errInvalidDependencies
	}
	seen := map[string]bool{}
	for _, p := range page.Profiles {
		if !p.Valid() || seen[p.ID] {
			return nil, "", errInvalidDependencies
		}
		seen[p.ID] = true
	}
	next := ""
	if page.HasMore {
		if len(page.Profiles) == 0 || page.LastID != page.Profiles[len(page.Profiles)-1].ID || page.LastCreated.IsZero() || page.LastCreated.After(pageRequest.Snapshot) {
			return nil, "", errInvalidDependencies
		}
		next, err = encodeConnectionCursor(service.CursorSigningKey, connectionCursor{Version: 1, Viewer: principal.UserID, Owner: profile.ID, Direction: direction, Snapshot: pageRequest.Snapshot, AfterCreated: page.LastCreated, AfterID: page.LastID})
		if err != nil {
			return nil, "", err
		}
	}
	if err = service.Audits.AppendAuditEvent(ctx, shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceListed, "profile_"+string(direction), profile.ID, audit.Succeeded)); err != nil {
		return nil, "", err
	}
	return page.Profiles, next, nil
}

type connectionCursor struct {
	Version      int                 `json:"v"`
	Viewer       string              `json:"viewer"`
	Owner        string              `json:"owner"`
	Direction    ConnectionDirection `json:"direction"`
	Snapshot     time.Time           `json:"snapshot"`
	AfterCreated time.Time           `json:"afterCreated"`
	AfterID      string              `json:"afterId"`
}

func encodeConnectionCursor(key []byte, c connectionCursor) (string, error) {
	body, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(append(body, mac.Sum(nil)...)), nil
}
func decodeConnectionCursor(key []byte, value string) (connectionCursor, error) {
	var c connectionCursor
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) <= sha256.Size || len(value) > 4096 {
		return c, ports.ErrInvalidArgument
	}
	body, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) || json.Unmarshal(body, &c) != nil || c.Version != 1 || !validOpaqueID(c.Viewer) || !validOpaqueID(c.Owner) || !c.Direction.Valid() || c.Snapshot.IsZero() || c.Snapshot.Location() != time.UTC || c.AfterCreated.IsZero() || c.AfterCreated.Location() != time.UTC || c.AfterCreated.After(c.Snapshot) || !validOpaqueID(c.AfterID) {
		return connectionCursor{}, ports.ErrInvalidArgument
	}
	canonical, err := json.Marshal(c)
	if err != nil || !hmac.Equal(body, canonical) {
		return connectionCursor{}, ports.ErrInvalidArgument
	}
	return c, nil
}
