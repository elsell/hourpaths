package social

import (
	"context"
	"crypto/sha256"
	"errors"
	"strconv"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	SendNudgeOperation           = "social.nudge.send"
	UpdateNudgeAudienceOperation = "social.path_nudge_preference.update"
)

var (
	ErrNudgeGoalComplete   = errors.New("nudge recipient goal is complete")
	ErrNudgeAudienceDenied = errors.New("nudge recipient audience disallows sender")
	ErrNudgeAlreadySent    = errors.New("nudge already sent for current limit window")
)

type NudgeEligibilityReason string

const (
	NudgeGoalCompleteReason NudgeEligibilityReason = "goal_complete"
	NudgeRateLimitedReason  NudgeEligibilityReason = "rate_limited"
)

func (reason NudgeEligibilityReason) Valid() bool {
	return reason == NudgeGoalCompleteReason || reason == NudgeRateLimitedReason
}

type NudgeAudiencePreference struct {
	PathID, UserID string
	Audience       domain.NudgeAudience
	Revision       int64
	UpdatedAt      time.Time
}

func (preference NudgeAudiencePreference) valid() bool {
	if preference.PathID == "" || preference.UserID == "" || !preference.Audience.Valid() || preference.Revision < 0 {
		return false
	}
	if preference.Revision == 0 {
		return preference.Audience == domain.DefaultNudgeAudience && preference.UpdatedAt.IsZero()
	}
	return !preference.UpdatedAt.IsZero() && preference.UpdatedAt.Location() == time.UTC
}

type NudgeEligibility struct {
	PathID, RecipientUserID string
	Eligible                bool
	Reason                  NudgeEligibilityReason
}

func (eligibility NudgeEligibility) valid() bool {
	return eligibility.PathID != "" && eligibility.RecipientUserID != "" &&
		((eligibility.Eligible && eligibility.Reason == "") || (!eligibility.Eligible && eligibility.Reason.Valid()))
}

type NudgePreferenceCommand struct {
	ActorUserID      string
	PathID           string
	Audience         domain.NudgeAudience
	ExpectedRevision int64
	ChangedAt        time.Time
	Idempotency      ports.Idempotency
	Audit            audit.Event
}

type NudgeCommand struct {
	Nudge       domain.Nudge
	Idempotency ports.Idempotency
	Audit       audit.Event
}

type NudgeMutationResult struct {
	Nudge    domain.Nudge
	Replayed bool
}

type NudgeRepository interface {
	GetNudgeAudience(context.Context, string, string) (NudgeAudiencePreference, error)
	UpdateNudgeAudience(context.Context, NudgePreferenceCommand) (NudgeAudiencePreference, error)
	GetNudgeEligibility(context.Context, string, string, string, time.Time) (NudgeEligibility, error)
	SendNudge(context.Context, NudgeCommand) (NudgeMutationResult, error)
}

func (service *Service) GetNudgeAudience(ctx context.Context, authorization, pathID string) (NudgeAudiencePreference, error) {
	principal, now, err := service.nudgeReady(ctx, authorization, pathID, "", false)
	if err != nil {
		return NudgeAudiencePreference{}, err
	}
	preference, err := service.Nudges.GetNudgeAudience(ctx, principal.UserID, pathID)
	if err != nil {
		return NudgeAudiencePreference{}, service.nudgeError(ctx, principal.UserID, "nudge_preference", err, now)
	}
	if !preference.valid() || preference.PathID != pathID || preference.UserID != principal.UserID {
		return NudgeAudiencePreference{}, errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "nudge_preference", pathID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return NudgeAudiencePreference{}, err
	}
	return preference, nil
}

func (service *Service) UpdateNudgeAudience(ctx context.Context, authorization, pathID, key string, expectedRevision int64, audience domain.NudgeAudience) (NudgeAudiencePreference, error) {
	if !validRelationshipIdempotencyKey(key) || expectedRevision < 0 || !audience.Valid() {
		return NudgeAudiencePreference{}, ports.ErrInvalidArgument
	}
	principal, now, err := service.nudgeReady(ctx, authorization, pathID, "", false)
	if err != nil {
		return NudgeAudiencePreference{}, err
	}
	command := NudgePreferenceCommand{ActorUserID: principal.UserID, PathID: pathID, Audience: audience, ExpectedRevision: expectedRevision, ChangedAt: now}
	command.Idempotency = nudgePreferenceIdempotency(principal.UserID, key, pathID, expectedRevision, audience)
	command.Audit = shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceUpdated, "nudge_preference", pathID, audit.Succeeded)
	command.Audit.OccurredAt = now
	preference, err := service.Nudges.UpdateNudgeAudience(ctx, command)
	if err != nil {
		return NudgeAudiencePreference{}, service.nudgeError(ctx, principal.UserID, "nudge_preference", err, now)
	}
	if !preference.valid() || preference.PathID != pathID || preference.UserID != principal.UserID || preference.Audience != audience || preference.Revision != expectedRevision+1 {
		return NudgeAudiencePreference{}, errInvalidDependencies
	}
	return preference, nil
}

func (service *Service) GetNudgeEligibility(ctx context.Context, authorization, pathID, recipientUserID string) (NudgeEligibility, error) {
	principal, now, err := service.nudgeReady(ctx, authorization, pathID, recipientUserID, false)
	if err != nil {
		return NudgeEligibility{}, err
	}
	eligibility, err := service.Nudges.GetNudgeEligibility(ctx, principal.UserID, recipientUserID, pathID, now)
	if err != nil {
		return NudgeEligibility{}, service.nudgeError(ctx, principal.UserID, "nudge", err, now)
	}
	if !eligibility.valid() || eligibility.PathID != pathID || eligibility.RecipientUserID != recipientUserID {
		return NudgeEligibility{}, errInvalidDependencies
	}
	event := shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceViewed, "nudge_eligibility", pathID+":"+recipientUserID, audit.Succeeded)
	event.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, event); err != nil {
		return NudgeEligibility{}, err
	}
	return eligibility, nil
}

func (service *Service) SendNudge(ctx context.Context, authorization, pathID, recipientUserID string, content domain.NudgeContent, key string) (domain.Nudge, error) {
	if !content.Valid() || !validRelationshipIdempotencyKey(key) {
		return domain.Nudge{}, ports.ErrInvalidArgument
	}
	principal, now, err := service.nudgeReady(ctx, authorization, pathID, recipientUserID, true)
	if err != nil {
		return domain.Nudge{}, err
	}
	nudge := domain.Nudge{ID: service.NewID(), SenderID: principal.UserID, RecipientID: recipientUserID, PathID: pathID, Content: content, SentAt: now}
	if !nudge.Valid() {
		return domain.Nudge{}, errInvalidDependencies
	}
	command := NudgeCommand{Nudge: nudge, Idempotency: nudgeIdempotency(principal.UserID, key, pathID, recipientUserID, content)}
	command.Audit = shared.NewAuditEvent(ctx, service.Clock, principal.UserID, principal.UserID, audit.ResourceCreated, "nudge", nudge.ID, audit.Succeeded)
	command.Audit.OccurredAt = now
	result, err := service.Nudges.SendNudge(ctx, command)
	if err != nil {
		return domain.Nudge{}, service.nudgeError(ctx, principal.UserID, "nudge", err, now)
	}
	if !result.Nudge.Valid() || result.Nudge.SenderID != principal.UserID || result.Nudge.RecipientID != recipientUserID || result.Nudge.PathID != pathID || result.Nudge.Content != nudge.Content ||
		(!result.Replayed && result.Nudge != nudge) || (result.Replayed && result.Nudge.SentAt.After(now)) {
		return domain.Nudge{}, errInvalidDependencies
	}
	return result.Nudge, nil
}

func (service *Service) nudgeReady(ctx context.Context, authorization, pathID, recipient string, requireID bool) (ports.Principal, time.Time, error) {
	principal, err := service.authenticate(ctx, authorization)
	if err != nil {
		return ports.Principal{}, time.Time{}, err
	}
	if !validOpaqueID(pathID) || (recipient != "" && (recipient == principal.UserID || !validOpaqueID(recipient))) || service.Nudges == nil || service.Authorizer == nil || service.NudgeRateLimiter == nil || service.Audits == nil || service.Clock == nil || (requireID && service.NewID == nil) {
		if !validOpaqueID(pathID) || (recipient != "" && (recipient == principal.UserID || !validOpaqueID(recipient))) {
			return ports.Principal{}, time.Time{}, ports.ErrInvalidArgument
		}
		return ports.Principal{}, time.Time{}, errInvalidDependencies
	}
	now := service.Clock.Now().UTC().Truncate(time.Microsecond)
	if now.IsZero() {
		return ports.Principal{}, time.Time{}, errInvalidDependencies
	}
	target := "nudge:" + pathID
	if recipient != "" {
		target += ":" + recipient
	}
	if !service.NudgeRateLimiter.Allow(principal.UserID, target, now) {
		return ports.Principal{}, time.Time{}, platformapp.ErrRateLimited
	}
	allowed, err := service.Authorizer.Check(ctx, "path", pathID, "view", principal.UserID)
	if err != nil {
		return ports.Principal{}, time.Time{}, err
	}
	if !allowed {
		return ports.Principal{}, time.Time{}, service.nudgeError(ctx, principal.UserID, "nudge", ports.ErrNotFound, now)
	}
	return principal, now, nil
}

func (service *Service) nudgeError(ctx context.Context, actor, targetType string, cause error, now time.Time) error {
	switch {
	case errors.Is(cause, ErrNudgeGoalComplete):
		return ports.ErrConflict
	case errors.Is(cause, ErrNudgeAlreadySent):
		return platformapp.ErrRateLimited
	case !errors.Is(cause, ports.ErrNotFound) && !errors.Is(cause, ErrNudgeAudienceDenied):
		return cause
	}
	denial := shared.NewAuditEvent(ctx, service.Clock, actor, actor, audit.ResourceAccessDenied, targetType, "hidden", audit.Denied)
	denial.OccurredAt = now
	if err := service.Audits.AppendAuditEvent(ctx, denial); err != nil {
		return err
	}
	return ports.ErrNotFound
}

func nudgePreferenceIdempotency(actor, key, pathID string, expectedRevision int64, audience domain.NudgeAudience) ports.Idempotency {
	digest := sha256.Sum256([]byte(UpdateNudgeAudienceOperation + "\x00" + pathID + "\x00" + strconv.FormatInt(expectedRevision, 10) + "\x00" + string(audience)))
	return ports.Idempotency{PrincipalID: actor, Operation: UpdateNudgeAudienceOperation, Key: key, RequestHash: digest[:]}
}

func nudgeIdempotency(actor, key, pathID, recipient string, content domain.NudgeContent) ports.Idempotency {
	digest := sha256.Sum256([]byte(SendNudgeOperation + "\x00" + pathID + "\x00" + recipient + "\x00" + string(content.Kind) + "\x00" + string(content.Preset)))
	return ports.Idempotency{PrincipalID: actor, Operation: SendNudgeOperation, Key: key, RequestHash: digest[:]}
}
