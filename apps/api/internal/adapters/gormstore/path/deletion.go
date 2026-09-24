package pathstore

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) DeletionReplay(ctx context.Context, actor string, pathID domain.ID, idempotency ports.Idempotency) (bool, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(actor) != actor || actor == "" || pathID == "" ||
		idempotency.PrincipalID != actor || idempotency.Operation != application.DeletePathOperation ||
		strings.TrimSpace(idempotency.Key) == "" || len(idempotency.RequestHash) != 32 {
		return false, ports.ErrInvalidArgument
	}
	var reservation idempotencyModel
	err := r.DB.WithContext(ctx).Where("principal_id = ? AND operation = ? AND key = ?", actor, idempotency.Operation, idempotency.Key).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if reservation.ResourceID != string(pathID) || !bytes.Equal(reservation.RequestHash, idempotency.RequestHash) {
		return false, ports.ErrIdempotencyConflict
	}
	return true, nil
}

type deletionMemberRow struct{ UserID, Role string }
type deletionCreatorRow struct{ ID, Name, Username, DisplayName, Visibility string }

func (r *Repository) DeletePath(ctx context.Context, command application.DeletePathCommand) (application.DeletePathResult, error) {
	if !validDeletePathCommand(r, command) {
		return application.DeletePathResult{}, ports.ErrInvalidArgument
	}
	var result application.DeletePathResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{PrincipalID: command.Idempotency.PrincipalID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ResourceID: string(command.PathID), CreatedAt: command.DeletedAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existing idempotencyModel
			if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).First(&existing).Error; err != nil {
				return err
			}
			if existing.ResourceID != string(command.PathID) || !bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = application.DeletePathResult{PathID: command.PathID, Deleted: true, Replayed: true}
			return nil
		}

		var creator deletionCreatorRow
		if err := tx.Table("path_models AS path").Select("path.owner_user_id AS id, path.name, path.visibility, creator.username, creator.display_name").Joins("JOIN user_models AS creator ON creator.id = path.owner_user_id").Where("path.id = ?", command.PathID).Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "path"}}).First(&creator).Error; err != nil {
			return err
		}
		if creator.ID != command.ActorUserID || strings.TrimSpace(creator.Name) == "" || strings.TrimSpace(creator.Username) == "" || strings.TrimSpace(creator.DisplayName) == "" ||
			(creator.Visibility != "private" && creator.Visibility != "followers" && creator.Visibility != "public") {
			return ports.ErrNotFound
		}
		if creator.Name != command.ExpectedName {
			return ports.ErrConflict
		}
		var members []deletionMemberRow
		if err := tx.Table("path_membership_models").Select("user_id, role").Where("path_id = ?", command.PathID).Order("user_id").Find(&members).Error; err != nil {
			return err
		}
		memberIDs := map[string]struct{}{command.ActorUserID: {}}
		for _, member := range members {
			if strings.TrimSpace(member.UserID) == "" {
				return ports.ErrInvalidArgument
			}
			memberIDs[member.UserID] = struct{}{}
			if member.UserID == command.ActorUserID {
				continue
			}
			notificationID := command.NewID()
			row := map[string]any{
				"id": notificationID, "recipient_user_id": member.UserID, "actor_user_id": command.ActorUserID,
				"path_id": nil, "path_invitation_id": nil, "path_ownership_transfer_id": nil,
				"kind": string(application.NotificationPathDeleted), "presentation_class": string(application.NotificationInformational),
				"channel": invitationNotificationChannel, "offered_role": nil, "created_at": command.DeletedAt,
				"path_name_snapshot": creator.Name, "actor_username_snapshot": creator.Username, "actor_display_name_snapshot": creator.DisplayName,
			}
			if err := tx.Table("notification_models").Create(row).Error; err != nil {
				return err
			}
			if err := createNotificationPushDelivery(tx, notificationID, member.UserID, command.DeletedAt); err != nil {
				return err
			}
		}
		deleted := tx.Where("id = ? AND owner_user_id = ?", command.PathID, command.ActorUserID).Delete(&model{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return ports.ErrNotFound
		}

		roles := []string{"creator", "administrator", "participant", "supporter"}
		orderedMemberIDs := make([]string, 0, len(memberIDs))
		for userID := range memberIDs {
			orderedMemberIDs = append(orderedMemberIDs, userID)
		}
		sort.Strings(orderedMemberIDs)
		for _, userID := range orderedMemberIDs {
			for _, role := range roles {
				change := ports.AuthorizationChange{ID: command.NewID(), ResourceType: "path", ResourceID: string(command.PathID), Relation: role, SubjectType: "user", SubjectID: userID, OwnerUserID: command.ActorUserID, ActorUserID: command.ActorUserID, Operation: ports.AuthorizationDelete, LockedBy: command.AuthorizationWorker, Lease: command.AuthorizationLease}
				outbox := authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: command.DeletedAt}
				if err := tx.Create(&outbox).Error; err != nil {
					return err
				}
				if err := tx.Model(&outbox).Update("locked_until", gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", command.AuthorizationLease.Milliseconds())).Error; err != nil {
					return err
				}
				result.AuthorizationChanges = append(result.AuthorizationChanges, change)
			}
		}
		visibilityRelation, visibilitySubject := "", ""
		if creator.Visibility == "followers" {
			visibilityRelation, visibilitySubject = "followers_owner", command.ActorUserID
		} else if creator.Visibility == "public" {
			visibilityRelation, visibilitySubject = "public_viewer", "*"
		}
		if visibilityRelation != "" {
			change := ports.AuthorizationChange{ID: command.NewID(), ResourceType: "path", ResourceID: string(command.PathID), Relation: visibilityRelation, SubjectType: "user", SubjectID: visibilitySubject, OwnerUserID: command.ActorUserID, ActorUserID: command.ActorUserID, Operation: ports.AuthorizationDelete, LockedBy: command.AuthorizationWorker, Lease: command.AuthorizationLease}
			outbox := authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: command.DeletedAt}
			if err := tx.Create(&outbox).Error; err != nil {
				return err
			}
			if err := tx.Model(&outbox).Update("locked_until", gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", command.AuthorizationLease.Milliseconds())).Error; err != nil {
				return err
			}
			result.AuthorizationChanges = append(result.AuthorizationChanges, change)
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result.PathID, result.Deleted = command.PathID, true
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validDeletePathCommand(r *Repository, command application.DeletePathCommand) bool {
	entity := domain.Entity{ID: command.PathID, OwnerUserID: command.ActorUserID}
	return r != nil && r.DB != nil && strings.TrimSpace(command.ActorUserID) == command.ActorUserID && command.ActorUserID != "" && command.PathID != "" && strings.TrimSpace(command.ExpectedName) == command.ExpectedName && command.ExpectedName != "" &&
		!command.DeletedAt.IsZero() && command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == application.DeletePathOperation && strings.TrimSpace(command.Idempotency.Key) != "" && len(command.Idempotency.RequestHash) == 32 &&
		validEvent(command.Audit, audit.ResourceDeleted, entity) && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.DeletedAt) &&
		command.NewID != nil && strings.TrimSpace(command.AuthorizationWorker) != "" && command.AuthorizationLease > 0
}
