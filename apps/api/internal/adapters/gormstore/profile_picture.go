package gormstore

import (
	"bytes"
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type profilePictureModel struct {
	ID, UserID string
	JPEG       []byte
	CreatedAt  time.Time
}

func (profilePictureModel) TableName() string { return "user_profile_picture_models" }

type profilePictureMutationModel struct {
	UserID, IdempotencyKey string
	RequestHash            []byte
	AssetID, URL           string
	Revision               int64
	CreatedAt              time.Time
}

func (profilePictureMutationModel) TableName() string { return "user_profile_picture_mutation_models" }
func (s *Store) GetOwnPicture(ctx context.Context, owner string) (app.OwnPicture, error) {
	if s == nil || s.DB == nil || owner == "" {
		return app.OwnPicture{}, ports.ErrInvalidArgument
	}
	var result struct {
		ID, AssetID, URL string
		PictureRevision  int64
	}
	err := s.DB.WithContext(ctx).Table("user_models u").Select("u.id,u.picture_revision,COALESCE(u.profile_picture_url,'') AS url,COALESCE(p.id::text,'') AS asset_id").Joins("LEFT JOIN user_profile_picture_models p ON p.user_id=u.id").Where("u.id=? AND u.status=?", owner, identity.StatusActive).Take(&result).Error
	return app.OwnPicture{Owner: result.ID, AssetID: result.AssetID, URL: result.URL, Revision: result.PictureRevision}, classifyProfileError(err)
}
func (s *Store) ReadPublicPicture(ctx context.Context, id string) ([]byte, error) {
	if s == nil || s.DB == nil {
		return nil, ports.ErrUnavailable
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, ports.ErrNotFound
	}
	var row profilePictureModel
	err := s.DB.WithContext(ctx).Table("user_profile_picture_models p").Select("p.*").Joins("JOIN user_models u ON u.id=p.user_id").Where("p.id=? AND u.status=? AND u.profile_picture_url IS NOT NULL", id, identity.StatusActive).Take(&row).Error
	return row.JPEG, classifyProfileError(err)
}
func (s *Store) WriteOwnPicture(ctx context.Context, c app.PictureWriteCommand) (app.OwnPicture, error) {
	if s == nil || s.DB == nil || !validPictureWrite(c) {
		return app.OwnPicture{}, ports.ErrInvalidArgument
	}
	var result app.OwnPicture
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var owner userModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=? AND status=?", c.Owner, identity.StatusActive).Take(&owner).Error; err != nil {
			return err
		}
		var replay profilePictureMutationModel
		err := tx.Where("user_id=? AND idempotency_key=?", c.Owner, c.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, c.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = app.OwnPicture{Owner: c.Owner, AssetID: replay.AssetID, URL: replay.URL, Revision: replay.Revision}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if owner.PictureRevision != c.ExpectedRevision {
			return ports.ErrConflict
		}
		if err = tx.Where("user_id=?", c.Owner).Delete(&profilePictureModel{}).Error; err != nil {
			return err
		}
		if len(c.JPEG) > 0 {
			if err = tx.Create(&profilePictureModel{ID: c.AssetID, UserID: c.Owner, JPEG: c.JPEG, CreatedAt: c.ChangedAt}).Error; err != nil {
				return err
			}
		}
		var location *string
		if c.URL != "" {
			location = &c.URL
		}
		if err = tx.Model(&userModel{}).Where("id=?", c.Owner).Updates(map[string]any{"profile_picture_url": location, "picture_revision": owner.PictureRevision + 1, "updated_at": c.ChangedAt}).Error; err != nil {
			return err
		}
		replay = profilePictureMutationModel{UserID: c.Owner, IdempotencyKey: c.Idempotency.Key, RequestHash: c.Idempotency.RequestHash, AssetID: c.AssetID, URL: c.URL, Revision: owner.PictureRevision + 1, CreatedAt: c.ChangedAt}
		if err = tx.Create(&replay).Error; err != nil {
			return err
		}
		if err = appendAuditEvent(tx, c.Audit); err != nil {
			return err
		}
		result = app.OwnPicture{Owner: c.Owner, AssetID: c.AssetID, URL: c.URL, Revision: replay.Revision}
		return nil
	})
	return result, classifyProfileError(err)
}
func validPictureWrite(c app.PictureWriteCommand) bool {
	imageValid := len(c.JPEG) == 0 && c.AssetID == "" && c.URL == ""
	if len(c.JPEG) >= 4 && len(c.JPEG) <= 1048576 && c.JPEG[0] == 0xff && c.JPEG[1] == 0xd8 && c.JPEG[len(c.JPEG)-2] == 0xff && c.JPEG[len(c.JPEG)-1] == 0xd9 && c.URL != "" {
		_, err := uuid.Parse(c.AssetID)
		imageValid = err == nil
	}
	return imageValid && c.Owner != "" && c.ExpectedRevision > 0 && !c.ChangedAt.IsZero() && c.ChangedAt.Location() == time.UTC &&
		c.Idempotency.PrincipalID == c.Owner && c.Idempotency.Operation == "account.profile.picture.update" && len(c.Idempotency.Key) >= 16 && len(c.Idempotency.Key) <= 128 && len(c.Idempotency.RequestHash) == 32 &&
		c.Audit.Valid() && c.Audit.OwnerUserID == c.Owner && c.Audit.ActorUserID == c.Owner && c.Audit.Action == audit.ResourceUpdated && c.Audit.TargetType == "user" && c.Audit.TargetID == c.Owner && c.Audit.Outcome == audit.Succeeded && c.Audit.OccurredAt == c.ChangedAt
}
