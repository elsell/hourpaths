package gormstore

import (
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

// Preserve unfinished grants for surviving users when deleting transfer records,
// then append removals after all older resource changes in the existing outbox.
func queueAccountDeletionRelationships(tx *gorm.DB, command application.AccountDeletionCommand, owned []string) error {
	ownedSet := make(map[string]bool, len(owned))
	for _, id := range owned {
		ownedSet[id] = true
	}
	var changes []ports.RelationshipUpdate
	var batches []authorizationBatchOutboxModel
	if err := tx.Where("completed_at IS NULL AND (owner_user_id = ? OR actor_user_id = ? OR resource_id IN ? OR path_ownership_transfer_id IN (SELECT id FROM path_ownership_transfer_models WHERE initiator_user_id = ? OR recipient_user_id = ?))", command.UserID, command.UserID, owned, command.UserID, command.UserID).Order("created_at, id").Find(&batches).Error; err != nil {
		return err
	}
	for _, batch := range batches {
		updates, err := decodeRelationshipUpdates(batch.RelationshipUpdates)
		if err != nil {
			return err
		}
		for _, update := range updates {
			if update.SubjectID == command.UserID || ownedSet[update.ResourceID] {
				update.Operation = ports.AuthorizationDelete
			}
			changes = append(changes, update)
		}
	}
	remove := func(resourceType, resourceID, relation, subjectID string) {
		changes = append(changes, ports.RelationshipUpdate{ResourceType: resourceType, ResourceID: resourceID, Relation: relation, SubjectType: "user", SubjectID: subjectID, Operation: ports.AuthorizationDelete})
	}
	var previous []authorizationOutboxModel
	if err := tx.Where("(subject_type = 'user' AND subject_id = ?) OR (resource_type = 'user' AND resource_id = ?) OR (resource_type = 'path' AND resource_id IN ?)", command.UserID, command.UserID, owned).Find(&previous).Error; err != nil {
		return err
	}
	for _, row := range previous {
		remove(row.ResourceType, row.ResourceID, row.Relation, row.SubjectID)
	}
	var memberships []struct{ PathID, UserID string }
	if err := tx.Table("path_membership_models").Select("path_id,user_id").Where("user_id = ? OR path_id IN ?", command.UserID, owned).Order("path_id,user_id").Find(&memberships).Error; err != nil {
		return err
	}
	for _, member := range memberships {
		for _, role := range []string{"creator", "administrator", "participant", "supporter"} {
			remove("path", member.PathID, role, member.UserID)
		}
	}
	for _, pathID := range owned {
		remove("path", pathID, "creator", command.UserID)
		remove("path", pathID, "followers_owner", command.UserID)
		remove("path", pathID, "public_viewer", "*")
	}
	var followers []struct{ FollowerUserID, FollowingUserID string }
	if err := tx.Table("follow_models").Where("follower_user_id = ? OR following_user_id = ?", command.UserID, command.UserID).Find(&followers).Error; err != nil {
		return err
	}
	for _, follower := range followers {
		remove("user", follower.FollowingUserID, "follower", follower.FollowerUserID)
	}
	var resources []resourceModel
	if err := tx.Where("owner_user_id = ?", command.UserID).Find(&resources).Error; err != nil {
		return err
	}
	for _, resource := range resources {
		remove("resource", resource.Domain+"/"+resource.ID, "owner", command.UserID)
	}
	for _, change := range changes {
		row := authorizationOutboxModel{ID: command.NewID(), ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, Operation: change.Operation, OwnerUserID: command.UserID, ActorUserID: command.UserID, CreatedAt: command.DeletedAt}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}
