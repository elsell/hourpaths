package gormstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
)

type socialCommentModel struct {
	ID, SocialFeedEventID, AuthorUserID, Body string
	Version                                   int64
	CreatedAt, UpdatedAt                      time.Time
}

func (socialCommentModel) TableName() string { return "social_practice_comment_models" }

type socialCommentRevisionModel struct {
	CommentID, Body string
	Version         int64
	ChangedAt       time.Time
}

func (socialCommentRevisionModel) TableName() string {
	return "social_practice_comment_revision_models"
}

type socialCommentReplayModel struct {
	ActorUserID, Operation, IdempotencyKey string
	RequestHash                            []byte
	CommentID, SocialFeedEventID           string
	AuthorUserID, ResultBody               string
	ResultVersion                          int64
	ResultCreatedAt, ResultUpdatedAt       time.Time
	ResultDeleted                          bool
	CreatedAt                              time.Time
}

func (socialCommentReplayModel) TableName() string { return "social_practice_comment_replay_models" }

type socialCommentNotificationModel struct{ ID string }

func (socialCommentNotificationModel) TableName() string { return "notification_models" }

func (repository *SocialFeedRepository) FindPracticeCommentReplay(ctx context.Context, idempotency ports.Idempotency) (socialapp.PracticeCommentReplay, bool, error) {
	if repository == nil || repository.db == nil || !validCommentIdempotency(idempotency) {
		return socialapp.PracticeCommentReplay{}, false, ports.ErrInvalidArgument
	}
	replay, found, err := readSocialCommentReplay(repository.db.WithContext(ctx), idempotency)
	if err != nil {
		return socialapp.PracticeCommentReplay{}, false, classifySocialCommentError(err)
	}
	if !found {
		return socialapp.PracticeCommentReplay{}, false, nil
	}
	return mapSocialCommentReplay(replay), true, nil
}

func (repository *SocialFeedRepository) ResolvePracticeCommentTarget(ctx context.Context, viewer, eventID string) (socialapp.PracticeCommentTarget, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(viewer) || !validOpaquePersistenceID(eventID) {
		return socialapp.PracticeCommentTarget{}, ports.ErrInvalidArgument
	}
	target, err := resolvePracticeCommentTarget(repository.db.WithContext(ctx), viewer, eventID)
	return target, classifySocialCommentError(err)
}

func (repository *SocialFeedRepository) ResolvePracticeComment(ctx context.Context, viewer, eventID, commentID string) (socialapp.ResolvedPracticeComment, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(viewer) || !validOpaquePersistenceID(eventID) || !validOpaquePersistenceID(commentID) {
		return socialapp.ResolvedPracticeComment{}, ports.ErrInvalidArgument
	}
	resolved, err := resolvePracticeComment(repository.db.WithContext(ctx), viewer, eventID, commentID)
	return resolved, classifySocialCommentError(err)
}

func (repository *SocialFeedRepository) ListPracticeComments(ctx context.Context, viewer, eventID string, request socialapp.CommentPageRequest) (socialapp.CommentPage, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(viewer) || !validOpaquePersistenceID(eventID) || !validCommentPageRequest(request) {
		return socialapp.CommentPage{}, ports.ErrInvalidArgument
	}
	db := repository.db.WithContext(ctx)
	target, err := resolvePracticeCommentTarget(db, viewer, eventID)
	if err != nil {
		return socialapp.CommentPage{}, classifySocialCommentError(err)
	}
	commentsEnabled, err := socialInteractionEnabled(db, target.OwnerUserID, "comments_enabled")
	if err != nil {
		return socialapp.CommentPage{}, classifySocialCommentError(err)
	}
	if !commentsEnabled {
		return socialapp.CommentPage{Items: []socialapp.PracticeCommentItem{}}, nil
	}
	type commentRow struct {
		ID, SocialFeedEventID, AuthorUserID, Body string
		Version                                   int64
		CreatedAt, UpdatedAt                      time.Time
		ProfileID, Username, DisplayName          string
		ProfilePictureURL, Description            string
		FollowerCount, FollowingCount             int64
		Relationship                              socialdomain.RelationshipState
		HeartCount                                int64
		HeartedByViewer                           bool
	}
	var rows []commentRow
	query := db.Table("social_practice_comment_models AS comment_row").Select(fmt.Sprintf(`comment_row.id, comment_row.social_feed_event_id, comment_row.author_user_id, comment_row.body, comment_row.version, comment_row.created_at, comment_row.updated_at,
u.id AS profile_id, u.username, u.display_name, COALESCE(u.profile_picture_url, '') AS profile_picture_url,
COALESCE(u.description, '') AS description, %s, %s,
(SELECT COUNT(*) FROM social_practice_comment_heart_models heart_count
 WHERE heart_count.comment_id = comment_row.id
   AND NOT EXISTS (SELECT 1 FROM block_models heart_count_block
     WHERE (heart_count_block.blocker_user_id = ? AND heart_count_block.blocked_user_id = heart_count.actor_user_id)
        OR (heart_count_block.blocker_user_id = heart_count.actor_user_id AND heart_count_block.blocked_user_id = ?))) AS heart_count,
EXISTS (SELECT 1 FROM social_practice_comment_heart_models viewer_heart WHERE viewer_heart.comment_id = comment_row.id AND viewer_heart.actor_user_id = ?) AS hearted_by_viewer`, socialCountsSQL, socialRelationshipSQL), viewer, viewer, viewer, viewer, viewer, viewer).
		Joins("JOIN user_models u ON u.id = comment_row.author_user_id AND u.status = 'active' AND u.username IS NOT NULL").
		Where("comment_row.social_feed_event_id = ? AND comment_row.created_at <= ?", eventID, request.Snapshot).
		Where(`NOT EXISTS (SELECT 1 FROM block_models comment_block
WHERE (comment_block.blocker_user_id = ? AND comment_block.blocked_user_id = comment_row.author_user_id)
   OR (comment_block.blocker_user_id = comment_row.author_user_id AND comment_block.blocked_user_id = ?))`, viewer, viewer)
	if request.AfterID != "" {
		query = query.Where("comment_row.created_at > ? OR (comment_row.created_at = ? AND comment_row.id > ?)", request.AfterCreated, request.AfterCreated, request.AfterID)
	}
	if err := query.Order("comment_row.created_at ASC, comment_row.id ASC").Limit(request.Limit + 1).Scan(&rows).Error; err != nil {
		return socialapp.CommentPage{}, classifySocialCommentError(err)
	}
	hasMore := len(rows) > request.Limit
	if hasMore {
		rows = rows[:request.Limit]
	}
	page := socialapp.CommentPage{Items: make([]socialapp.PracticeCommentItem, 0, len(rows)), HasMore: hasMore}
	for _, row := range rows {
		comment := mapSocialComment(socialCommentModel{ID: row.ID, SocialFeedEventID: row.SocialFeedEventID, AuthorUserID: row.AuthorUserID, Body: row.Body, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt})
		author := socialdomain.PublicProfile{ID: row.ProfileID, Username: row.Username, DisplayName: row.DisplayName, ProfilePictureURL: row.ProfilePictureURL, Description: row.Description, FollowerCount: row.FollowerCount, FollowingCount: row.FollowingCount, Relationship: row.Relationship}
		page.Items = append(page.Items, socialapp.PracticeCommentItem{Comment: comment, Author: author, HeartCount: row.HeartCount, HeartedByViewer: row.HeartedByViewer})
	}
	return page, nil
}

func (repository *SocialFeedRepository) ListCommentHistory(ctx context.Context, viewer, eventID, commentID string, request socialapp.CommentHistoryPageRequest) (socialapp.CommentHistoryPage, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(viewer) || !validOpaquePersistenceID(eventID) || !validOpaquePersistenceID(commentID) || !validCommentHistoryPageRequest(request) {
		return socialapp.CommentHistoryPage{}, ports.ErrInvalidArgument
	}
	db := repository.db.WithContext(ctx)
	resolved, err := resolvePracticeComment(db, viewer, eventID, commentID)
	if err != nil {
		return socialapp.CommentHistoryPage{}, classifySocialCommentError(err)
	}
	var revisions []socialCommentRevisionModel
	query := db.Where("comment_id = ? AND version > ? AND changed_at <= ?", commentID, request.AfterVersion, request.Snapshot).
		Order("version ASC").Limit(request.Limit + 1)
	if err := query.Find(&revisions).Error; err != nil {
		return socialapp.CommentHistoryPage{}, classifySocialCommentError(err)
	}
	versions := make([]socialdomain.CommentVersion, 0, len(revisions)+1)
	for _, revision := range revisions {
		versions = append(versions, socialdomain.CommentVersion{CommentID: revision.CommentID, Text: revision.Body, Version: revision.Version, CreatedAt: revision.ChangedAt.UTC()})
	}
	if resolved.Comment.Version > request.AfterVersion && !resolved.Comment.UpdatedAt.After(request.Snapshot) {
		versions = append(versions, socialdomain.CommentVersion{CommentID: commentID, Text: resolved.Comment.Text, Version: resolved.Comment.Version, CreatedAt: resolved.Comment.UpdatedAt})
	}
	sort.Slice(versions, func(left, right int) bool { return versions[left].Version < versions[right].Version })
	hasMore := len(versions) > request.Limit
	if hasMore {
		versions = versions[:request.Limit]
	}
	return socialapp.CommentHistoryPage{Versions: versions, HasMore: hasMore}, nil
}

func (repository *SocialFeedRepository) CreatePracticeComment(ctx context.Context, command socialapp.CommentCommand) (socialapp.CommentMutationResult, error) {
	if !validSocialCommentCommand(repository, command, socialapp.CreatePracticeCommentOperation) {
		return socialapp.CommentMutationResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.CommentMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialPathAudience(tx, command.Target.PathID); err != nil {
			return err
		}
		if err := lockSocialCommentIdempotency(tx, command.Idempotency); err != nil {
			return err
		}
		if err := lockSocialInteractionPairs(tx, command.ActorUserID, command.Target.EventID, ""); err != nil {
			return err
		}
		if err := lockSocialInteractionOwner(tx, command.Target.OwnerUserID); err != nil {
			return err
		}
		enabled, err := socialInteractionEnabled(tx, command.Target.OwnerUserID, "comments_enabled")
		if err != nil {
			return err
		}
		if !enabled {
			return ports.ErrNotFound
		}
		if replay, found, err := readSocialCommentReplay(tx, command.Idempotency); err != nil {
			return err
		} else if found {
			mapped := mapSocialCommentReplay(replay)
			result = socialapp.CommentMutationResult{Comment: mapped.Comment, Replayed: true}
			return nil
		}
		if err := lockSocialCommentTarget(tx, command.Target.EventID); err != nil {
			return err
		}
		target, err := resolvePracticeCommentTarget(tx, command.ActorUserID, command.Target.EventID)
		if err != nil {
			return err
		}
		if target != command.Target {
			return ports.ErrNotFound
		}
		comment := socialCommentModel{
			ID: socialCommentID(command.Idempotency), SocialFeedEventID: command.Target.EventID,
			AuthorUserID: command.ActorUserID, Body: command.Text, Version: 1,
			CreatedAt: command.OccurredAt, UpdatedAt: command.OccurredAt,
		}
		if err := tx.Create(&comment).Error; err != nil {
			return err
		}
		if command.NotifyEventOwner {
			if err := createCommentNotification(tx, command, comment.ID); err != nil {
				return err
			}
		}
		if err := createSocialCommentReplay(tx, command, comment, false); err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result.Comment = mapSocialComment(comment)
		return nil
	})
	return result, classifySocialCommentError(err)
}

func (repository *SocialFeedRepository) EditPracticeComment(ctx context.Context, command socialapp.CommentCommand) (socialapp.CommentMutationResult, error) {
	if !validSocialCommentCommand(repository, command, socialapp.EditPracticeCommentOperation) {
		return socialapp.CommentMutationResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.CommentMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialCommentIdempotency(tx, command.Idempotency); err != nil {
			return err
		}
		if err := lockSocialInteractionPairs(tx, command.ActorUserID, command.Target.EventID, command.CommentID); err != nil {
			return err
		}
		if replay, found, err := readSocialCommentReplay(tx, command.Idempotency); err != nil {
			return err
		} else if found {
			mapped := mapSocialCommentReplay(replay)
			result = socialapp.CommentMutationResult{Comment: mapped.Comment, Replayed: true}
			return nil
		}
		if err := lockSocialCommentTarget(tx, command.CommentID); err != nil {
			return err
		}
		resolved, err := resolvePracticeComment(tx, command.ActorUserID, command.Target.EventID, command.CommentID)
		if err != nil {
			return err
		}
		if resolved.Target != command.Target || resolved.Comment.AuthorID != command.ActorUserID {
			return ports.ErrNotFound
		}
		if resolved.Comment.Version != command.ExpectedVersion {
			return ports.ErrConflict
		}
		if command.OccurredAt.Before(resolved.Comment.UpdatedAt) {
			return ports.ErrConflict
		}
		revision := socialCommentRevisionModel{CommentID: command.CommentID, Body: resolved.Comment.Text, Version: resolved.Comment.Version, ChangedAt: resolved.Comment.UpdatedAt}
		if err := tx.Create(&revision).Error; err != nil {
			return err
		}
		updated := tx.Model(&socialCommentModel{}).Where("id = ? AND version = ?", command.CommentID, command.ExpectedVersion).
			Updates(map[string]any{"body": command.Text, "version": command.ExpectedVersion + 1, "updated_at": command.OccurredAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrConflict
		}
		comment := socialCommentModel{ID: command.CommentID, SocialFeedEventID: command.Target.EventID, AuthorUserID: command.ActorUserID, Body: command.Text, Version: command.ExpectedVersion + 1, CreatedAt: resolved.Comment.CreatedAt, UpdatedAt: command.OccurredAt}
		if err := createSocialCommentReplay(tx, command, comment, false); err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result.Comment = mapSocialComment(comment)
		return nil
	})
	return result, classifySocialCommentError(err)
}

func (repository *SocialFeedRepository) DeletePracticeComment(ctx context.Context, command socialapp.CommentCommand) (socialapp.CommentDeleteResult, error) {
	if !validSocialCommentCommand(repository, command, socialapp.DeletePracticeCommentOperation) {
		return socialapp.CommentDeleteResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.CommentDeleteResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialCommentIdempotency(tx, command.Idempotency); err != nil {
			return err
		}
		if replay, found, err := readSocialCommentReplay(tx, command.Idempotency); err != nil {
			return err
		} else if found {
			result = socialapp.CommentDeleteResult{CommentID: replay.CommentID, Deleted: replay.ResultDeleted, Replayed: true}
			return nil
		}
		if err := lockSocialInteractionPairs(tx, command.ActorUserID, command.Target.EventID, command.CommentID); err != nil {
			return err
		}
		if err := lockSocialCommentTarget(tx, command.CommentID); err != nil {
			return err
		}
		resolved, err := resolvePracticeComment(tx, command.ActorUserID, command.Target.EventID, command.CommentID)
		if err != nil {
			return err
		}
		if resolved.Target != command.Target || (resolved.Comment.AuthorID != command.ActorUserID && resolved.Target.OwnerUserID != command.ActorUserID) {
			return ports.ErrNotFound
		}
		deleted := tx.Where("id = ?", command.CommentID).Delete(&socialCommentModel{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		comment := socialCommentModel{ID: resolved.Comment.ID, SocialFeedEventID: resolved.Comment.EventID, AuthorUserID: resolved.Comment.AuthorID, Body: resolved.Comment.Text, Version: resolved.Comment.Version, CreatedAt: resolved.Comment.CreatedAt, UpdatedAt: resolved.Comment.UpdatedAt}
		if err := createSocialCommentReplay(tx, command, comment, true); err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result = socialapp.CommentDeleteResult{CommentID: command.CommentID, Deleted: true}
		return nil
	})
	return result, classifySocialCommentError(err)
}

func resolvePracticeCommentTarget(tx *gorm.DB, viewer, eventID string) (socialapp.PracticeCommentTarget, error) {
	target, err := resolvePracticeReactionTarget(tx, viewer, eventID)
	if err != nil {
		return socialapp.PracticeCommentTarget{}, err
	}
	return socialapp.PracticeCommentTarget{EventID: target.EventID, PathID: target.PathID, OwnerUserID: target.OwnerUserID}, nil
}

func resolvePracticeComment(tx *gorm.DB, viewer, eventID, commentID string) (socialapp.ResolvedPracticeComment, error) {
	target, err := resolvePracticeCommentTarget(tx, viewer, eventID)
	if err != nil {
		return socialapp.ResolvedPracticeComment{}, err
	}
	enabled, err := socialInteractionEnabled(tx, target.OwnerUserID, "comments_enabled")
	if err != nil {
		return socialapp.ResolvedPracticeComment{}, err
	}
	if !enabled {
		return socialapp.ResolvedPracticeComment{}, ports.ErrNotFound
	}
	var model socialCommentModel
	err = tx.Table("social_practice_comment_models AS comment_row").Select("comment_row.*").
		Joins("JOIN user_models author ON author.id = comment_row.author_user_id AND author.status = 'active'").
		Where("comment_row.id = ? AND comment_row.social_feed_event_id = ?", commentID, eventID).
		Where(`NOT EXISTS (SELECT 1 FROM block_models comment_block
WHERE (comment_block.blocker_user_id = ? AND comment_block.blocked_user_id = comment_row.author_user_id)
   OR (comment_block.blocker_user_id = comment_row.author_user_id AND comment_block.blocked_user_id = ?))`, viewer, viewer).
		Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return socialapp.ResolvedPracticeComment{}, ports.ErrNotFound
	}
	if err != nil {
		return socialapp.ResolvedPracticeComment{}, err
	}
	return socialapp.ResolvedPracticeComment{Target: target, Comment: mapSocialComment(model)}, nil
}

func readSocialCommentReplay(tx *gorm.DB, idempotency ports.Idempotency) (socialCommentReplayModel, bool, error) {
	var replay socialCommentReplayModel
	err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", idempotency.PrincipalID, idempotency.Operation, idempotency.Key).Take(&replay).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return replay, false, nil
	}
	if err != nil {
		return replay, false, err
	}
	if !bytes.Equal(replay.RequestHash, idempotency.RequestHash) {
		return replay, false, ports.ErrIdempotencyConflict
	}
	return replay, true, nil
}

func createSocialCommentReplay(tx *gorm.DB, command socialapp.CommentCommand, comment socialCommentModel, deleted bool) error {
	return tx.Create(&socialCommentReplayModel{
		ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key,
		RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), CommentID: comment.ID,
		SocialFeedEventID: comment.SocialFeedEventID, AuthorUserID: comment.AuthorUserID, ResultBody: comment.Body,
		ResultVersion: comment.Version, ResultCreatedAt: comment.CreatedAt, ResultUpdatedAt: comment.UpdatedAt,
		ResultDeleted: deleted, CreatedAt: command.OccurredAt,
	}).Error
}

func mapSocialComment(model socialCommentModel) socialdomain.Comment {
	return socialdomain.Comment{ID: model.ID, EventID: model.SocialFeedEventID, AuthorID: model.AuthorUserID, Text: model.Body, Version: model.Version, CreatedAt: model.CreatedAt.UTC(), UpdatedAt: model.UpdatedAt.UTC()}
}

func mapSocialCommentReplay(model socialCommentReplayModel) socialapp.PracticeCommentReplay {
	return socialapp.PracticeCommentReplay{
		ActorUserID: model.ActorUserID, EventID: model.SocialFeedEventID, Deleted: model.ResultDeleted,
		Comment: mapSocialComment(socialCommentModel{ID: model.CommentID, SocialFeedEventID: model.SocialFeedEventID, AuthorUserID: model.AuthorUserID, Body: model.ResultBody, Version: model.ResultVersion, CreatedAt: model.ResultCreatedAt, UpdatedAt: model.ResultUpdatedAt}),
	}
}

func socialCommentID(idempotency ports.Idempotency) string {
	digest := sha256.Sum256([]byte(idempotency.PrincipalID + "\x00" + idempotency.Operation + "\x00" + idempotency.Key))
	return "comment:" + hex.EncodeToString(digest[:16])
}

func lockSocialCommentIdempotency(tx *gorm.DB, idempotency ports.Idempotency) error {
	key := socialLockKey("social-practice-comment-idempotency", idempotency.PrincipalID, idempotency.Operation, idempotency.Key)
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock
}

func lockSocialCommentTarget(tx *gorm.DB, target string) error {
	key := socialLockKey("social-practice-comment-target", target)
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock
}

func createCommentNotification(tx *gorm.DB, command socialapp.CommentCommand, commentID string) error {
	digest := sha256.Sum256([]byte(commentID + "\x00" + command.ActorUserID + "\x00" + command.Idempotency.Key))
	notificationID := "comment:" + hex.EncodeToString(digest[:16])
	row := map[string]any{
		"id": notificationID, "recipient_user_id": command.Target.OwnerUserID, "actor_user_id": command.ActorUserID,
		"path_id": command.Target.PathID, "social_feed_event_id": command.Target.EventID, "comment_id": commentID,
		"kind": "practice_comment", "presentation_class": "informational", "channel": "comments", "created_at": command.NotificationEligibleAt,
	}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	return createSocialInteractionPush(tx, notificationID, command.Target.OwnerUserID, command.NotificationEligibleAt)
}

func createSocialInteractionPush(tx *gorm.DB, notificationID, recipientUserID string, eligibleAt time.Time) error {
	if err := tx.Table("notification_push_outbox_models").Create(map[string]any{"notification_id": notificationID, "created_at": eligibleAt}).Error; err != nil {
		return err
	}
	if err := tx.Exec(`INSERT INTO notification_push_delivery_models (
notification_id, installation_id, recipient_user_id, provider, platform, locale,
token_ciphertext, token_nonce, token_hash, available_at, created_at)
SELECT ?, installation.id, ?, installation.provider, installation.platform, installation.locale,
installation.token_ciphertext, installation.token_nonce, installation.token_hash, ?, ?
FROM push_installation_models installation
WHERE installation.owner_user_id = ? AND installation.deleted_at IS NULL
ON CONFLICT (notification_id, installation_id) DO NOTHING`, notificationID, recipientUserID, eligibleAt, eligibleAt, recipientUserID).Error; err != nil {
		return err
	}
	return tx.Exec(`UPDATE notification_push_outbox_models
SET suppressed_at = ?, failure_code = 'no_active_installation'
WHERE notification_id = ?
  AND NOT EXISTS (SELECT 1 FROM notification_push_delivery_models WHERE notification_id = ?)`, eligibleAt, notificationID, notificationID).Error
}

func validSocialCommentCommand(repository *SocialFeedRepository, command socialapp.CommentCommand, operation string) bool {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(command.ActorUserID) || !validOpaquePersistenceID(command.Target.EventID) ||
		!validOpaquePersistenceID(command.Target.PathID) || !validOpaquePersistenceID(command.Target.OwnerUserID) || !validCommentIdempotency(command.Idempotency) ||
		command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != operation || command.OccurredAt.IsZero() || command.OccurredAt.Location() != time.UTC {
		return false
	}
	action, targetID := audit.ResourceCreated, command.Target.EventID
	switch operation {
	case socialapp.CreatePracticeCommentOperation:
		normalized, err := socialdomain.NormalizeCommentText(command.Text)
		if err != nil || normalized != command.Text || command.CommentID != "" || command.ExpectedVersion != 0 ||
			!command.NotificationEligibleAt.Equal(command.OccurredAt.Add(5*time.Second)) || command.NotifyEventOwner != (command.ActorUserID != command.Target.OwnerUserID) {
			return false
		}
	case socialapp.EditPracticeCommentOperation:
		action, targetID = audit.ResourceUpdated, command.CommentID
		normalized, err := socialdomain.NormalizeCommentText(command.Text)
		if err != nil || normalized != command.Text || !validOpaquePersistenceID(command.CommentID) || command.ExpectedVersion < 1 || !command.NotificationEligibleAt.IsZero() || command.NotifyEventOwner {
			return false
		}
	case socialapp.DeletePracticeCommentOperation:
		action, targetID = audit.ResourceDeleted, command.CommentID
		if command.Text != "" || !validOpaquePersistenceID(command.CommentID) || command.ExpectedVersion != 0 || !command.NotificationEligibleAt.IsZero() || command.NotifyEventOwner {
			return false
		}
	default:
		return false
	}
	return validMutationAudit(command.Audit, action, "practice_comment", targetID, command.ActorUserID) && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.OccurredAt)
}

func validCommentIdempotency(value ports.Idempotency) bool {
	if !validOpaquePersistenceID(value.PrincipalID) || (value.Operation != socialapp.CreatePracticeCommentOperation && value.Operation != socialapp.EditPracticeCommentOperation && value.Operation != socialapp.DeletePracticeCommentOperation) ||
		len(value.Key) < 16 || len(value.Key) > 128 || strings.TrimSpace(value.Key) != value.Key || len(value.RequestHash) != sha256.Size {
		return false
	}
	for index := range len(value.Key) {
		if value.Key[index] < 0x20 || value.Key[index] > 0x7e {
			return false
		}
	}
	return true
}

func validCommentPageRequest(request socialapp.CommentPageRequest) bool {
	return request.Limit >= 1 && request.Limit <= 100 && !request.Snapshot.IsZero() && request.Snapshot.Location() == time.UTC &&
		(request.AfterID == "") == request.AfterCreated.IsZero() && (request.AfterID == "" || (validOpaquePersistenceID(request.AfterID) && request.AfterCreated.Location() == time.UTC && !request.AfterCreated.After(request.Snapshot)))
}

func validCommentHistoryPageRequest(request socialapp.CommentHistoryPageRequest) bool {
	return request.Limit >= 1 && request.Limit <= 100 && !request.Snapshot.IsZero() && request.Snapshot.Location() == time.UTC &&
		(request.AfterVersion == 0) == request.AfterCreated.IsZero() && request.AfterVersion >= 0 && (request.AfterVersion == 0 || (request.AfterCreated.Location() == time.UTC && !request.AfterCreated.After(request.Snapshot)))
}

func validOpaquePersistenceID(value string) bool {
	return value != "" && len(value) <= 256 && strings.TrimSpace(value) == value
}

func classifySocialCommentError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, ports.ErrConflict):
		return err
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return ports.ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ports.ErrConflict
	default:
		return fmt.Errorf("social comment persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
