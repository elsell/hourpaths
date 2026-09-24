package path

import (
	"errors"
	"testing"
	"time"
)

func TestNewOwnershipTransferStoresAnAbsoluteExpirationWithoutChangingRoles(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(7 * 24 * time.Hour)

	transfer, err := NewOwnershipTransfer(
		"transfer-1",
		"path-1",
		"creator-1",
		"recipient-1",
		createdAt,
		expiresAt,
	)
	if err != nil {
		t.Fatalf("NewOwnershipTransfer() error = %v", err)
	}
	if transfer.ID != "transfer-1" || transfer.PathID != "path-1" {
		t.Fatalf("NewOwnershipTransfer() identity = (%q, %q), want transfer-1 and path-1", transfer.ID, transfer.PathID)
	}
	if transfer.InitiatorUserID != "creator-1" || transfer.RecipientUserID != "recipient-1" {
		t.Fatalf("NewOwnershipTransfer() actors = (%q, %q), want creator-1 and recipient-1", transfer.InitiatorUserID, transfer.RecipientUserID)
	}
	if transfer.CreatedAt != createdAt || transfer.ExpiresAt != expiresAt {
		t.Fatalf("NewOwnershipTransfer() times = (%v, %v), want (%v, %v)", transfer.CreatedAt, transfer.ExpiresAt, createdAt, expiresAt)
	}
	if !transfer.Pending(createdAt) {
		t.Fatal("new transfer is not pending at its creation instant")
	}
}

func TestNewOwnershipTransferRejectsInvalidIdentityAndTimeFields(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(time.Hour)
	tests := []struct {
		name      string
		id        OwnershipTransferID
		pathID    ID
		initiator string
		recipient string
		createdAt time.Time
		expiresAt time.Time
	}{
		{name: "missing transfer id", pathID: "path-1", initiator: "creator-1", recipient: "recipient-1", createdAt: createdAt, expiresAt: expiresAt},
		{name: "blank transfer id", id: "  ", pathID: "path-1", initiator: "creator-1", recipient: "recipient-1", createdAt: createdAt, expiresAt: expiresAt},
		{name: "missing path id", id: "transfer-1", initiator: "creator-1", recipient: "recipient-1", createdAt: createdAt, expiresAt: expiresAt},
		{name: "blank path id", id: "transfer-1", pathID: "  ", initiator: "creator-1", recipient: "recipient-1", createdAt: createdAt, expiresAt: expiresAt},
		{name: "missing initiator", id: "transfer-1", pathID: "path-1", recipient: "recipient-1", createdAt: createdAt, expiresAt: expiresAt},
		{name: "missing recipient", id: "transfer-1", pathID: "path-1", initiator: "creator-1", createdAt: createdAt, expiresAt: expiresAt},
		{name: "same actor", id: "transfer-1", pathID: "path-1", initiator: "same-user", recipient: "same-user", createdAt: createdAt, expiresAt: expiresAt},
		{name: "same actor after trimming", id: "transfer-1", pathID: "path-1", initiator: " same-user ", recipient: "same-user", createdAt: createdAt, expiresAt: expiresAt},
		{name: "missing creation instant", id: "transfer-1", pathID: "path-1", initiator: "creator-1", recipient: "recipient-1", expiresAt: expiresAt},
		{name: "missing expiration instant", id: "transfer-1", pathID: "path-1", initiator: "creator-1", recipient: "recipient-1", createdAt: createdAt},
		{name: "expiration equals creation", id: "transfer-1", pathID: "path-1", initiator: "creator-1", recipient: "recipient-1", createdAt: createdAt, expiresAt: createdAt},
		{name: "expiration before creation", id: "transfer-1", pathID: "path-1", initiator: "creator-1", recipient: "recipient-1", createdAt: createdAt, expiresAt: createdAt.Add(-time.Second)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewOwnershipTransfer(test.id, test.pathID, test.initiator, test.recipient, test.createdAt, test.expiresAt)
			if !errors.Is(err, ErrInvalidFields) {
				t.Fatalf("NewOwnershipTransfer() error = %v, want %v", err, ErrInvalidFields)
			}
		})
	}
}

func TestOwnershipTransferAcceptsOnlyTheRecipientBeforeExpiry(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(time.Hour)
	transfer := mustNewOwnershipTransfer(t, createdAt, expiresAt)

	for _, test := range []struct {
		name string
		user string
		at   time.Time
	}{
		{name: "wrong user", user: "creator-1", at: createdAt.Add(time.Minute)},
		{name: "blank user", user: " ", at: createdAt.Add(time.Minute)},
		{name: "zero time", user: "recipient-1"},
		{name: "before creation", user: "recipient-1", at: createdAt.Add(-time.Nanosecond)},
		{name: "at expiration", user: "recipient-1", at: expiresAt},
		{name: "after expiration", user: "recipient-1", at: expiresAt.Add(time.Nanosecond)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := transfer.Accept(test.user, test.at); !errors.Is(err, ErrOwnershipTransferUnavailable) {
				t.Fatalf("Accept() error = %v, want %v", err, ErrOwnershipTransferUnavailable)
			}
		})
	}

	acceptedAt := expiresAt.Add(-time.Nanosecond)
	accepted, err := transfer.Accept(" recipient-1 ", acceptedAt)
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if accepted.Pending(acceptedAt) || accepted.AcceptedAt != acceptedAt {
		t.Fatalf("Accept() = %+v, want terminal acceptance at %v", accepted, acceptedAt)
	}
	if !transfer.AcceptedAt.IsZero() {
		t.Fatalf("Accept() mutated original transfer: %+v", transfer)
	}
	assertOwnershipTransferUnavailableForAllTransitions(t, accepted, acceptedAt.Add(time.Nanosecond))
}

func TestOwnershipTransferCanBeDeclinedOnlyByRecipient(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(time.Hour)
	transfer := mustNewOwnershipTransfer(t, createdAt, expiresAt)

	if _, err := transfer.Decline("creator-1", createdAt.Add(time.Minute)); !errors.Is(err, ErrOwnershipTransferUnavailable) {
		t.Fatalf("Decline(creator) error = %v, want unavailable", err)
	}
	declinedAt := createdAt.Add(time.Minute)
	declined, err := transfer.Decline("recipient-1", declinedAt)
	if err != nil {
		t.Fatalf("Decline(recipient) error = %v", err)
	}
	if declined.Pending(declinedAt) || declined.DeclinedAt != declinedAt {
		t.Fatalf("Decline() = %+v, want terminal decline", declined)
	}
	assertOwnershipTransferUnavailableForAllTransitions(t, declined, declinedAt.Add(time.Second))
}

func TestOwnershipTransferCanBeCanceledOnlyByInitiatingCreator(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(time.Hour)
	transfer := mustNewOwnershipTransfer(t, createdAt, expiresAt)

	if _, err := transfer.Cancel("recipient-1", createdAt.Add(time.Minute)); !errors.Is(err, ErrOwnershipTransferUnavailable) {
		t.Fatalf("Cancel(recipient) error = %v, want unavailable", err)
	}
	canceledAt := createdAt.Add(time.Minute)
	canceled, err := transfer.Cancel(" creator-1 ", canceledAt)
	if err != nil {
		t.Fatalf("Cancel(creator) error = %v", err)
	}
	if canceled.Pending(canceledAt) || canceled.CanceledAt != canceledAt {
		t.Fatalf("Cancel() = %+v, want terminal cancellation", canceled)
	}
	assertOwnershipTransferUnavailableForAllTransitions(t, canceled, canceledAt.Add(time.Second))
}

func TestExpiredOwnershipTransferIsUnavailableAndRetainsNoTransition(t *testing.T) {
	createdAt := time.Date(2026, 7, 27, 14, 0, 0, 0, time.UTC)
	expiresAt := createdAt.Add(time.Hour)
	transfer := mustNewOwnershipTransfer(t, createdAt, expiresAt)

	if transfer.Pending(expiresAt) {
		t.Fatal("transfer is pending at its expiration instant")
	}
	if !transfer.Expired(expiresAt) {
		t.Fatal("transfer is not expired at its expiration instant")
	}
	assertOwnershipTransferUnavailableForAllTransitions(t, transfer, expiresAt)
	if !transfer.AcceptedAt.IsZero() || !transfer.DeclinedAt.IsZero() || !transfer.CanceledAt.IsZero() {
		t.Fatalf("expiration recorded a role-affecting transition: %+v", transfer)
	}
}

func mustNewOwnershipTransfer(t *testing.T, createdAt, expiresAt time.Time) OwnershipTransfer {
	t.Helper()
	transfer, err := NewOwnershipTransfer("transfer-1", "path-1", "creator-1", "recipient-1", createdAt, expiresAt)
	if err != nil {
		t.Fatalf("NewOwnershipTransfer() error = %v", err)
	}
	return transfer
}

func assertOwnershipTransferUnavailableForAllTransitions(t *testing.T, transfer OwnershipTransfer, at time.Time) {
	t.Helper()
	transitions := []struct {
		name string
		run  func() error
	}{
		{name: "accept", run: func() error { _, err := transfer.Accept("recipient-1", at); return err }},
		{name: "decline", run: func() error { _, err := transfer.Decline("recipient-1", at); return err }},
		{name: "cancel", run: func() error { _, err := transfer.Cancel("creator-1", at); return err }},
	}
	for _, transition := range transitions {
		t.Run(transition.name, func(t *testing.T) {
			if err := transition.run(); !errors.Is(err, ErrOwnershipTransferUnavailable) {
				t.Fatalf("transition error = %v, want %v", err, ErrOwnershipTransferUnavailable)
			}
		})
	}
}
