package ports

import (
	"context"
	"time"
)

// LiveActivityRefresh is a wake signal. It deliberately cannot carry timer
// state, action targets, Path names, or remote tracking commands.
type LiveActivityRefresh struct {
	OwnerUserID, AccountLease, DeviceToken string
	ExpiresAt, StaleAt                     time.Time
}

type LiveActivityDelivery string

const (
	LiveActivityAccepted             LiveActivityDelivery = "accepted"
	LiveActivityRetry                LiveActivityDelivery = "retry"
	LiveActivityTokenInvalid         LiveActivityDelivery = "token_invalid"
	LiveActivityConfigurationFailure LiveActivityDelivery = "configuration_failure"
)

type LiveActivityProvider interface {
	Refresh(context.Context, LiveActivityRefresh) (LiveActivityDelivery, error)
}
