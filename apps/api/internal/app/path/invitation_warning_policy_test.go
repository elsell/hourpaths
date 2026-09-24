package path

import (
	"context"
	"errors"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestVisibilityInvitationWarningPolicyReturnsCurrentVisibilityAndRetainedActivityContext(t *testing.T) {
	tests := []struct {
		name        string
		input       InvitationWarningInput
		wantWarning bool
		wantContext InvitationWarningContext
		err         error
	}{
		{
			name: "private participant joining followers Path",
			input: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPrivate,
				PathVisibility:      "followers",
				OfferedRole:         domain.RoleParticipant,
			},
			wantWarning: true,
			wantContext: InvitationWarningContext{
				PathVisibility: "followers",
			},
		},
		{
			name: "private participant rejoining public Path with retained activity",
			input: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPrivate,
				PathVisibility:      "public",
				OfferedRole:         domain.RoleParticipant,
				HasRetainedActivity: true,
			},
			wantWarning: true,
			wantContext: InvitationWarningContext{
				PathVisibility:      "public",
				HasRetainedActivity: true,
			},
		},
		{
			name: "private participant joining private Path",
			input: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPrivate,
				PathVisibility:      "private",
				OfferedRole:         domain.RoleParticipant,
			},
		},
		{
			name: "private supporter joining public Path",
			input: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPrivate,
				PathVisibility:      "public",
				OfferedRole:         domain.RoleSupporter,
			},
		},
		{
			name: "public participant joining public Path",
			input: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPublic,
				PathVisibility:      "public",
				OfferedRole:         domain.RoleParticipant,
			},
		},
		{
			name: "invalid profile visibility",
			input: InvitationWarningInput{
				RecipientVisibility: "friends",
				PathVisibility:      "public",
				OfferedRole:         domain.RoleParticipant,
			},
			err: ports.ErrInvalidArgument,
		},
		{
			name: "invalid Path visibility",
			input: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPrivate,
				PathVisibility:      "friends",
				OfferedRole:         domain.RoleParticipant,
			},
			err: ports.ErrInvalidArgument,
		},
		{
			name: "invalid offered role",
			input: InvitationWarningInput{
				RecipientVisibility: identity.ProfileVisibilityPrivate,
				PathVisibility:      "public",
				OfferedRole:         "administrator",
			},
			err: ports.ErrInvalidArgument,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (VisibilityInvitationWarningPolicy{}).Evaluate(context.Background(), test.input)
			if !errors.Is(err, test.err) ||
				got.Required != test.wantWarning ||
				got.Context != test.wantContext {
				t.Fatalf(
					"Evaluate() = %+v, %v; want required=%v context=%+v error=%v",
					got, err, test.wantWarning, test.wantContext, test.err,
				)
			}
		})
	}
}
