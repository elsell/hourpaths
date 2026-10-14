package ports

import (
	"context"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/liveactivity"
	"time"
)

type LiveActivityClaim struct {
	Registration liveactivity.Registration
	Version      int64
	Failures     int
}

type LiveActivityTransition struct {
	Outcome     LiveActivityDelivery
	AvailableAt time.Time
	Retired     bool
	Audit       audit.Event
}

type LiveActivityRegistrationRepository interface {
	UpsertLiveActivity(context.Context, liveactivity.Registration, audit.Event) error
	RevokeLiveActivity(context.Context, string, string, string, audit.Event) error
}

type LiveActivityRefreshRepository interface {
	ClaimLiveActivities(context.Context, string, time.Time, time.Duration, int) ([]LiveActivityClaim, error)
	// Implementations serialize the final eligibility check and callback against
	// token replacement/revocation, and atomically persist its transition + audit.
	HandoffLiveActivity(context.Context, string, LiveActivityClaim, time.Time, func(bool) LiveActivityTransition) error
}
