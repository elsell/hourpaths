package path

import (
	"context"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
)

// Capabilities are the effective actions the authenticated user may take on
// the projected Path. They are derived from authorization decisions rather
// than membership data supplied by a client.
type Capabilities struct {
	TrackTime         bool
	RenamePath        bool
	InviteMembers     bool
	ManageMembers     bool
	ManageGoals       bool
	ManageLifecycle   bool
	ManageVisibility  bool
	TransferOwnership bool
	LeavePath         bool
}

type Projection struct {
	Path         domain.Entity
	Capabilities Capabilities
	Home         HomeOrganization
}

func (s *Service) GetProjected(ctx context.Context, authorization string, id domain.ID) (Projection, error) {
	entity, err := s.Get(ctx, authorization, id)
	if err != nil {
		return Projection{}, err
	}
	return s.Project(ctx, authorization, entity)
}

func (s *Service) Project(ctx context.Context, authorization string, entity domain.Entity) (Projection, error) {
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return Projection{}, err
	}
	if entity.ID == "" || s.Authorizer == nil {
		return Projection{}, errInvalidPathDependencies
	}
	capabilities, err := s.projectCapabilities(ctx, principal.UserID, entity)
	if err != nil {
		return Projection{}, err
	}
	projection := Projection{Path: entity, Capabilities: capabilities}
	home, err := s.Repository.ProjectHome(ctx, principal.UserID, []domain.ID{entity.ID})
	if err != nil || !home.Preferences.Valid() {
		if err != nil {
			return Projection{}, err
		}
		return Projection{}, errInvalidPathDependencies
	}
	if organization, member := home.Organization[entity.ID]; member {
		if organization.PathID != entity.ID || (organization.Classification != HomeSolo && organization.Classification != HomeShared && organization.Classification != HomeSupporting) {
			return Projection{}, errInvalidPathDependencies
		}
		projection.Home = organization
	}
	return projection, nil
}

func (s *Service) ListProjected(ctx context.Context, authorization, cursor string, limit int) ([]Projection, string, error) {
	return s.listProjected(ctx, authorization, cursor, limit, false)
}

func (s *Service) ListArchivedProjected(ctx context.Context, authorization, cursor string, limit int) ([]Projection, string, error) {
	return s.listProjected(ctx, authorization, cursor, limit, true)
}

func (s *Service) listProjected(ctx context.Context, authorization, cursor string, limit int, archived bool) ([]Projection, string, error) {
	var (
		entities   []domain.Entity
		nextCursor string
		err        error
	)
	if archived {
		entities, nextCursor, err = s.ListArchived(ctx, authorization, cursor, limit)
	} else {
		entities, nextCursor, err = s.List(ctx, authorization, cursor, limit)
	}
	if err != nil {
		return nil, "", err
	}
	principal, err := s.authenticate(ctx, authorization)
	if err != nil {
		return nil, "", err
	}
	projections := make([]Projection, 0, len(entities))
	pathIDs := make([]domain.ID, 0, len(entities))
	for _, entity := range entities {
		pathIDs = append(pathIDs, entity.ID)
	}
	home, err := s.Repository.ProjectHome(ctx, principal.UserID, pathIDs)
	if err != nil || !home.Preferences.Valid() || len(home.Organization) != len(entities) {
		if err != nil {
			return nil, "", err
		}
		return nil, "", errInvalidPathDependencies
	}
	for _, entity := range entities {
		capabilities, err := s.projectCapabilities(ctx, principal.UserID, entity)
		if err != nil {
			return nil, "", err
		}
		organization, exists := home.Organization[entity.ID]
		if !exists || organization.PathID != entity.ID || (organization.Classification != HomeSolo && organization.Classification != HomeShared && organization.Classification != HomeSupporting) {
			return nil, "", errInvalidPathDependencies
		}
		projections = append(projections, Projection{Path: entity, Capabilities: capabilities, Home: organization})
	}
	return projections, nextCursor, nil
}

func (s *Service) projectCapabilities(ctx context.Context, userID string, entity domain.Entity) (Capabilities, error) {
	permissions := []struct {
		name    string
		allowed *bool
	}{
		{name: "track"},
		{name: "rename"},
		{name: "manage_members"},
		{name: "manage_goals"},
		{name: "manage_lifecycle"},
		{name: "manage_visibility"},
		{name: "transfer_ownership"},
		{name: "leave"},
	}
	capabilities := Capabilities{}
	permissions[0].allowed = &capabilities.TrackTime
	permissions[1].allowed = &capabilities.RenamePath
	permissions[2].allowed = &capabilities.InviteMembers
	permissions[3].allowed = &capabilities.ManageGoals
	permissions[4].allowed = &capabilities.ManageLifecycle
	permissions[5].allowed = &capabilities.ManageVisibility
	permissions[6].allowed = &capabilities.TransferOwnership
	permissions[7].allowed = &capabilities.LeavePath
	for _, permission := range permissions {
		allowed, err := s.Authorizer.Check(ctx, "path", string(entity.ID), permission.name, userID)
		if err != nil {
			return Capabilities{}, err
		}
		*permission.allowed = allowed
	}
	capabilities.ManageMembers = capabilities.InviteMembers
	if entity.Archived() {
		capabilities.TrackTime = false
		capabilities.RenamePath = false
		capabilities.InviteMembers = false
		capabilities.ManageMembers = false
		capabilities.ManageGoals = false
		capabilities.ManageVisibility = false
		capabilities.TransferOwnership = false
		capabilities.LeavePath = false
	}
	return capabilities, nil
}
