package ports

import (
	"context"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
)

type PushInstallation struct {
	ID, OwnerUserID, Provider, Platform, Locale, Token string
	CreatedAt, UpdatedAt                               time.Time
}

type PushDelivery struct {
	NotificationID, InstallationID, RecipientUserID string
	Provider, Platform, Locale, Token               string
	Attempts                                        int
	CreatedAt, LockedUntil                          time.Time
	LockedBy                                        string
	ProviderTicket                                  string
}

type PushDeliveryOutcome string

const (
	PushDeliveryDelivered         PushDeliveryOutcome = "delivered"
	PushDeliveryAwaitingReceipt   PushDeliveryOutcome = "awaiting_receipt"
	PushDeliveryRetry             PushDeliveryOutcome = "retry"
	PushDeliverySuppressed        PushDeliveryOutcome = "suppressed"
	PushDeliveryPermanentlyFailed PushDeliveryOutcome = "permanently_failed"
)

type PushDeliveryTransition struct {
	NotificationID, InstallationID string
	Outcome                        PushDeliveryOutcome
	OccurredAt, AvailableAt        time.Time
	ProviderTicket, FailureCode    string
}

type PushRepository interface {
	UpsertPushInstallation(context.Context, PushInstallation, audit.Event) error
	DeletePushInstallation(context.Context, string, string, time.Time, audit.Event) error
	ClaimPushDeliveries(context.Context, string, time.Duration, int) ([]PushDelivery, error)
	PushDeliveryEligible(context.Context, string, string, string) (bool, error)
	HandoffPushDelivery(context.Context, string, string, string, func() PushTicket) (PushTicket, bool, error)
	TransitionPushDelivery(context.Context, string, PushDeliveryTransition, audit.Event) error
	DisablePushInstallation(context.Context, string, string, string, time.Time, string, audit.Event) error
}
