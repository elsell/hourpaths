package path

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type ID string
type Attributes struct {
	Name          string
	Visibility    string
	IntervalGoal  IntervalGoal
	OverallTarget OverallTarget
}
type Entity struct {
	ID          ID
	OwnerUserID string
	Attributes
	CreatedAt, UpdatedAt, ArchivedAt time.Time
}

func New(id ID, ownerUserID string, attributes Attributes) (Entity, error) {
	attributes.Name = strings.TrimSpace(attributes.Name)
	if !validName(attributes.Name) {
		return Entity{}, ErrInvalidFields
	}
	attributes.Visibility = strings.TrimSpace(attributes.Visibility)
	if attributes.Visibility == "" {
		return Entity{}, ErrInvalidFields
	}
	if !attributes.IntervalGoal.valid() || !attributes.OverallTarget.valid() {
		return Entity{}, ErrInvalidFields
	}
	if id == "" || strings.TrimSpace(ownerUserID) == "" {
		return Entity{}, ErrInvalidFields
	}
	return Entity{ID: id, OwnerUserID: ownerUserID, Attributes: attributes}, nil
}

func (entity Entity) ReconfigureGoals(intervalGoal IntervalGoal, overallTarget OverallTarget, updatedAt time.Time) (Entity, error) {
	if entity.Archived() {
		return Entity{}, ErrInvalidState
	}
	if entity.CreatedAt.IsZero() || updatedAt.IsZero() {
		return Entity{}, ErrInvalidFields
	}
	attributes := entity.Attributes
	attributes.IntervalGoal = intervalGoal
	attributes.OverallTarget = overallTarget
	reconfigured, err := New(entity.ID, entity.OwnerUserID, attributes)
	if err != nil {
		return Entity{}, err
	}
	reconfigured.CreatedAt = entity.CreatedAt
	reconfigured.UpdatedAt = updatedAt
	reconfigured.ArchivedAt = entity.ArchivedAt
	return reconfigured, nil
}

func (entity Entity) Rename(name string, updatedAt time.Time) (Entity, error) {
	if entity.Archived() {
		return Entity{}, ErrInvalidState
	}
	if entity.CreatedAt.IsZero() || updatedAt.IsZero() || updatedAt.Before(entity.UpdatedAt) {
		return Entity{}, ErrInvalidFields
	}
	attributes := entity.Attributes
	attributes.Name = name
	renamed, err := New(entity.ID, entity.OwnerUserID, attributes)
	if err != nil {
		return Entity{}, err
	}
	renamed.CreatedAt = entity.CreatedAt
	renamed.UpdatedAt = updatedAt
	return renamed, nil
}

func (entity Entity) SetVisibility(visibility string, updatedAt time.Time) (Entity, error) {
	if entity.Archived() || entity.CreatedAt.IsZero() || updatedAt.IsZero() || updatedAt.Before(entity.UpdatedAt) {
		return Entity{}, ErrInvalidState
	}
	attributes := entity.Attributes
	attributes.Visibility = visibility
	updated, err := New(entity.ID, entity.OwnerUserID, attributes)
	if err != nil || (visibility != "private" && visibility != "followers" && visibility != "public") {
		return Entity{}, ErrInvalidFields
	}
	updated.CreatedAt = entity.CreatedAt
	updated.UpdatedAt = updatedAt
	return updated, nil
}

func (entity Entity) Archive(archivedAt time.Time) (Entity, error) {
	if entity.Archived() || entity.CreatedAt.IsZero() || archivedAt.IsZero() || archivedAt.Before(entity.UpdatedAt) {
		return Entity{}, ErrInvalidState
	}
	entity.ArchivedAt = archivedAt
	entity.UpdatedAt = archivedAt
	return entity, nil
}

func (entity Entity) Unarchive(unarchivedAt time.Time) (Entity, error) {
	if !entity.Archived() || unarchivedAt.IsZero() || unarchivedAt.Before(entity.ArchivedAt) || unarchivedAt.Before(entity.UpdatedAt) {
		return Entity{}, ErrInvalidState
	}
	entity.ArchivedAt = time.Time{}
	entity.UpdatedAt = unarchivedAt
	return entity, nil
}

func (entity Entity) Archived() bool { return !entity.ArchivedAt.IsZero() }

func (entity Entity) GoalConfiguration() GoalConfiguration {
	return GoalConfiguration{IntervalGoal: entity.IntervalGoal, OverallTarget: entity.OverallTarget}
}

func validName(name string) bool {
	if !utf8.ValidString(name) {
		return false
	}
	characterCount := utf8.RuneCountInString(name)
	if characterCount < 1 || characterCount > 100 {
		return false
	}
	for _, character := range name {
		if !unicode.IsPrint(character) {
			return false
		}
	}
	return true
}
