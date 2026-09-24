package spicedb

import (
	"testing"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestOwnershipTransferRelationshipsAreOneValidatedAtomicBatch(t *testing.T) {
	updates, err := relationshipUpdates([]ports.RelationshipUpdate{
		{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: "path-1", Relation: "creator", SubjectType: "user", SubjectID: "old"},
		{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: "path-1", Relation: "creator", SubjectType: "user", SubjectID: "new"},
		{Operation: ports.AuthorizationTouch, ResourceType: "path", ResourceID: "path-1", Relation: "administrator", SubjectType: "user", SubjectID: "old"},
		{Operation: ports.AuthorizationDelete, ResourceType: "path", ResourceID: "path-1", Relation: "administrator", SubjectType: "user", SubjectID: "new"},
	})
	if err != nil {
		t.Fatalf("relationshipUpdates() error = %v", err)
	}
	if len(updates) != 4 {
		t.Fatalf("relationshipUpdates() count = %d, want 4", len(updates))
	}
	operations := []v1.RelationshipUpdate_Operation{
		v1.RelationshipUpdate_OPERATION_DELETE,
		v1.RelationshipUpdate_OPERATION_TOUCH,
		v1.RelationshipUpdate_OPERATION_TOUCH,
		v1.RelationshipUpdate_OPERATION_DELETE,
	}
	for index, update := range updates {
		if update.Operation != operations[index] || update.Relationship.Resource.ObjectId != "path-1" {
			t.Fatalf("relationship update %d = %+v", index, update)
		}
	}
}

func TestRelationshipBatchRejectsEmptyMalformedAndUnsupportedUpdates(t *testing.T) {
	for name, updates := range map[string][]ports.RelationshipUpdate{
		"empty":                 nil,
		"missing field":         {{Operation: ports.AuthorizationTouch, ResourceType: "path"}},
		"unsupported operation": {{Operation: "replace", ResourceType: "path", ResourceID: "path-1", Relation: "creator", SubjectType: "user", SubjectID: "user-1"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := relationshipUpdates(updates); err == nil {
				t.Fatal("relationshipUpdates() accepted invalid batch")
			}
		})
	}
}
