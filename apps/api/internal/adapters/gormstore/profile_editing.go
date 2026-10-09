package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type profileMutationModel struct {
	UserID, IdempotencyKey             string
	RequestHash                        []byte
	Username, DisplayName, Description string
	Revision                           int64
	CreatedAt                          time.Time
}

func (profileMutationModel) TableName() string { return "user_profile_mutation_models" }

func ownProfileFromRow(row userModel) app.OwnProfile {
	profile := app.OwnProfile{UserID: row.ID, Revision: row.ProfileRevision, Text: identity.ProfileText{DisplayName: row.DisplayName}}
	if row.Username != nil {
		profile.Text.Username = *row.Username
	}
	if row.Description != nil {
		profile.Text.Description = *row.Description
	}
	return profile
}

func (s *Store) GetOwnProfile(ctx context.Context, userID string) (app.OwnProfile, error) {
	if s == nil || s.DB == nil || userID == "" {
		return app.OwnProfile{}, ports.ErrInvalidArgument
	}
	var row userModel
	err := s.DB.WithContext(ctx).Where("id = ? AND status = ?", userID, identity.StatusActive).Take(&row).Error
	if err != nil {
		return app.OwnProfile{}, classifyProfileError(err)
	}
	return ownProfileFromRow(row), nil
}

func (s *Store) UpdateOwnProfile(ctx context.Context, command app.ProfileUpdateCommand) (app.OwnProfile, error) {
	if s == nil || s.DB == nil || !validProfileCommand(command) {
		return app.OwnProfile{}, ports.ErrInvalidArgument
	}
	var result app.OwnProfile
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row userModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND status = ?", command.ActorUserID, identity.StatusActive).Take(&row).Error; err != nil {
			return err
		}
		var replay profileMutationModel
		err := tx.Where("user_id = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = app.OwnProfile{UserID: command.ActorUserID, Revision: replay.Revision, Text: identity.ProfileText{Username: replay.Username, DisplayName: replay.DisplayName, Description: replay.Description}}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.ProfileRevision != command.Update.ExpectedRevision {
			return ports.ErrConflict
		}
		text := command.Update.Text
		var description *string
		if text.Description != "" {
			description = &text.Description
		}
		updated := tx.Model(&userModel{}).Where("id = ? AND status = ? AND profile_revision = ?", command.ActorUserID, identity.StatusActive, command.Update.ExpectedRevision).Updates(map[string]any{"username": text.Username, "display_name": text.DisplayName, "description": description, "profile_revision": row.ProfileRevision + 1, "updated_at": command.ChangedAt})
		if errors.Is(updated.Error, gorm.ErrDuplicatedKey) {
			return ports.ErrUsernameUnavailable
		}
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrConflict
		}
		replay = profileMutationModel{UserID: command.ActorUserID, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), Username: text.Username, DisplayName: text.DisplayName, Description: text.Description, Revision: row.ProfileRevision + 1, CreatedAt: command.ChangedAt}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result = app.OwnProfile{UserID: command.ActorUserID, Text: text, Revision: replay.Revision}
		return nil
	})
	return result, classifyProfileError(err)
}

func validProfileCommand(command app.ProfileUpdateCommand) bool {
	text, err := identity.NormalizeProfileText(command.Update.Text)
	return err == nil && text == command.Update.Text && command.ActorUserID != "" && command.Update.ExpectedRevision > 0 &&
		!command.ChangedAt.IsZero() && command.ChangedAt.Location() == time.UTC &&
		command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == "account.profile.update" &&
		len(command.Idempotency.Key) >= 16 && len(command.Idempotency.Key) <= 128 && len(command.Idempotency.RequestHash) == 32 &&
		command.Audit.Valid() && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OwnerUserID == command.ActorUserID &&
		command.Audit.Action == audit.ResourceUpdated && command.Audit.TargetType == "user" && command.Audit.TargetID == command.ActorUserID &&
		command.Audit.Outcome == audit.Succeeded && command.Audit.OccurredAt == command.ChangedAt
}
func classifyProfileError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrConflict), errors.Is(err, ports.ErrIdempotencyConflict):
		return err
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ports.ErrNotFound
	default:
		return fmt.Errorf("profile persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
