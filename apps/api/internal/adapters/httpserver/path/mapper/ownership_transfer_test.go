package mapper

import (
	"testing"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func TestOwnershipTransferMapperProjectsIdentityLifecycleAndAcceptedPath(t *testing.T) {
	createdAt := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	acceptedAt := createdAt.Add(time.Hour)
	transfer, err := domain.NewOwnershipTransfer("transfer-1", "path-1", "creator-1", "recipient-1", createdAt, createdAt.Add(7*24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	transfer.AcceptedAt = acceptedAt
	result := OwnershipTransferResult(pathapp.OwnershipTransferResult{
		Transfer: transfer,
		Path:     domain.Entity{ID: "path-1", OwnerUserID: "recipient-1", Attributes: domain.Attributes{Name: "Violin", Visibility: "private"}},
		Replayed: true,
	})
	if result.Transfer.ID != "transfer-1" || result.Transfer.PathID != "path-1" ||
		result.Transfer.CreatorUserID != "creator-1" || result.Transfer.RecipientUserID != "recipient-1" ||
		result.Transfer.State != "accepted" || result.Transfer.AcceptedAt == nil || !result.Transfer.AcceptedAt.Equal(acceptedAt) ||
		result.Path == nil || result.Path.ID != "path-1" || result.Path.Name != "Violin" || !result.Replayed {
		t.Fatalf("OwnershipTransferResult() = %+v", result)
	}
}

func TestOwnershipTransferMapperProjectsEveryTerminalStateWithoutInventingPath(t *testing.T) {
	createdAt := time.Date(2026, time.July, 27, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, state string
		mutate      func(*domain.OwnershipTransfer)
	}{
		{name: "pending", state: "pending", mutate: func(*domain.OwnershipTransfer) {}},
		{name: "declined", state: "declined", mutate: func(value *domain.OwnershipTransfer) { value.DeclinedAt = createdAt.Add(time.Minute) }},
		{name: "canceled", state: "canceled", mutate: func(value *domain.OwnershipTransfer) { value.CanceledAt = createdAt.Add(time.Minute) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			transfer, err := domain.NewOwnershipTransfer("transfer-1", "path-1", "creator-1", "recipient-1", createdAt, createdAt.Add(time.Hour))
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(&transfer)
			result := OwnershipTransferResult(pathapp.OwnershipTransferResult{Transfer: transfer})
			if result.Transfer.State != test.state || result.Path != nil {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

func TestOwnershipTransferCandidateMapperPreservesPublicIdentityAndAdministratorEligibility(t *testing.T) {
	result := OwnershipTransferCandidate(pathapp.OwnershipTransferCandidate{
		UserID: "participant-1", Username: "Reader.One", DisplayName: "Reader One", Administrator: true,
	})
	if result.UserID != "participant-1" || result.Username != "Reader.One" || result.DisplayName != "Reader One" || !result.Administrator {
		t.Fatalf("OwnershipTransferCandidate() = %+v", result)
	}
}
