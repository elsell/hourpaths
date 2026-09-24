package pathstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) SetVisibility(ctx context.Context, command application.SetVisibilityCommand) (application.SetVisibilityResult, error) {
	if !validSetVisibilityCommand(r, command) {
		return application.SetVisibilityResult{}, ports.ErrInvalidArgument
	}
	var result application.SetVisibilityResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockVisibilityNotificationWriters(tx, string(command.Path.ID)); err != nil {
			return err
		}
		reservation := idempotencyModel{PrincipalID: command.Idempotency.PrincipalID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ResourceID: string(command.Path.ID), CreatedAt: command.ChangedAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		replayed := created.RowsAffected == 0
		if replayed {
			var existing idempotencyModel
			if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", command.Idempotency.PrincipalID, command.Idempotency.Operation, command.Idempotency.Key).First(&existing).Error; err != nil {
				return err
			}
			if existing.ResourceID != string(command.Path.ID) || !bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			var stored visibilityReplayModel
			if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", command.Idempotency.PrincipalID, command.Idempotency.Operation, command.Idempotency.Key).First(&stored).Error; err != nil {
				return err
			}
			var original domain.Entity
			if err := json.Unmarshal(stored.Result, &original); err != nil || !validEntityForPersistence(original) || original.ID != command.Path.ID || original.Visibility != command.Path.Visibility {
				return ports.ErrIdempotencyConflict
			}
			result = application.SetVisibilityResult{Path: original, Replayed: true}
			return nil
		}
		var owner struct {
			ProfileVisibility *identity.ProfileVisibility
			Status            identity.Status
		}
		if err := tx.Table("user_models").Select("profile_visibility, status").Where("id = ?", command.ActorUserID).Clauses(clause.Locking{Strength: "UPDATE"}).Take(&owner).Error; err != nil {
			return err
		}
		if owner.ProfileVisibility == nil || identity.ValidateProfileVisibility(*owner.ProfileVisibility) != nil || owner.Status != identity.StatusActive {
			return errInvalidPersistedPath
		}
		if command.Path.Visibility == "public" && *owner.ProfileVisibility == identity.ProfileVisibilityPrivate {
			return ports.ErrInvalidArgument
		}
		row, err := goalUpdatePathForActor(tx, command.ActorUserID, command.Path.ID)
		if err != nil {
			return err
		}
		current, err := toEntity(row)
		if err != nil {
			return err
		}
		if current.Archived() {
			return ports.ErrConflict
		}
		if current.Visibility != command.ExpectedVisibility {
			return ports.ErrConflict
		}
		if !sameVisibilityIdentity(current, command.Path) || !command.Path.UpdatedAt.Equal(command.ChangedAt) || !command.ChangedAt.After(current.UpdatedAt) {
			return ports.ErrInvalidArgument
		}
		updated := tx.Model(&model{}).Where("id = ? AND owner_user_id = ? AND archived_at IS NULL AND visibility = ?", command.Path.ID, command.ActorUserID, command.ExpectedVisibility).Updates(map[string]any{"visibility": command.Path.Visibility, "updated_at": command.ChangedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrConflict
		}
		changes := visibilityAuthorizationChanges(command, current.Visibility, command.Path.Visibility)
		for _, change := range changes {
			if err := tx.Create(&authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: command.ChangedAt}).Error; err != nil {
				return err
			}
		}
		if visibilityRank(command.Path.Visibility) > visibilityRank(current.Visibility) {
			if err := createVisibilityNotifications(tx, command); err != nil {
				return err
			}
		} else if err := retireNotificationsOutsideVisibility(tx, string(command.Path.ID), command.ActorUserID, command.Path.Visibility, command.ChangedAt); err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		encoded, err := json.Marshal(command.Path)
		if err != nil {
			return err
		}
		if err := tx.Create(&visibilityReplayModel{PrincipalID: command.Idempotency.PrincipalID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key, Result: encoded, CreatedAt: command.ChangedAt}).Error; err != nil {
			return err
		}
		result = application.SetVisibilityResult{Path: command.Path, AuthorizationChanges: changes}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func (r *Repository) SetVisibilityReplay(ctx context.Context, actor string, pathID domain.ID, idempotency ports.Idempotency) (*application.SetVisibilityResult, error) {
	if r == nil || r.DB == nil || strings.TrimSpace(actor) != actor || actor == "" || pathID == "" || idempotency.PrincipalID != actor || idempotency.Operation != application.SetVisibilityOperation || idempotency.Key == "" || len(idempotency.RequestHash) != 32 {
		return nil, ports.ErrInvalidArgument
	}
	var reservation idempotencyModel
	err := r.DB.WithContext(ctx).Where("principal_id = ? AND operation = ? AND key = ?", actor, idempotency.Operation, idempotency.Key).First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if reservation.ResourceID != string(pathID) || !bytes.Equal(reservation.RequestHash, idempotency.RequestHash) {
		return nil, ports.ErrIdempotencyConflict
	}
	var stored visibilityReplayModel
	if err := r.DB.WithContext(ctx).Where("principal_id = ? AND operation = ? AND key = ?", actor, idempotency.Operation, idempotency.Key).First(&stored).Error; err != nil {
		return nil, err
	}
	var original domain.Entity
	if err := json.Unmarshal(stored.Result, &original); err != nil || !validEntityForPersistence(original) || original.ID != pathID || original.OwnerUserID != actor {
		return nil, ports.ErrIdempotencyConflict
	}
	result := application.SetVisibilityResult{Path: original, Replayed: true}
	return &result, nil
}

type visibilityReplayModel struct {
	PrincipalID string `gorm:"primaryKey"`
	Operation   string `gorm:"primaryKey"`
	Key         string `gorm:"primaryKey"`
	Result      []byte
	CreatedAt   time.Time
}

func (visibilityReplayModel) TableName() string { return "path_visibility_replay_models" }

func visibilityAuthorizationChanges(command application.SetVisibilityCommand, oldVisibility, newVisibility string) []ports.AuthorizationChange {
	changes := make([]ports.AuthorizationChange, 0, 2)
	appendChange := func(visibility string, operation ports.AuthorizationOperation) {
		relation, subject := visibilityRelation(visibility, command.ActorUserID)
		if relation == "" {
			return
		}
		changes = append(changes, ports.AuthorizationChange{ID: command.NewID(), ResourceType: "path", ResourceID: string(command.Path.ID), Relation: relation, SubjectType: "user", SubjectID: subject, OwnerUserID: command.ActorUserID, ActorUserID: command.ActorUserID, Operation: operation, LockedBy: command.AuthorizationWorker, Lease: command.AuthorizationLease})
	}
	appendChange(oldVisibility, ports.AuthorizationDelete)
	appendChange(newVisibility, ports.AuthorizationTouch)
	return changes
}
func visibilityRelation(visibility, owner string) (string, string) {
	if visibility == "followers" {
		return "followers_owner", owner
	}
	if visibility == "public" {
		return "public_viewer", "*"
	}
	return "", ""
}
func visibilityRank(value string) int {
	switch value {
	case "private":
		return 0
	case "followers":
		return 1
	case "public":
		return 2
	}
	return -1
}

func createVisibilityNotifications(tx *gorm.DB, command application.SetVisibilityCommand) error {
	var recipients []string
	if err := tx.Table("path_membership_models AS membership").Select("membership.user_id").Joins("JOIN user_models AS member ON member.id = membership.user_id").Where("membership.path_id = ? AND membership.role IN ('administrator','participant') AND membership.user_id <> ? AND member.profile_visibility = ? AND member.status = ?", command.Path.ID, command.ActorUserID, "private", "active").Order("membership.user_id").Scan(&recipients).Error; err != nil {
		return err
	}
	for _, recipient := range recipients {
		id := command.NewID()
		row := map[string]any{"id": id, "recipient_user_id": recipient, "actor_user_id": command.ActorUserID, "path_id": string(command.Path.ID), "kind": string(application.NotificationPathVisibilityChanged), "presentation_class": string(application.NotificationInformational), "channel": invitationNotificationChannel, "path_visibility": command.Path.Visibility, "created_at": command.ChangedAt}
		if err := tx.Table("notification_models").Create(row).Error; err != nil {
			return err
		}
		if err := createNotificationPushDelivery(tx, id, recipient, command.ChangedAt); err != nil {
			return err
		}
	}
	return nil
}

func validSetVisibilityCommand(r *Repository, command application.SetVisibilityCommand) bool {
	return r != nil && r.DB != nil && strings.TrimSpace(command.ActorUserID) == command.ActorUserID && command.ActorUserID != "" && command.ExpectedVisibility != command.Path.Visibility && visibilityRank(command.ExpectedVisibility) >= 0 && visibilityRank(command.Path.Visibility) >= 0 && validEntityForPersistence(command.Path) && !command.Path.Archived() && command.Path.OwnerUserID == command.ActorUserID && command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == application.SetVisibilityOperation && strings.TrimSpace(command.Idempotency.Key) != "" && len(command.Idempotency.RequestHash) == 32 && validEvent(command.Audit, audit.PathVisibilityChanged, command.Path) && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.ChangedAt) && command.NewID != nil && command.AuthorizationWorker != "" && command.AuthorizationLease > 0
}

func validVisibilityNotificationRow(row invitationNotificationRow, kind application.InvitationNotificationKind, presentation application.NotificationPresentation) bool {
	return kind == application.NotificationPathVisibilityChanged && presentation == application.NotificationInformational &&
		row.Channel == "path_access" && row.PathID != "" && row.RecipientUserID != row.ActorUserID &&
		(row.PathVisibility == "private" || row.PathVisibility == "followers" || row.PathVisibility == "public") &&
		row.PathInvitationID == "" && row.PathOwnershipTransferID == "" && row.FollowRequestID == "" &&
		row.FollowSubjectUserID == "" && row.SocialFeedEventID == "" && row.ReactionType == "" && row.CommentID == "" && row.OfferedRole == ""
}

func sameVisibilityIdentity(current, replacement domain.Entity) bool {
	return current.ID == replacement.ID && current.OwnerUserID == replacement.OwnerUserID && current.Name == replacement.Name && current.IntervalGoal == replacement.IntervalGoal && current.OverallTarget == replacement.OverallTarget && current.CreatedAt.Equal(replacement.CreatedAt)
}
