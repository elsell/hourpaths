package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type OwnProfile struct {
	UserID   string
	Text     identity.ProfileText
	Revision int64
}

type ProfileUpdate struct {
	Text             identity.ProfileText
	ExpectedRevision int64
}

type ProfileUpdateCommand struct {
	ActorUserID string
	Update      ProfileUpdate
	ChangedAt   time.Time
	Idempotency ports.Idempotency
	Audit       audit.Event
}

type ProfileRepository interface {
	GetOwnProfile(context.Context, string) (OwnProfile, error)
	UpdateOwnProfile(context.Context, ProfileUpdateCommand) (OwnProfile, error)
}

func (a App) OwnProfile(ctx context.Context, authorization string) (OwnProfile, error) {
	u, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return OwnProfile{}, err
	}
	if a.Profiles == nil || a.Audits == nil {
		return OwnProfile{}, ports.ErrUnavailable
	}
	profile, err := a.Profiles.GetOwnProfile(ctx, u.ID)
	if err != nil {
		return OwnProfile{}, err
	}
	if !validOwnProfile(profile, u.ID) {
		return OwnProfile{}, ports.ErrUnavailable
	}
	if err := a.Audits.AppendAuditEvent(ctx, a.auditEvent(ctx, u.ID, u.ID, audit.UserViewed, "user", u.ID, audit.Succeeded)); err != nil {
		return OwnProfile{}, err
	}
	return profile, nil
}

func (a App) UpdateOwnProfile(ctx context.Context, authorization, key string, input ProfileUpdate) (OwnProfile, error) {
	u, err := a.currentUserForAuditedOperation(ctx, authorization)
	if err != nil {
		return OwnProfile{}, err
	}
	if a.Profiles == nil {
		return OwnProfile{}, ports.ErrUnavailable
	}
	text, err := identity.NormalizeProfileText(input.Text)
	if err != nil || input.ExpectedRevision < 1 || !validTimeZoneMutationKey(key) {
		return OwnProfile{}, ports.ErrInvalidArgument
	}
	input.Text = text
	encoded, err := json.Marshal(input)
	if err != nil {
		return OwnProfile{}, ports.ErrInvalidArgument
	}
	hash := sha256.Sum256(encoded)
	changedAt := a.Clock.Now().UTC().Truncate(time.Microsecond)
	event := a.auditEvent(ctx, u.ID, u.ID, audit.ResourceUpdated, "user", u.ID, audit.Succeeded)
	event.OccurredAt = changedAt
	profile, err := a.Profiles.UpdateOwnProfile(ctx, ProfileUpdateCommand{
		ActorUserID: u.ID, Update: input, ChangedAt: changedAt, Audit: event,
		Idempotency: ports.Idempotency{PrincipalID: u.ID, Operation: "account.profile.update", Key: key, RequestHash: hash[:]},
	})
	if err != nil {
		return OwnProfile{}, err
	}
	if !validOwnProfile(profile, u.ID) || profile.Text != text {
		return OwnProfile{}, ports.ErrUnavailable
	}
	return profile, nil
}

func validOwnProfile(profile OwnProfile, owner string) bool {
	text, err := identity.NormalizeProfileText(profile.Text)
	return err == nil && text == profile.Text && owner != "" && profile.UserID == owner && profile.Revision > 0
}
