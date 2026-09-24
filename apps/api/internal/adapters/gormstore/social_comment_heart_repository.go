package gormstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type socialCommentHeartModel struct {
	CommentID, ActorUserID string
	CreatedAt              time.Time
}

func (socialCommentHeartModel) TableName() string { return "social_practice_comment_heart_models" }

type socialCommentHeartReplayModel struct {
	ActorUserID, Operation, IdempotencyKey string
	RequestHash                            []byte
	SocialFeedEventID, CommentID           string
	ResultHearted                          bool
	ResultHeartCount                       int64
	CreatedAt                              time.Time
}

func (socialCommentHeartReplayModel) TableName() string {
	return "social_practice_comment_heart_replay_models"
}

func (repository *SocialFeedRepository) FindPracticeCommentHeartReplay(ctx context.Context, idempotency ports.Idempotency) (socialapp.PracticeCommentHeartReplay, bool, error) {
	if repository == nil || repository.db == nil || !validCommentHeartIdempotency(idempotency) {
		return socialapp.PracticeCommentHeartReplay{}, false, ports.ErrInvalidArgument
	}
	replay, found, err := readCommentHeartReplay(repository.db.WithContext(ctx), idempotency)
	if err != nil || !found {
		return socialapp.PracticeCommentHeartReplay{}, found, classifySocialCommentError(err)
	}
	return mapCommentHeartReplay(replay), true, nil
}

func (repository *SocialFeedRepository) SetPracticeCommentHeart(ctx context.Context, command socialapp.CommentHeartCommand) (socialapp.CommentHeartSummary, error) {
	return repository.mutatePracticeCommentHeart(ctx, command, true)
}

func (repository *SocialFeedRepository) RemovePracticeCommentHeart(ctx context.Context, command socialapp.CommentHeartCommand) (socialapp.CommentHeartSummary, error) {
	return repository.mutatePracticeCommentHeart(ctx, command, false)
}

func (repository *SocialFeedRepository) mutatePracticeCommentHeart(ctx context.Context, command socialapp.CommentHeartCommand, hearted bool) (socialapp.CommentHeartSummary, error) {
	if !validCommentHeartCommand(repository, command, hearted) {
		return socialapp.CommentHeartSummary{}, ports.ErrInvalidArgument
	}
	var summary socialapp.CommentHeartSummary
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialPathAudience(tx, command.Target.PathID); err != nil {
			return err
		}
		if err := lockSocialInteractionPairs(tx, command.ActorUserID, command.Target.EventID, command.Comment.ID); err != nil {
			return err
		}
		if err := lockSocialCommentTarget(tx, command.Comment.ID); err != nil {
			return err
		}
		if hearted {
			if err := lockSocialInteractionOwner(tx, command.Target.OwnerUserID); err != nil {
				return err
			}
		}
		resolved, err := resolvePracticeComment(tx, command.ActorUserID, command.Target.EventID, command.Comment.ID)
		if err != nil {
			return err
		}
		if resolved.Target != command.Target || resolved.Comment != command.Comment {
			return ports.ErrNotFound
		}
		if replay, found, err := readCommentHeartReplay(tx, command.Idempotency); err != nil {
			return err
		} else if found {
			mapped := mapCommentHeartReplay(replay)
			if mapped.EventID != command.Target.EventID || mapped.CommentID != command.Comment.ID || mapped.Hearted != hearted {
				return ports.ErrIdempotencyConflict
			}
			summary = mapped.Summary
			return nil
		}
		if hearted {
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&socialCommentHeartModel{CommentID: command.Comment.ID, ActorUserID: command.ActorUserID, CreatedAt: command.OccurredAt})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 1 && command.NotifyCommentAuthor {
				if err := createCommentHeartNotification(tx, command); err != nil {
					return err
				}
			}
		} else {
			if err := tx.Where("comment_id = ? AND actor_user_id = ?", command.Comment.ID, command.ActorUserID).Delete(&socialCommentHeartModel{}).Error; err != nil {
				return err
			}
			if err := tx.Table("notification_models").Where("kind = 'comment_heart' AND comment_id = ? AND actor_user_id = ? AND recipient_user_id = ?", command.Comment.ID, command.ActorUserID, command.Comment.AuthorID).Delete(&socialCommentNotificationModel{}).Error; err != nil {
				return err
			}
		}
		summary, err = commentHeartSummary(tx, command.Comment.ID, command.ActorUserID)
		if err != nil {
			return err
		}
		replay := socialCommentHeartReplayModel{ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), SocialFeedEventID: command.Target.EventID, CommentID: command.Comment.ID, ResultHearted: hearted, ResultHeartCount: summary.HeartCount, CreatedAt: command.OccurredAt}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		return appendAuditEvent(tx, command.Audit)
	})
	return summary, classifySocialCommentError(err)
}

func (repository *SocialFeedRepository) ListPracticeCommentHearts(ctx context.Context, viewer, eventID, commentID string, request socialapp.CommentHeartRosterPageRequest) (socialapp.CommentHeartRosterPage, error) {
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(viewer) || !validOpaquePersistenceID(eventID) || !validOpaquePersistenceID(commentID) || !validCommentHeartPageRequest(request) {
		return socialapp.CommentHeartRosterPage{}, ports.ErrInvalidArgument
	}
	db := repository.db.WithContext(ctx)
	if _, err := resolvePracticeComment(db, viewer, eventID, commentID); err != nil {
		return socialapp.CommentHeartRosterPage{}, classifySocialCommentError(err)
	}
	type row struct {
		UserID, Username, DisplayName, ProfilePictureURL, Description string
		FollowerCount, FollowingCount                                 int64
		Relationship                                                  socialdomain.RelationshipState
		CreatedAt                                                     time.Time
	}
	var rows []row
	query := db.Table("social_practice_comment_heart_models AS heart").Select(fmt.Sprintf(`heart.actor_user_id AS user_id, heart.created_at,
u.username, u.display_name, COALESCE(u.profile_picture_url, '') AS profile_picture_url, COALESCE(u.description, '') AS description,
%s, %s`, socialCountsSQL, socialRelationshipSQL), viewer, viewer, viewer).
		Joins("JOIN user_models u ON u.id = heart.actor_user_id AND u.status = 'active' AND u.username IS NOT NULL").
		Where("heart.comment_id = ? AND heart.created_at <= ?", commentID, request.Snapshot).
		Where(`NOT EXISTS (SELECT 1 FROM block_models heart_block
WHERE (heart_block.blocker_user_id = ? AND heart_block.blocked_user_id = heart.actor_user_id)
   OR (heart_block.blocker_user_id = heart.actor_user_id AND heart_block.blocked_user_id = ?))`, viewer, viewer)
	if request.AfterUserID != "" {
		query = query.Where("heart.created_at > ? OR (heart.created_at = ? AND heart.actor_user_id > ?)", request.AfterCreated, request.AfterCreated, request.AfterUserID)
	}
	if err := query.Order("heart.created_at ASC, heart.actor_user_id ASC").Limit(request.Limit + 1).Scan(&rows).Error; err != nil {
		return socialapp.CommentHeartRosterPage{}, classifySocialCommentError(err)
	}
	hasMore := len(rows) > request.Limit
	if hasMore {
		rows = rows[:request.Limit]
	}
	page := socialapp.CommentHeartRosterPage{Items: make([]socialapp.CommentHeartRosterItem, 0, len(rows)), HasMore: hasMore}
	for _, row := range rows {
		page.Items = append(page.Items, socialapp.CommentHeartRosterItem{Profile: socialdomain.PublicProfile{ID: row.UserID, Username: row.Username, DisplayName: row.DisplayName, ProfilePictureURL: row.ProfilePictureURL, Description: row.Description, FollowerCount: row.FollowerCount, FollowingCount: row.FollowingCount, Relationship: row.Relationship}, HeartedAt: row.CreatedAt.UTC()})
	}
	return page, nil
}

func readCommentHeartReplay(tx *gorm.DB, idempotency ports.Idempotency) (socialCommentHeartReplayModel, bool, error) {
	var replay socialCommentHeartReplayModel
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

func mapCommentHeartReplay(model socialCommentHeartReplayModel) socialapp.PracticeCommentHeartReplay {
	return socialapp.PracticeCommentHeartReplay{ActorUserID: model.ActorUserID, EventID: model.SocialFeedEventID, CommentID: model.CommentID, Hearted: model.ResultHearted, Summary: socialapp.CommentHeartSummary{CommentID: model.CommentID, HeartCount: model.ResultHeartCount, HeartedByViewer: model.ResultHearted}}
}

func commentHeartSummary(tx *gorm.DB, commentID, viewer string) (socialapp.CommentHeartSummary, error) {
	var row struct {
		Count   int64
		Hearted bool
	}
	err := tx.Table("social_practice_comment_heart_models").Select("COUNT(*) AS count, COUNT(*) FILTER (WHERE actor_user_id = ?) > 0 AS hearted", viewer).
		Where("comment_id = ?", commentID).
		Where(`NOT EXISTS (SELECT 1 FROM block_models heart_count_block
WHERE (heart_count_block.blocker_user_id = ? AND heart_count_block.blocked_user_id = social_practice_comment_heart_models.actor_user_id)
   OR (heart_count_block.blocker_user_id = social_practice_comment_heart_models.actor_user_id AND heart_count_block.blocked_user_id = ?))`, viewer, viewer).
		Scan(&row).Error
	return socialapp.CommentHeartSummary{CommentID: commentID, HeartCount: row.Count, HeartedByViewer: row.Hearted}, err
}

func createCommentHeartNotification(tx *gorm.DB, command socialapp.CommentHeartCommand) error {
	digest := sha256.Sum256([]byte(command.Comment.ID + "\x00" + command.ActorUserID))
	notificationID := "comment-heart:" + hex.EncodeToString(digest[:16])
	row := map[string]any{"id": notificationID, "recipient_user_id": command.Comment.AuthorID, "actor_user_id": command.ActorUserID, "path_id": command.Target.PathID, "social_feed_event_id": command.Target.EventID, "comment_id": command.Comment.ID, "kind": "comment_heart", "presentation_class": "informational", "channel": "comment_hearts", "created_at": command.NotificationEligibleAt}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	return createSocialInteractionPush(tx, notificationID, command.Comment.AuthorID, command.NotificationEligibleAt)
}

func validCommentHeartCommand(repository *SocialFeedRepository, command socialapp.CommentHeartCommand, hearted bool) bool {
	wantOperation, wantAction := socialapp.RemovePracticeCommentHeartOperation, audit.ResourceDeleted
	if hearted {
		wantOperation, wantAction = socialapp.SetPracticeCommentHeartOperation, audit.ResourceCreated
	}
	if repository == nil || repository.db == nil || !validOpaquePersistenceID(command.ActorUserID) || !validHeartTarget(command.Target, command.Comment.EventID) || !command.Comment.Valid() || command.Comment.EventID != command.Target.EventID || !validCommentHeartIdempotency(command.Idempotency) || command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != wantOperation || command.OccurredAt.IsZero() || command.OccurredAt.Location() != time.UTC || !validMutationAudit(command.Audit, wantAction, "practice_comment_heart", command.Comment.ID, command.ActorUserID) {
		return false
	}
	if hearted {
		return command.NotificationEligibleAt.Equal(command.OccurredAt.Add(5*time.Second)) && command.NotifyCommentAuthor == (command.ActorUserID != command.Comment.AuthorID)
	}
	return command.NotificationEligibleAt.IsZero() && !command.NotifyCommentAuthor
}

func validHeartTarget(target socialapp.PracticeCommentTarget, eventID string) bool {
	return target.EventID == eventID && validOpaquePersistenceID(target.PathID) && validOpaquePersistenceID(target.OwnerUserID)
}
func validCommentHeartIdempotency(value ports.Idempotency) bool {
	return validCommentIdempotencyShape(value) && (value.Operation == socialapp.SetPracticeCommentHeartOperation || value.Operation == socialapp.RemovePracticeCommentHeartOperation)
}
func validCommentIdempotencyShape(value ports.Idempotency) bool {
	if !validOpaquePersistenceID(value.PrincipalID) || len(value.Key) < 16 || len(value.Key) > 128 || strings.TrimSpace(value.Key) != value.Key || len(value.RequestHash) != sha256.Size {
		return false
	}
	for index := range len(value.Key) {
		if value.Key[index] < 0x20 || value.Key[index] > 0x7e {
			return false
		}
	}
	return true
}
func validCommentHeartPageRequest(request socialapp.CommentHeartRosterPageRequest) bool {
	return request.Limit >= 1 && request.Limit <= 100 && !request.Snapshot.IsZero() && request.Snapshot.Location() == time.UTC && (request.AfterUserID == "") == request.AfterCreated.IsZero() && (request.AfterUserID == "" || (validOpaquePersistenceID(request.AfterUserID) && request.AfterCreated.Location() == time.UTC && !request.AfterCreated.After(request.Snapshot)))
}
