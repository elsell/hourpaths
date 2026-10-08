package gormstore

import "github.com/elsell/hour-paths/apps/api/internal/domain/identity"

func identityUserFromModel(user userModel) identity.User {
	email := ""
	if user.ProviderEmailVerified {
		email = user.Email
	}
	visibility := identity.ProfileVisibility("")
	if user.ProfileVisibility != nil {
		visibility = *user.ProfileVisibility
	}
	return identity.User{ID: user.ID, Email: email, DisplayName: user.DisplayName, Status: user.Status, InvitationAdmin: user.InvitationAdmin, ProfileVisibility: visibility, CreatedAt: user.CreatedAt, UpdatedAt: user.UpdatedAt}
}
