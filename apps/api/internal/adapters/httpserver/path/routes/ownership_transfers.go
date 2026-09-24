package routes

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/mapper"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type OwnershipTransferService interface {
	ListCandidates(context.Context, string, domain.ID, string, int) ([]pathapp.OwnershipTransferCandidate, string, error)
	GetPending(context.Context, string, domain.ID) (pathapp.OwnershipTransferDecision, error)
	Review(context.Context, string, domain.ID, string) (pathapp.OwnershipTransferReview, error)
	Initiate(context.Context, string, domain.ID, string, string, bool) (pathapp.OwnershipTransferResult, error)
	Accept(context.Context, string, domain.OwnershipTransferID, string) (pathapp.OwnershipTransferResult, error)
	Decline(context.Context, string, domain.OwnershipTransferID, string) (pathapp.OwnershipTransferResult, error)
	Cancel(context.Context, string, domain.OwnershipTransferID, string) (pathapp.OwnershipTransferResult, error)
}

var _ OwnershipTransferService = (*pathapp.OwnershipTransferService)(nil)

type OwnershipTransferPathInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
}

type OwnershipTransferCandidateListInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" minimum:"1" maximum:"100" default:"25"`
}

type OwnershipTransferCreateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.OwnershipTransferCreate
}

type OwnershipTransferReviewInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	Body          dto.OwnershipTransferReviewRequest
}

type OwnershipTransferReviewOutput struct {
	Body struct {
		Data dto.OwnershipTransferReview `json:"data"`
	}
}

type OwnershipTransferMutationInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	TransferID     string `path:"transferId"`
}

type OwnershipTransferCandidateListOutput struct {
	Body struct {
		Data []dto.OwnershipTransferCandidate `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}

type OwnershipTransferOutput struct {
	Body struct {
		Data dto.OwnershipTransferResult `json:"data"`
	}
}

func RegisterOwnershipTransfers(api huma.API, service OwnershipTransferService) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{
		OperationID: "list-path-ownership-transfer-candidates", Method: http.MethodGet,
		Path: "/v1/paths/{pathId}/ownership-transfer-candidates", Security: security,
	}, func(ctx context.Context, input *OwnershipTransferCandidateListInput) (*OwnershipTransferCandidateListOutput, error) {
		candidates, nextCursor, err := service.ListCandidates(ctx, input.Authorization, domain.ID(input.PathID), input.Cursor, input.Limit)
		if err != nil {
			return nil, mapOwnershipTransferError(err)
		}
		output := &OwnershipTransferCandidateListOutput{}
		output.Body.Data = make([]dto.OwnershipTransferCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			output.Body.Data = append(output.Body.Data, mapper.OwnershipTransferCandidate(candidate))
		}
		output.Body.Meta.NextCursor = nextCursor
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "review-path-ownership-transfer", Method: http.MethodPost,
		Path: "/v1/paths/{pathId}/ownership-transfer/review", Security: security,
	}, func(ctx context.Context, input *OwnershipTransferReviewInput) (*OwnershipTransferReviewOutput, error) {
		review, err := service.Review(ctx, input.Authorization, domain.ID(input.PathID), input.Body.RecipientUserID)
		if err != nil {
			return nil, mapOwnershipTransferError(err)
		}
		output := &OwnershipTransferReviewOutput{}
		output.Body.Data = dto.OwnershipTransferReview{
			Recipient:  dto.OwnershipTransferPublicIdentity{UserID: review.Recipient.UserID, Username: review.Recipient.Username, DisplayName: review.Recipient.DisplayName},
			ReviewedAt: review.ReviewedAt, ExpiresAt: review.ExpiresAt, ReservationToken: review.ReservationToken, ViewerTimeZone: review.ViewerTimeZone,
		}
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "initiate-path-ownership-transfer", Method: http.MethodPost,
		Path: "/v1/paths/{pathId}/ownership-transfers", DefaultStatus: http.StatusCreated, Security: security,
	}, func(ctx context.Context, input *OwnershipTransferCreateInput) (*OwnershipTransferOutput, error) {
		result, err := service.Initiate(ctx, input.Authorization, domain.ID(input.PathID), input.Body.ReservationToken, input.IdempotencyKey, true)
		if err != nil {
			return nil, mapOwnershipTransferError(err)
		}
		return ownershipTransferOutput(result), nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "get-pending-path-ownership-transfer", Method: http.MethodGet,
		Path: "/v1/paths/{pathId}/ownership-transfer", Security: security,
	}, func(ctx context.Context, input *OwnershipTransferPathInput) (*OwnershipTransferOutput, error) {
		decision, err := service.GetPending(ctx, input.Authorization, domain.ID(input.PathID))
		if err != nil {
			return nil, mapOwnershipTransferError(err)
		}
		return ownershipTransferOutput(pathapp.OwnershipTransferResult{Transfer: decision.Transfer, Path: decision.Path, Counterpart: decision.Counterpart, CounterpartRole: decision.CounterpartRole, ViewerTimeZone: decision.ViewerTimeZone}), nil
	})
	registerOwnershipTransferMutation(api, security, "accept-path-ownership-transfer", "accept", service.Accept)
	registerOwnershipTransferMutation(api, security, "decline-path-ownership-transfer", "decline", service.Decline)
	registerOwnershipTransferMutation(api, security, "cancel-path-ownership-transfer", "cancel", service.Cancel)
}

type ownershipTransferMutation func(context.Context, string, domain.OwnershipTransferID, string) (pathapp.OwnershipTransferResult, error)

func registerOwnershipTransferMutation(api huma.API, security []map[string][]string, operationID, action string, mutate ownershipTransferMutation) {
	huma.Register(api, huma.Operation{
		OperationID: operationID, Method: http.MethodPost,
		Path: "/v1/path-ownership-transfers/{transferId}/" + action, Security: security,
	}, func(ctx context.Context, input *OwnershipTransferMutationInput) (*OwnershipTransferOutput, error) {
		result, err := mutate(ctx, input.Authorization, domain.OwnershipTransferID(input.TransferID), input.IdempotencyKey)
		if err != nil {
			return nil, mapOwnershipTransferError(err)
		}
		return ownershipTransferOutput(result), nil
	})
}

func ownershipTransferOutput(result pathapp.OwnershipTransferResult) *OwnershipTransferOutput {
	output := &OwnershipTransferOutput{}
	output.Body.Data = mapper.OwnershipTransferResult(result)
	return output
}

func mapOwnershipTransferError(err error) error {
	if errors.Is(err, domain.ErrOwnershipTransferUnavailable) {
		return shared.MapError(ports.ErrNotFound, true)
	}
	return shared.MapError(err, true)
}
