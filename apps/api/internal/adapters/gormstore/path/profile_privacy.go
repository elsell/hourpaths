package pathstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	timerstore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/notificationtimer"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type privacyUserModel struct {
	ID                     string
	ProfileVisibility      identity.ProfileVisibility
	ProfilePrivacyRevision int64
}

func (privacyUserModel) TableName() string { return "user_models" }

type privacyMutationModel struct {
	UserID, IdempotencyKey string
	RequestHash            []byte
	Visibility             identity.ProfileVisibility
	Revision               int64
	AffectedPathIDs        []byte `gorm:"type:jsonb"`
	CreatedAt              time.Time
}

func (privacyMutationModel) TableName() string { return "user_profile_privacy_mutation_models" }

func lockProfilePaths(tx *gorm.DB, owner string) error {
	return progresslock.LockKey(tx, sociallock.ProfilePathsKey(owner))
}

func (r *Repository) GetOwnProfilePrivacy(ctx context.Context, owner string) (app.OwnProfilePrivacy, error) {
	if r == nil || r.DB == nil || owner == "" {
		return app.OwnProfilePrivacy{}, ports.ErrInvalidArgument
	}
	var row privacyUserModel
	err := r.DB.WithContext(ctx).Where("id = ? AND status = ?", owner, identity.StatusActive).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return app.OwnProfilePrivacy{}, ports.ErrNotFound
	}
	if err != nil {
		return app.OwnProfilePrivacy{}, err
	}
	return app.OwnProfilePrivacy{UserID: row.ID, Visibility: row.ProfileVisibility, Revision: row.ProfilePrivacyRevision}, nil
}

func (r *Repository) UpdateOwnProfilePrivacy(ctx context.Context, command app.ProfilePrivacyCommand) (app.ProfilePrivacyResult, error) {
	if r == nil || r.DB == nil || !validPrivacyCommand(command) {
		return app.ProfilePrivacyResult{}, ports.ErrInvalidArgument
	}
	var result app.ProfilePrivacyResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockProfilePaths(tx, command.ActorUserID); err != nil {
			return err
		}
		// Acquire all existing Path audience locks before the owner row, matching
		// individual visibility mutations and notification writers.
		var pathIDs []string
		if err := tx.Model(&model{}).Where("owner_user_id = ? AND visibility = 'public'", command.ActorUserID).Order("id").Pluck("id", &pathIDs).Error; err != nil {
			return err
		}
		for _, id := range pathIDs {
			if err := lockVisibilityNotificationWriters(tx, id); err != nil {
				return err
			}
		}
		var owner privacyUserModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", command.ActorUserID, identity.StatusActive).Take(&owner).Error; err != nil {
			return err
		}
		var replay privacyMutationModel
		err := tx.Where("user_id = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result.Profile = app.OwnProfilePrivacy{UserID: command.ActorUserID, Visibility: replay.Visibility, Revision: replay.Revision}
			return json.Unmarshal(replay.AffectedPathIDs, &result.AffectedPathIDs)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if owner.ProfilePrivacyRevision != command.Update.ExpectedRevision || owner.ProfileVisibility == command.Update.Visibility {
			return ports.ErrConflict
		}
		if identity.ValidateProfileVisibility(owner.ProfileVisibility) != nil {
			return ports.ErrUnavailable
		}
		result.AffectedPathIDs = []string{}
		if command.Update.Visibility == identity.ProfileVisibilityPrivate {
			for _, id := range pathIDs {
				var row model
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner_user_id = ?", id, command.ActorUserID).Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
					continue
				} else if err != nil {
					return err
				}
				if row.Visibility != "public" {
					continue
				}
				changedAt := command.ChangedAt
				if !changedAt.After(row.UpdatedAt) {
					changedAt = row.UpdatedAt.Add(time.Microsecond)
				}
				entity, err := toEntity(row)
				if err != nil {
					return err
				}
				entity.Visibility = "followers"
				entity.UpdatedAt = changedAt
				if err := tx.Model(&model{}).Where("id = ? AND owner_user_id = ?", id, command.ActorUserID).Updates(map[string]any{"visibility": "followers", "updated_at": changedAt}).Error; err != nil {
					return err
				}
				visibilityCommand := pathapp.SetVisibilityCommand{ActorUserID: command.ActorUserID, Path: entity, NewID: command.NewID, AuthorizationWorker: command.AuthorizationWorker, AuthorizationLease: command.AuthorizationLease}
				for _, change := range visibilityAuthorizationChanges(visibilityCommand, "public", "followers") {
					if err := tx.Create(&authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: changedAt}).Error; err != nil {
						return err
					}
				}
				if err := timerstore.Retire(tx, "", id, changedAt); err != nil {
					return err
				}
				if err := retireNotificationsOutsideVisibility(tx, id, command.ActorUserID, "followers", changedAt); err != nil {
					return err
				}
				pathAudit := command.Audit
				pathAudit.ID = command.NewID()
				pathAudit.Action = audit.PathVisibilityChanged
				pathAudit.TargetType = "path"
				pathAudit.TargetID = id
				pathAudit.OccurredAt = changedAt
				if err := tx.Create(fromAudit(pathAudit)).Error; err != nil {
					return err
				}
				result.AffectedPathIDs = append(result.AffectedPathIDs, id)
			}
		}
		updated := tx.Model(&privacyUserModel{}).Where("id = ? AND status = ? AND profile_privacy_revision = ?", command.ActorUserID, identity.StatusActive, command.Update.ExpectedRevision).Updates(map[string]any{"profile_visibility": command.Update.Visibility, "profile_privacy_revision": owner.ProfilePrivacyRevision + 1, "updated_at": command.ChangedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrConflict
		}
		result.Profile = app.OwnProfilePrivacy{UserID: command.ActorUserID, Visibility: command.Update.Visibility, Revision: owner.ProfilePrivacyRevision + 1}
		encoded, err := json.Marshal(result.AffectedPathIDs)
		if err != nil {
			return err
		}
		replay = privacyMutationModel{UserID: command.ActorUserID, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), Visibility: result.Profile.Visibility, Revision: result.Profile.Revision, AffectedPathIDs: encoded, CreatedAt: command.ChangedAt}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		return tx.Create(fromAudit(command.Audit)).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validPrivacyCommand(c app.ProfilePrivacyCommand) bool {
	return c.ActorUserID != "" && c.Update.Confirmed && c.Update.ExpectedRevision > 0 && identity.ValidateProfileVisibility(c.Update.Visibility) == nil && !c.ChangedAt.IsZero() && c.ChangedAt.Location() == time.UTC && c.NewID != nil && c.AuthorizationWorker != "" && c.AuthorizationLease > 0 && c.Idempotency.PrincipalID == c.ActorUserID && c.Idempotency.Operation == "account.profile.privacy" && len(c.Idempotency.Key) >= 16 && len(c.Idempotency.Key) <= 128 && len(c.Idempotency.RequestHash) == 32 && c.Audit.Valid() && c.Audit.ActorUserID == c.ActorUserID && c.Audit.OwnerUserID == c.ActorUserID && c.Audit.Action == audit.ResourceUpdated && c.Audit.TargetType == "user" && c.Audit.TargetID == c.ActorUserID && c.Audit.Outcome == audit.Succeeded && c.Audit.OccurredAt == c.ChangedAt
}
