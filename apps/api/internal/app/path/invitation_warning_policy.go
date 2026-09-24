package path

import (
	"context"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type VisibilityInvitationWarningPolicy struct{}

func (VisibilityInvitationWarningPolicy) Evaluate(
	_ context.Context,
	input InvitationWarningInput,
) (InvitationWarningDecision, error) {
	if identity.ValidateProfileVisibility(input.RecipientVisibility) != nil ||
		!input.OfferedRole.Valid() ||
		(input.PathVisibility != "private" &&
			input.PathVisibility != "followers" &&
			input.PathVisibility != "public") {
		return InvitationWarningDecision{}, ports.ErrInvalidArgument
	}
	required := input.RecipientVisibility == identity.ProfileVisibilityPrivate &&
		input.OfferedRole == domain.RoleParticipant &&
		input.PathVisibility != "private"
	if !required {
		return InvitationWarningDecision{}, nil
	}
	return InvitationWarningDecision{
		Required: true,
		Context: InvitationWarningContext{
			PathVisibility:      input.PathVisibility,
			HasRetainedActivity: input.HasRetainedActivity,
		},
	}, nil
}
