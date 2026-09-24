package spicedb

import (
	"context"
	"errors"
	"strings"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func (a *Authorizer) WriteRelationships(ctx context.Context, changes []ports.RelationshipUpdate) error {
	updates, err := relationshipUpdates(changes)
	if err != nil {
		return err
	}
	_, err = a.client.WriteRelationships(ctx, &v1.WriteRelationshipsRequest{Updates: updates})
	return err
}

func relationshipUpdates(changes []ports.RelationshipUpdate) ([]*v1.RelationshipUpdate, error) {
	if len(changes) == 0 || len(changes) > 100 {
		return nil, errors.New("relationship update batch is invalid")
	}
	updates := make([]*v1.RelationshipUpdate, 0, len(changes))
	for _, change := range changes {
		if strings.TrimSpace(change.ResourceType) == "" ||
			strings.TrimSpace(change.ResourceID) == "" ||
			strings.TrimSpace(change.Relation) == "" ||
			strings.TrimSpace(change.SubjectType) == "" ||
			strings.TrimSpace(change.SubjectID) == "" {
			return nil, errors.New("relationship update batch is invalid")
		}
		operation := v1.RelationshipUpdate_OPERATION_UNSPECIFIED
		switch change.Operation {
		case ports.AuthorizationTouch:
			operation = v1.RelationshipUpdate_OPERATION_TOUCH
		case ports.AuthorizationDelete:
			operation = v1.RelationshipUpdate_OPERATION_DELETE
		default:
			return nil, errors.New("relationship update batch is invalid")
		}
		updates = append(updates, &v1.RelationshipUpdate{
			Operation: operation,
			Relationship: &v1.Relationship{
				Resource: &v1.ObjectReference{ObjectType: change.ResourceType, ObjectId: change.ResourceID},
				Relation: change.Relation,
				Subject:  &v1.SubjectReference{Object: &v1.ObjectReference{ObjectType: change.SubjectType, ObjectId: change.SubjectID}},
			},
		})
	}
	return updates, nil
}

var _ ports.RelationshipBatchWriter = (*Authorizer)(nil)
