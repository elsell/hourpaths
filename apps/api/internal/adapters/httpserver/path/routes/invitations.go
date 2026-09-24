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
)

type InvitationService interface {
	ReviewRecipient(context.Context, string, domain.ID, string) (pathapp.InvitationRecipientReview, error)
	Send(context.Context, string, string, domain.ID, string, string, domain.MembershipRole) (pathapp.SendInvitationResult, error)
	ListPending(context.Context, string, string, int) ([]pathapp.PendingInvitation, string, error)
	ListNotifications(context.Context, string, string, int) ([]pathapp.InvitationNotificationProjection, string, int64, error)
	GetNotification(context.Context, string, string) (pathapp.InvitationNotificationProjection, error)
	MarkNotificationRead(context.Context, string, string) (pathapp.NotificationMutationResult, error)
	DeleteNotification(context.Context, string, string) (pathapp.NotificationMutationResult, error)
	MarkAllNotificationsRead(context.Context, string) (pathapp.NotificationMutationResult, error)
	Accept(context.Context, string, string, domain.InvitationID) (pathapp.AcceptInvitationResult, error)
	Reject(context.Context, string, string, domain.InvitationID) (pathapp.RejectInvitationResult, error)
	ListManagedPending(context.Context, string, domain.ID, string, int) ([]pathapp.ManagedInvitation, string, error)
	CancelInvitation(context.Context, string, string, domain.ID, domain.InvitationID) (pathapp.CancelInvitationResult, error)
}

type invitationConfirmationService interface {
	AcceptConfirmed(
		context.Context,
		string,
		string,
		domain.InvitationID,
		pathapp.InvitationWarningAcknowledgement,
	) (pathapp.AcceptInvitationResult, error)
}

var errInvitationConfirmationUnavailable = errors.New("path invitation confirmation service is unavailable")

var _ InvitationService = (*pathapp.InvitationService)(nil)
var _ InvitationService = (*pathapp.Service)(nil)
var _ invitationConfirmationService = (*pathapp.InvitationService)(nil)
var _ invitationConfirmationService = (*pathapp.Service)(nil)

type PathInvitationSendInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.PathInvitationCreate
}

type PathInvitationRecipientInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	Username      string `query:"username" required:"true" minLength:"3" maxLength:"64" pattern:"^[A-Za-z0-9_.]+$"`
}

type PathInvitationListInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}

type ManagedPathInvitationListInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}

type PathInvitationCancelInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	InvitationID   string `path:"invitationId"`
}

type PathInvitationAcceptInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	InvitationID   string `path:"invitationId"`
	Body           *dto.PathInvitationAccept
}

type PathInvitationRejectInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	InvitationID   string `path:"invitationId"`
}

type NotificationListInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}

type NotificationMutationInput struct {
	Authorization  string `header:"Authorization"`
	NotificationID string `path:"notificationId"`
}

type NotificationMarkAllInput struct {
	Authorization string `header:"Authorization"`
}

type PathInvitationOutput struct {
	Body struct {
		Data dto.PathInvitation `json:"data"`
	}
}

type PathInvitationRejectionOutput struct {
	Body struct {
		Data dto.PathInvitationRejection `json:"data"`
	}
}

type PathInvitationRecipientOutput struct {
	Body struct {
		Data dto.PathInvitationRecipient `json:"data"`
	}
}

type PathInvitationListOutput struct {
	Body struct {
		Data []dto.PendingPathInvitation `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}

type ManagedPathInvitationListOutput struct {
	Body struct {
		Data []dto.ManagedPathInvitation `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}

type PathInvitationCancellationOutput struct {
	Body struct {
		Data dto.PathInvitationCancellation `json:"data"`
	}
}

type NotificationListOutput struct {
	Body struct {
		Data []dto.PathInvitationNotification `json:"data" nullable:"false"`
		Meta dto.NotificationListMeta         `json:"meta"`
	}
}

type NotificationMutationOutput struct {
	Body struct {
		Data dto.NotificationMutationResult `json:"data"`
	}
}

type NotificationOutput struct {
	Body struct {
		Data dto.PathInvitationNotification `json:"data"`
	}
}

func RegisterInvitations(api huma.API, service InvitationService) {
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{
		OperationID: "review-path-invitation-recipient",
		Method:      http.MethodGet,
		Path:        "/v1/paths/{pathId}/invitation-recipient",
		Security:    security,
	}, func(ctx context.Context, input *PathInvitationRecipientInput) (*PathInvitationRecipientOutput, error) {
		recipient, err := service.ReviewRecipient(
			ctx, input.Authorization, domain.ID(input.PathID), input.Username,
		)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &PathInvitationRecipientOutput{}
		output.Body.Data = mapper.InvitationRecipient(recipient)
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID:   "send-path-invitation",
		Method:        http.MethodPost,
		Path:          "/v1/paths/{pathId}/invitations",
		DefaultStatus: http.StatusCreated,
		Security:      security,
	}, func(ctx context.Context, input *PathInvitationSendInput) (*PathInvitationOutput, error) {
		result, err := service.Send(
			ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID),
			input.Body.Username, input.Body.ExpectedRecipientUserID,
			domain.MembershipRole(input.Body.OfferedRole),
		)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &PathInvitationOutput{}
		output.Body.Data = mapper.Invitation(result.Invitation)
		return output, nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-managed-path-invitations", Method: http.MethodGet, Path: "/v1/paths/{pathId}/invitations", Security: security}, func(ctx context.Context, input *ManagedPathInvitationListInput) (*ManagedPathInvitationListOutput, error) {
		invitations, next, err := service.ListManagedPending(ctx, input.Authorization, domain.ID(input.PathID), input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &ManagedPathInvitationListOutput{}
		output.Body.Data = make([]dto.ManagedPathInvitation, 0, len(invitations))
		for _, invitation := range invitations {
			output.Body.Data = append(output.Body.Data, mapper.ManagedInvitation(invitation))
		}
		output.Body.Meta.NextCursor = next
		return output, nil
	})
	huma.Register(api, huma.Operation{OperationID: "cancel-path-invitation", Method: http.MethodDelete, Path: "/v1/paths/{pathId}/invitations/{invitationId}", Security: security}, func(ctx context.Context, input *PathInvitationCancelInput) (*PathInvitationCancellationOutput, error) {
		result, err := service.CancelInvitation(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), domain.InvitationID(input.InvitationID))
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &PathInvitationCancellationOutput{}
		output.Body.Data = mapper.InvitationCancellation(result)
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "list-pending-path-invitations",
		Method:      http.MethodGet,
		Path:        "/v1/path-invitations",
		Security:    security,
	}, func(ctx context.Context, input *PathInvitationListInput) (*PathInvitationListOutput, error) {
		invitations, nextCursor, err := service.ListPending(ctx, input.Authorization, input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		output := &PathInvitationListOutput{}
		output.Body.Data = make([]dto.PendingPathInvitation, 0, len(invitations))
		for _, invitation := range invitations {
			output.Body.Data = append(output.Body.Data, mapper.PendingInvitation(invitation))
		}
		output.Body.Meta.NextCursor = nextCursor
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "list-notifications",
		Method:      http.MethodGet,
		Path:        "/v1/notifications",
		Security:    security,
	}, func(ctx context.Context, input *NotificationListInput) (*NotificationListOutput, error) {
		notifications, nextCursor, unreadCount, err := service.ListNotifications(
			ctx, input.Authorization, input.Cursor, input.Limit,
		)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		output := &NotificationListOutput{}
		output.Body.Data = make([]dto.PathInvitationNotification, 0, len(notifications))
		for _, notification := range notifications {
			output.Body.Data = append(output.Body.Data, mapper.InvitationNotification(notification))
		}
		output.Body.Meta.NextCursor = nextCursor
		output.Body.Meta.UnreadCount = unreadCount
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "get-notification",
		Method:      http.MethodGet,
		Path:        "/v1/notifications/{notificationId}",
		Summary:     "Resolve one current notification",
		Security:    security,
	}, func(ctx context.Context, input *NotificationMutationInput) (*NotificationOutput, error) {
		notification, err := service.GetNotification(
			ctx, input.Authorization, input.NotificationID,
		)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &NotificationOutput{}
		output.Body.Data = mapper.InvitationNotification(notification)
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "mark-notification-read",
		Method:      http.MethodPatch,
		Path:        "/v1/notifications/{notificationId}/read",
		Security:    security,
	}, func(ctx context.Context, input *NotificationMutationInput) (*NotificationMutationOutput, error) {
		result, err := service.MarkNotificationRead(ctx, input.Authorization, input.NotificationID)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		output := &NotificationMutationOutput{}
		output.Body.Data.UnreadCount = result.UnreadCount
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "delete-notification",
		Method:      http.MethodDelete,
		Path:        "/v1/notifications/{notificationId}",
		Security:    security,
	}, func(ctx context.Context, input *NotificationMutationInput) (*NotificationMutationOutput, error) {
		result, err := service.DeleteNotification(ctx, input.Authorization, input.NotificationID)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		output := &NotificationMutationOutput{}
		output.Body.Data.UnreadCount = result.UnreadCount
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "mark-all-notifications-read",
		Method:      http.MethodPost,
		Path:        "/v1/notifications/read-all",
		Security:    security,
	}, func(ctx context.Context, input *NotificationMarkAllInput) (*NotificationMutationOutput, error) {
		result, err := service.MarkAllNotificationsRead(ctx, input.Authorization)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		output := &NotificationMutationOutput{}
		output.Body.Data.UnreadCount = result.UnreadCount
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "accept-path-invitation",
		Method:      http.MethodPost,
		Path:        "/v1/path-invitations/{invitationId}/accept",
		Security:    security,
	}, func(ctx context.Context, input *PathInvitationAcceptInput) (*PathInvitationOutput, error) {
		var result pathapp.AcceptInvitationResult
		var err error
		acknowledgement := input.Body
		if acknowledgement == nil || acknowledgement.VisibilityWarningAcknowledgement == nil {
			result, err = service.Accept(
				ctx, input.Authorization, input.IdempotencyKey, domain.InvitationID(input.InvitationID),
			)
		} else if confirmed, ok := service.(invitationConfirmationService); ok {
			result, err = confirmed.AcceptConfirmed(
				ctx, input.Authorization, input.IdempotencyKey, domain.InvitationID(input.InvitationID),
				pathapp.InvitationWarningAcknowledgement{
					PathVisibility: acknowledgement.VisibilityWarningAcknowledgement.PathVisibility,
				},
			)
		} else {
			err = errInvitationConfirmationUnavailable
		}
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &PathInvitationOutput{}
		output.Body.Data = mapper.Invitation(result.Invitation)
		return output, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "reject-path-invitation",
		Method:      http.MethodPost,
		Path:        "/v1/path-invitations/{invitationId}/reject",
		Security:    security,
	}, func(ctx context.Context, input *PathInvitationRejectInput) (*PathInvitationRejectionOutput, error) {
		result, err := service.Reject(
			ctx, input.Authorization, input.IdempotencyKey, domain.InvitationID(input.InvitationID),
		)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		output := &PathInvitationRejectionOutput{}
		output.Body.Data = mapper.InvitationRejection(result)
		return output, nil
	})
}
