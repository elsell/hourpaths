package routes

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	shared "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver"
	activitydto "github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/activity/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/dto"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/httpserver/path/mapper"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"net/http"
)

type Service interface {
	InvitationService
	Create(context.Context, string, string, domain.Attributes) (domain.Entity, error)
	ListProjected(context.Context, string, string, int) ([]pathapp.Projection, string, error)
	ListArchivedProjected(context.Context, string, string, int) ([]pathapp.Projection, string, error)
	ReadHomePreferences(context.Context, string) (domain.HomePreferences, error)
	UpdateHomePreferences(context.Context, string, string, int64, domain.HomeOrderMethod, []domain.ID, []domain.ID) (domain.HomePreferences, error)
	GetProjected(context.Context, string, domain.ID) (pathapp.Projection, error)
	Project(context.Context, string, domain.Entity) (pathapp.Projection, error)
	UpdateGoals(context.Context, string, string, domain.ID, bool, domain.GoalConfiguration, domain.IntervalGoal, domain.OverallTarget) (pathapp.UpdateGoalsResult, error)
	Rename(context.Context, string, string, domain.ID, string, string) (pathapp.RenameResult, error)
	SetArchiveState(context.Context, string, string, domain.ID, bool, bool, bool) (pathapp.SetArchiveStateResult, error)
	SetVisibility(context.Context, string, string, domain.ID, bool, string, string) (pathapp.SetVisibilityResult, error)
	Update(context.Context, string, domain.ID, domain.Attributes) (domain.Entity, error)
	Delete(context.Context, string, string, domain.ID, bool, string) (pathapp.DeletePathResult, error)
	Leave(context.Context, string, string, domain.ID, bool, bool) (pathapp.LeavePathResult, error)
	ReviewMemberRemoval(context.Context, string, domain.ID, string) (pathapp.MemberRemovalReview, error)
	RemoveMember(context.Context, string, string, domain.ID, string, bool, domain.MembershipRole) (pathapp.RemoveMemberResult, error)
	ChangeMemberRole(context.Context, string, string, domain.ID, string, bool, domain.MembershipRole, domain.MembershipRole) (pathapp.ChangeMemberRoleResult, error)
	ListMembers(context.Context, string, domain.ID, string, int) ([]pathapp.Member, string, error)
}

type PathAuthorizationInput struct {
	Authorization string `header:"Authorization"`
}
type PathListInput struct {
	Authorization string `header:"Authorization"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
	Archived      bool   `query:"archived" default:"false"`
}
type PathPathInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
}
type PathCreateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           dto.PathCreate
}
type PathUpdateInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
	Body          dto.PathUpdate
}
type PathGoalsUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.PathGoalsUpdate
}
type PathRenameInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.PathRename
}
type PathArchiveStateUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.PathArchiveStateUpdate
}
type PathVisibilityUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.PathVisibilityUpdate
}
type PathDeleteInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	ID             string `path:"id"`
	Body           dto.PathDelete
}
type PathLeaveInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	Body           dto.PathLeave
}
type MemberRemovalReviewInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	UserID        string `path:"userId"`
}
type MemberRemovalInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	UserID         string `path:"userId"`
	Body           dto.MemberRemoval
}
type MemberRoleChangeInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	PathID         string `path:"pathId"`
	UserID         string `path:"userId"`
	Body           dto.MemberRoleChange
}
type MemberListInput struct {
	Authorization string `header:"Authorization"`
	PathID        string `path:"pathId"`
	Cursor        string `query:"cursor"`
	Limit         int    `query:"limit" default:"25" minimum:"1" maximum:"100"`
}
type PathProjectionOutput struct {
	Body struct {
		Data dto.PathProjection `json:"data"`
	}
}
type PathGoalMutationOutput struct {
	Body struct {
		Data dto.PathGoalMutationResult `json:"data"`
	}
}
type PathListOutput struct {
	Body struct {
		Data []dto.PathProjection `json:"data" nullable:"false"`
		Meta PathListMeta         `json:"meta"`
	}
}
type PathListMeta struct {
	NextCursor      string              `json:"nextCursor,omitempty"`
	HomePreferences dto.HomePreferences `json:"homePreferences" required:"true"`
}
type HomePreferencesUpdateInput struct {
	Authorization  string `header:"Authorization"`
	IdempotencyKey string `header:"Idempotency-Key" required:"true" minLength:"16" maxLength:"128"`
	Body           dto.HomePreferencesUpdate
}
type HomePreferencesOutput struct {
	Body struct {
		Data dto.HomePreferences `json:"data"`
	}
}
type PathNoContentOutput struct{ Status int }
type PathDeletionOutput struct {
	Body struct {
		Data dto.PathDeletionReceipt `json:"data"`
	}
}
type PathLeaveOutput struct {
	Body struct {
		Data dto.PathLeaveReceipt `json:"data"`
	}
}
type MemberRemovalReviewOutput struct {
	Body struct {
		Data dto.MemberRemovalReview `json:"data"`
	}
}
type MemberRemovalOutput struct {
	Body struct {
		Data dto.MemberRemovalReceipt `json:"data"`
	}
}
type MemberRoleChangeOutput struct {
	Body struct {
		Data dto.MemberRoleChangeReceipt `json:"data"`
	}
}
type MemberListOutput struct {
	Body struct {
		Data []dto.PathMember `json:"data" nullable:"false"`
		Meta struct {
			NextCursor string `json:"nextCursor,omitempty"`
		} `json:"meta"`
	}
}

func Register(api huma.API, service Service) {
	RegisterInvitations(api, service)
	path := "/v1/paths"
	security := []map[string][]string{{"oidc": {}}}
	huma.Register(api, huma.Operation{OperationID: "create-path", Method: http.MethodPost, Path: path, DefaultStatus: http.StatusCreated, Security: security}, func(ctx context.Context, input *PathCreateInput) (*PathProjectionOutput, error) {
		attributes, err := mapper.Attributes(input.Body)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		entity, err := service.Create(ctx, input.Authorization, input.IdempotencyKey, attributes)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		projection, err := service.Project(ctx, input.Authorization, entity)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		out := &PathProjectionOutput{}
		out.Body.Data = mapper.ProjectedItem(projection)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-paths", Method: http.MethodGet, Path: path, Security: security}, func(ctx context.Context, input *PathListInput) (*PathListOutput, error) {
		var projections []pathapp.Projection
		var cursor string
		var err error
		if input.Archived {
			projections, cursor, err = service.ListArchivedProjected(ctx, input.Authorization, input.Cursor, input.Limit)
		} else {
			projections, cursor, err = service.ListProjected(ctx, input.Authorization, input.Cursor, input.Limit)
		}
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		out := &PathListOutput{}
		out.Body.Data = make([]dto.PathProjection, 0, len(projections))
		for _, projection := range projections {
			out.Body.Data = append(out.Body.Data, mapper.ProjectedItem(projection))
		}
		out.Body.Meta.NextCursor = cursor
		preferences, err := service.ReadHomePreferences(ctx, input.Authorization)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		out.Body.Meta.HomePreferences = mapper.HomePreferences(preferences)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-home-preferences", Method: http.MethodPut, Path: "/v1/me/home-preferences", Security: security}, func(ctx context.Context, input *HomePreferencesUpdateInput) (*HomePreferencesOutput, error) {
		pinned := make([]domain.ID, len(input.Body.PinnedPathIDs))
		for index, id := range input.Body.PinnedPathIDs {
			pinned[index] = domain.ID(id)
		}
		manual := make([]domain.ID, len(input.Body.ManualPathIDs))
		for index, id := range input.Body.ManualPathIDs {
			manual[index] = domain.ID(id)
		}
		preferences, err := service.UpdateHomePreferences(ctx, input.Authorization, input.IdempotencyKey, input.Body.ExpectedRevision, domain.HomeOrderMethod(input.Body.OrderMethod), pinned, manual)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		out := &HomePreferencesOutput{}
		out.Body.Data = mapper.HomePreferences(preferences)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "get-path", Method: http.MethodGet, Path: path + "/{id}", Security: security}, func(ctx context.Context, input *PathPathInput) (*PathProjectionOutput, error) {
		projection, err := service.GetProjected(ctx, input.Authorization, domain.ID(input.ID))
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathProjectionOutput{}
		out.Body.Data = mapper.ProjectedItem(projection)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-path-goals", Method: http.MethodPut, Path: path + "/{pathId}/goals", Security: security}, func(ctx context.Context, input *PathGoalsUpdateInput) (*PathGoalMutationOutput, error) {
		if !input.Body.Confirmed {
			return nil, shared.MapError(ports.ErrInvalidArgument, false)
		}
		expected, proposed, err := mapper.GoalConfigurations(input.Body)
		if err != nil {
			return nil, shared.MapError(err, false)
		}
		result, err := service.UpdateGoals(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), input.Body.Confirmed, expected, proposed.IntervalGoal, proposed.OverallTarget)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		projection, err := service.Project(ctx, input.Authorization, result.Path)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathGoalMutationOutput{}
		out.Body.Data.Path = mapper.ProjectedItem(projection)
		out.Body.Data.AccumulatedSeconds = result.AccumulatedSeconds
		if result.IntervalProgress != nil {
			out.Body.Data.IntervalProgress = &activitydto.IntervalProgress{
				AccumulatedSeconds: result.IntervalProgress.AccumulatedSeconds,
				TargetSeconds:      result.IntervalProgress.TargetSeconds,
			}
		}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "rename-path", Method: http.MethodPut, Path: path + "/{pathId}/name", Security: security}, func(ctx context.Context, input *PathRenameInput) (*PathProjectionOutput, error) {
		result, err := service.Rename(
			ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID),
			input.Body.ExpectedName, input.Body.Name,
		)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		projection, err := service.Project(ctx, input.Authorization, result.Path)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathProjectionOutput{}
		out.Body.Data = mapper.ProjectedItem(projection)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "set-path-archive-state", Method: http.MethodPut, Path: path + "/{pathId}/archive-state", Security: security}, func(ctx context.Context, input *PathArchiveStateUpdateInput) (*PathProjectionOutput, error) {
		if !input.Body.Confirmed || input.Body.ExpectedArchived == input.Body.Archived {
			return nil, shared.MapError(ports.ErrInvalidArgument, false)
		}
		result, err := service.SetArchiveState(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), input.Body.Confirmed, input.Body.ExpectedArchived, input.Body.Archived)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		projection, err := service.Project(ctx, input.Authorization, result.Path)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathProjectionOutput{}
		out.Body.Data = mapper.ProjectedItem(projection)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "set-path-visibility", Method: http.MethodPut, Path: path + "/{pathId}/visibility", Security: security}, func(ctx context.Context, input *PathVisibilityUpdateInput) (*PathProjectionOutput, error) {
		if !input.Body.Confirmed || input.Body.ExpectedVisibility == input.Body.Visibility {
			return nil, shared.MapError(ports.ErrInvalidArgument, false)
		}
		result, err := service.SetVisibility(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), input.Body.Confirmed, input.Body.ExpectedVisibility, input.Body.Visibility)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		projection, err := service.Project(ctx, input.Authorization, result.Path)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathProjectionOutput{}
		out.Body.Data = mapper.ProjectedItem(projection)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "update-path", Method: http.MethodPut, Path: path + "/{id}", Security: security}, func(ctx context.Context, input *PathUpdateInput) (*PathProjectionOutput, error) {
		entity, err := service.Update(ctx, input.Authorization, domain.ID(input.ID), mapper.UpdateAttributes(input.Body))
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		projection, err := service.Project(ctx, input.Authorization, entity)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathProjectionOutput{}
		out.Body.Data = mapper.ProjectedItem(projection)
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "delete-path", Method: http.MethodDelete, Path: path + "/{id}", Security: security}, func(ctx context.Context, input *PathDeleteInput) (*PathDeletionOutput, error) {
		if !input.Body.Confirmed {
			return nil, shared.MapError(ports.ErrInvalidArgument, false)
		}
		result, err := service.Delete(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.ID), input.Body.Confirmed, input.Body.ExpectedName)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathDeletionOutput{}
		out.Body.Data = dto.PathDeletionReceipt{PathID: string(result.PathID), Deleted: result.Deleted}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "leave-path", Method: http.MethodDelete, Path: path + "/{pathId}/membership", Security: security}, func(ctx context.Context, input *PathLeaveInput) (*PathLeaveOutput, error) {
		if !input.Body.Confirmed {
			return nil, shared.MapError(ports.ErrInvalidArgument, false)
		}
		result, err := service.Leave(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), input.Body.Confirmed, input.Body.RetainActivity)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &PathLeaveOutput{}
		out.Body.Data = dto.PathLeaveReceipt{PathID: string(result.PathID), Left: result.Left, ActivityRetained: result.ActivityRetained}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "review-path-member-removal", Method: http.MethodGet, Path: path + "/{pathId}/members/{userId}/removal-review", Security: security}, func(ctx context.Context, input *MemberRemovalReviewInput) (*MemberRemovalReviewOutput, error) {
		review, err := service.ReviewMemberRemoval(ctx, input.Authorization, domain.ID(input.PathID), input.UserID)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &MemberRemovalReviewOutput{}
		out.Body.Data = dto.MemberRemovalReview{UserID: review.UserID, Username: review.Username, DisplayName: review.DisplayName, Role: string(review.Role), SessionCount: review.SessionCount, TotalTrackedSeconds: review.TotalTrackedSeconds, RunningTimer: review.RunningTimer}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "list-path-members", Method: http.MethodGet, Path: path + "/{pathId}/members", Security: security}, func(ctx context.Context, input *MemberListInput) (*MemberListOutput, error) {
		members, next, err := service.ListMembers(ctx, input.Authorization, domain.ID(input.PathID), input.Cursor, input.Limit)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &MemberListOutput{}
		out.Body.Data = make([]dto.PathMember, 0, len(members))
		for _, member := range members {
			out.Body.Data = append(out.Body.Data, dto.PathMember{UserID: member.UserID, Username: member.Username, DisplayName: member.DisplayName, Role: member.Role, SessionCount: member.SessionCount, TotalTrackedSeconds: member.TotalTrackedSeconds, IntervalProgress: goalProgressDTO(member.IntervalProgress), OverallProgress: goalProgressDTO(member.OverallProgress), BlockedByViewer: member.BlockedByViewer, CanRemove: member.CanRemove, CanChangeRole: member.CanChangeRole, CanGrantAdministrator: member.CanGrantAdministrator, CanRevokeAdministrator: member.CanRevokeAdministrator, CanStepDownAdministrator: member.CanStepDownAdministrator, CanLeave: member.CanLeave})
		}
		out.Body.Meta.NextCursor = next
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "remove-path-member", Method: http.MethodDelete, Path: path + "/{pathId}/members/{userId}", Security: security}, func(ctx context.Context, input *MemberRemovalInput) (*MemberRemovalOutput, error) {
		role := domain.MembershipRole(input.Body.ExpectedRole)
		if !input.Body.Confirmed || !role.Valid() {
			return nil, shared.MapError(ports.ErrInvalidArgument, false)
		}
		result, err := service.RemoveMember(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), input.UserID, input.Body.Confirmed, role)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &MemberRemovalOutput{}
		out.Body.Data = dto.MemberRemovalReceipt{PathID: string(result.PathID), UserID: result.UserID, Removed: result.Removed, ActivityDeleted: result.ActivityDeleted}
		return out, nil
	})
	huma.Register(api, huma.Operation{OperationID: "change-path-member-role", Method: http.MethodPatch, Path: path + "/{pathId}/members/{userId}", Security: security}, func(ctx context.Context, input *MemberRoleChangeInput) (*MemberRoleChangeOutput, error) {
		expectedRole, role := domain.MembershipRole(input.Body.ExpectedRole), domain.MembershipRole(input.Body.Role)
		if !input.Body.Confirmed || !expectedRole.ValidPathMemberRole() || !role.ValidPathMemberRole() || expectedRole == role {
			return nil, shared.MapError(ports.ErrInvalidArgument, false)
		}
		result, err := service.ChangeMemberRole(ctx, input.Authorization, input.IdempotencyKey, domain.ID(input.PathID), input.UserID, input.Body.Confirmed, expectedRole, role)
		if err != nil {
			return nil, shared.MapError(err, true)
		}
		out := &MemberRoleChangeOutput{}
		out.Body.Data = dto.MemberRoleChangeReceipt{PathID: string(result.PathID), UserID: result.UserID, Role: string(result.Role), ActivityDeleted: result.ActivityDeleted}
		return out, nil
	})
}

func goalProgressDTO(progress *pathapp.GoalProgress) *dto.GoalProgress {
	if progress == nil {
		return nil
	}
	return &dto.GoalProgress{AccumulatedSeconds: progress.AccumulatedSeconds, TargetSeconds: progress.TargetSeconds}
}
