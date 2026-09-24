package mapper

import (
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

func OwnershipTransferCandidate(value pathapp.OwnershipTransferCandidate) dto.OwnershipTransferCandidate {
	return dto.OwnershipTransferCandidate{
		UserID: value.UserID, Username: value.Username, DisplayName: value.DisplayName,
		Administrator: value.Administrator,
	}
}

func OwnershipTransfer(value domain.OwnershipTransfer) dto.OwnershipTransfer {
	result := dto.OwnershipTransfer{
		ID: string(value.ID), PathID: string(value.PathID),
		CreatorUserID: value.InitiatorUserID, RecipientUserID: value.RecipientUserID,
		ReviewedAt: value.ReviewedAt, CreatedAt: value.CreatedAt, ExpiresAt: value.ExpiresAt, State: "pending",
	}
	switch {
	case !value.AcceptedAt.IsZero():
		result.State = "accepted"
		acceptedAt := value.AcceptedAt
		result.AcceptedAt = &acceptedAt
	case !value.DeclinedAt.IsZero():
		result.State = "declined"
		declinedAt := value.DeclinedAt
		result.DeclinedAt = &declinedAt
	case !value.CanceledAt.IsZero():
		result.State = "canceled"
		canceledAt := value.CanceledAt
		result.CanceledAt = &canceledAt
	}
	return result
}

func OwnershipTransferResult(value pathapp.OwnershipTransferResult) dto.OwnershipTransferResult {
	result := dto.OwnershipTransferResult{
		Transfer: OwnershipTransfer(value.Transfer), Replayed: value.Replayed,
		Counterpart:     dto.OwnershipTransferPublicIdentity{UserID: value.Counterpart.UserID, Username: value.Counterpart.Username, DisplayName: value.Counterpart.DisplayName},
		CounterpartRole: value.CounterpartRole, ViewerTimeZone: value.ViewerTimeZone,
	}
	if value.Path.ID != "" {
		path := Item(value.Path)
		result.Path = &path
	}
	return result
}
