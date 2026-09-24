package ports

import (
	"context"
	"fmt"
)

type PushPresentation string

const (
	PushActionable    PushPresentation = "actionable"
	PushInformational PushPresentation = "informational"
)

type PushMessage struct {
	DeviceToken    string
	Title          string
	Body           string
	NotificationID string
	Presentation   PushPresentation
}

type PushReceiptID string

type PushDeliveryState string

const (
	PushAccepted  PushDeliveryState = "accepted"
	PushPending   PushDeliveryState = "pending"
	PushDelivered PushDeliveryState = "delivered"
	PushFailed    PushDeliveryState = "failed"
)

type PushFailureDisposition string

const (
	PushRetryable PushFailureDisposition = "retryable"
	PushPermanent PushFailureDisposition = "permanent"
)

type PushFailure struct {
	Code          string
	Disposition   PushFailureDisposition
	DisableDevice bool
}

type PushTicket struct {
	ReceiptID PushReceiptID
	State     PushDeliveryState
	Failure   *PushFailure
}

type PushReceipt struct {
	ReceiptID PushReceiptID
	State     PushDeliveryState
	Failure   *PushFailure
}

type PushProvider interface {
	Send(context.Context, PushMessage) (PushTicket, error)
	Receipts(context.Context, []PushReceiptID) (map[PushReceiptID]PushReceipt, error)
}

// PushProviderError represents a request-level failure for which no individual
// ticket or receipt can be trusted. Its message intentionally excludes device
// tokens, notification copy, provider response bodies, and credentials.
type PushProviderError struct {
	Failure PushFailure
}

func (e *PushProviderError) Error() string {
	if e == nil {
		return "push provider failure"
	}
	return fmt.Sprintf("push provider failure: %s (%s)", e.Failure.Code, e.Failure.Disposition)
}
