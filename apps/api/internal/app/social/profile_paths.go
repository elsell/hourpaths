package social

import (
	"context"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

type ProfilePathCandidate struct {
	ID    string
	Timer *ActiveFollowingTimer
}
type ProfilePathsRepository interface {
	ListProfilePathCandidates(context.Context, string, string, time.Time) ([]ProfilePathCandidate, error)
}
type ProfilePaths struct {
	Count  int
	Active ActiveFollowingCandidate
}

func (service *Service) GetProfilePaths(ctx context.Context, authorization, rawUsername string) (ProfilePaths, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return ProfilePaths{}, err
	}
	username, err := domain.NormalizeUsername(rawUsername)
	if err != nil {
		return ProfilePaths{}, ports.ErrInvalidArgument
	}
	if err = service.ready(principal.UserID); err != nil {
		return ProfilePaths{}, err
	}
	repository, ok := service.Feed.(ProfilePathsRepository)
	if !ok || service.Authorizer == nil {
		return ProfilePaths{}, errInvalidDependencies
	}
	profile, err := service.Profiles.GetByUsername(ctx, principal.UserID, username)
	if err != nil {
		if err == ports.ErrNotFound {
			if auditErr := service.Audits.AppendAuditEvent(ctx, shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceAccessDenied, "profile", "hidden", audit.Denied)); auditErr != nil {
				return ProfilePaths{}, auditErr
			}
		}
		return ProfilePaths{}, err
	}
	if !profile.Valid() {
		return ProfilePaths{}, errInvalidDependencies
	}
	candidates, err := repository.ListProfilePathCandidates(ctx, principal.UserID, profile.ID, service.Clock.Now().UTC())
	if err != nil {
		return ProfilePaths{}, err
	}
	result := ProfilePaths{Active: ActiveFollowingCandidate{ParticipantID: profile.ID, Username: profile.Username, DisplayName: profile.DisplayName, ProfilePictureURL: profile.ProfilePictureURL, Timers: []ActiveFollowingTimer{}}}
	for _, path := range candidates {
		allowed, err := service.Authorizer.Check(ctx, "path", path.ID, "view", principal.UserID)
		if err != nil {
			return ProfilePaths{}, err
		}
		if !allowed {
			continue
		}
		result.Count++
		if path.Timer != nil {
			result.Active.Timers = append(result.Active.Timers, *path.Timer)
		}
	}
	if err := service.Audits.AppendAuditEvent(ctx, shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "profile", profile.ID, audit.Succeeded)); err != nil {
		return ProfilePaths{}, err
	}
	return result, nil
}
