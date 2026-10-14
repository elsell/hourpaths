package liveactivityapp

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"time"
)

type Worker struct {
	Repository ports.LiveActivityRefreshRepository
	Provider   ports.LiveActivityProvider
	Clock      ports.Clock
	WorkerID   string
	NewID      func() string
}

var errRefreshProcessing = errors.New("live_activity_refresh_processing_failed")

func (w Worker) RunOnce(ctx context.Context) (int, error) {
	if w.Repository == nil || w.Provider == nil || w.Clock == nil || w.WorkerID == "" || w.NewID == nil {
		return 0, ports.ErrInvalidArgument
	}
	claims, err := w.Repository.ClaimLiveActivities(ctx, w.WorkerID, w.Clock.Now().UTC(), 30*time.Second, 25)
	if err != nil {
		return 0, errRefreshProcessing
	}
	for index, claim := range claims {
		now := w.Clock.Now().UTC().Truncate(time.Microsecond)
		err := w.Repository.HandoffLiveActivity(ctx, w.WorkerID, claim, now, func(eligible bool) ports.LiveActivityTransition {
			registration := claim.Registration
			event := audit.Event{ID: w.NewID(), OwnerUserID: registration.OwnerUserID, ActorUserID: registration.OwnerUserID,
				Action: audit.ResourceUpdated, TargetType: "live_activity_registration", TargetID: registration.ID,
				Outcome: audit.Succeeded, OccurredAt: now, CorrelationID: w.WorkerID}
			transition := ports.LiveActivityTransition{Audit: event, AvailableAt: registration.ExpiresAt}
			if !eligible || !registration.Valid(now) {
				transition.Retired = true
				transition.Audit.Action = audit.ResourceDeleted
				return transition
			}
			deadline := registration.NextDeadline(now)
			handoffCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			outcome, providerErr := w.Provider.Refresh(handoffCtx, ports.LiveActivityRefresh{OwnerUserID: registration.OwnerUserID, AccountLease: registration.AccountLease, DeviceToken: registration.Token, ExpiresAt: registration.ExpiresAt, StaleAt: deadline})
			if providerErr != nil {
				outcome = ports.LiveActivityRetry
			}
			transition.Outcome = outcome
			switch outcome {
			case ports.LiveActivityAccepted:
				transition.AvailableAt = deadline
			case ports.LiveActivityTokenInvalid:
				transition.Retired = true
				transition.Audit.Action = audit.ResourceDeleted
			default:
				transition.Audit.Outcome = audit.Failed
				delay := 30 * time.Second
				for attempt := 0; attempt < claim.Failures && delay < 5*time.Minute; attempt++ {
					delay *= 2
				}
				if delay > 5*time.Minute {
					delay = 5 * time.Minute
				}
				transition.AvailableAt = w.Clock.Now().UTC().Add(delay)
				if transition.AvailableAt.After(registration.ExpiresAt) {
					transition.AvailableAt = registration.ExpiresAt
				}
			}
			return transition
		})
		if err != nil {
			return index, errRefreshProcessing
		}
	}
	return len(claims), nil
}
