package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type OwnProfilePrivacy struct {
	UserID     string
	Visibility identity.ProfileVisibility
	Revision   int64
}

type ProfilePrivacyUpdate struct {
	Visibility       identity.ProfileVisibility
	ExpectedRevision int64
	Confirmed        bool
}

type ProfilePrivacyCommand struct {
	ActorUserID         string
	Update              ProfilePrivacyUpdate
	ChangedAt           time.Time
	Idempotency         ports.Idempotency
	Audit               audit.Event
	NewID               func() string
	AuthorizationWorker string
	AuthorizationLease  time.Duration
}

type ProfilePrivacyResult struct {
	Profile OwnProfilePrivacy
	// Keep the affected resources in the replay so a lost response can finish
	// durable authorization work without repeating the privacy mutation.
	AffectedPathIDs []string
}

type ProfilePrivacyRepository interface {
	GetOwnProfilePrivacy(context.Context, string) (OwnProfilePrivacy, error)
	UpdateOwnProfilePrivacy(context.Context, ProfilePrivacyCommand) (ProfilePrivacyResult, error)
}

func (a App) OwnProfilePrivacy(ctx context.Context, authorization string) (OwnProfilePrivacy, error) {
	u, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return OwnProfilePrivacy{}, err
	}
	if a.ProfilePrivacy == nil || a.Audits == nil {
		return OwnProfilePrivacy{}, ports.ErrUnavailable
	}
	profile, err := a.ProfilePrivacy.GetOwnProfilePrivacy(ctx, u.ID)
	if err != nil {
		return OwnProfilePrivacy{}, err
	}
	if !validOwnProfilePrivacy(profile, u.ID) {
		return OwnProfilePrivacy{}, ports.ErrUnavailable
	}
	if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, u.ID, u.ID, audit.UserViewed, "user", u.ID, audit.Succeeded)); err != nil {
		return OwnProfilePrivacy{}, err
	}
	return profile, nil
}

func (a App) UpdateOwnProfilePrivacy(ctx context.Context, authorization, key string, input ProfilePrivacyUpdate) (OwnProfilePrivacy, error) {
	u, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return OwnProfilePrivacy{}, err
	}
	if !input.Confirmed || input.ExpectedRevision < 1 || identity.ValidateProfileVisibility(input.Visibility) != nil || !validTimeZoneMutationKey(key) {
		return OwnProfilePrivacy{}, ports.ErrInvalidArgument
	}
	if a.ProfilePrivacy == nil || a.Authorizer == nil || a.AuthorizationOutbox == nil || a.AuthorizationSerializer == nil || a.Audits == nil || a.Clock == nil {
		return OwnProfilePrivacy{}, ports.ErrUnavailable
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return OwnProfilePrivacy{}, ports.ErrInvalidArgument
	}
	hash := sha256.Sum256(encoded)
	now := a.Clock.Now().UTC().Truncate(time.Microsecond)
	worker := newID()
	event := a.auditEvent(ctx, u.ID, u.ID, audit.ResourceUpdated, "user", u.ID, audit.Succeeded)
	event.OccurredAt = now
	result, err := a.ProfilePrivacy.UpdateOwnProfilePrivacy(ctx, ProfilePrivacyCommand{
		ActorUserID: u.ID, Update: input, ChangedAt: now, Audit: event, NewID: newID,
		AuthorizationWorker: worker, AuthorizationLease: authorizationLease,
		Idempotency: ports.Idempotency{PrincipalID: u.ID, Operation: "account.profile.privacy", Key: key, RequestHash: hash[:]},
	})
	if err != nil {
		return OwnProfilePrivacy{}, err
	}
	if !validOwnProfilePrivacy(result.Profile, u.ID) || result.Profile.Visibility != input.Visibility || result.Profile.Revision != input.ExpectedRevision+1 {
		return OwnProfilePrivacy{}, ports.ErrUnavailable
	}
	for _, pathID := range result.AffectedPathIDs {
		if pathID == "" {
			return OwnProfilePrivacy{}, ports.ErrUnavailable
		}
		for {
			change, err := a.AuthorizationOutbox.ClaimAuthorizationChangeForResource(ctx, "path", pathID, worker, authorizationLease)
			if errors.Is(err, ports.ErrNotFound) {
				break
			}
			if err != nil {
				return OwnProfilePrivacy{}, err
			}
			if change.ResourceType != "path" || change.ResourceID != pathID {
				return OwnProfilePrivacy{}, ports.ErrUnavailable
			}
			if err := a.reconcileClaimedAuthorizationChange(ctx, change, worker); err != nil {
				return OwnProfilePrivacy{}, err
			}
		}
	}
	return result.Profile, nil
}

func validOwnProfilePrivacy(profile OwnProfilePrivacy, owner string) bool {
	return owner != "" && profile.UserID == owner && profile.Revision > 0 && identity.ValidateProfileVisibility(profile.Visibility) == nil
}
