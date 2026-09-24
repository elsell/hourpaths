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

type socialReactionModel struct {
	SocialFeedEventID, ActorUserID string
	ReactionType                   string
	CreatedAt, UpdatedAt           time.Time
}

func (socialReactionModel) TableName() string { return "social_practice_reaction_models" }

type socialReactionReplayModel struct {
	ActorUserID, Operation, IdempotencyKey, SocialFeedEventID string
	RequestHash                                               []byte
	CreatedAt                                                 time.Time
}

func (socialReactionReplayModel) TableName() string { return "social_practice_reaction_replay_models" }

type socialReactionNotificationModel struct{ ID string }

func (socialReactionNotificationModel) TableName() string { return "notification_models" }

func (repository *SocialFeedRepository) ResolvePracticeReactionTarget(ctx context.Context, viewer, eventID string) (socialapp.ReactionTarget, error) {
	if repository == nil || repository.db == nil || strings.TrimSpace(viewer) == "" || strings.TrimSpace(eventID) == "" {
		return socialapp.ReactionTarget{}, ports.ErrInvalidArgument
	}
	target, err := resolvePracticeReactionTarget(repository.db.WithContext(ctx), viewer, eventID)
	return target, classifySocialReactionError(err)
}

func (repository *SocialFeedRepository) SetPracticeReaction(ctx context.Context, command socialapp.ReactionCommand) (socialapp.ReactionMutationResult, error) {
	if !validReactionCommand(repository, command, true) {
		return socialapp.ReactionMutationResult{}, ports.ErrInvalidArgument
	}
	return repository.mutatePracticeReaction(ctx, command, true)
}

func (repository *SocialFeedRepository) RemovePracticeReaction(ctx context.Context, command socialapp.ReactionCommand) (socialapp.ReactionMutationResult, error) {
	if !validReactionCommand(repository, command, false) {
		return socialapp.ReactionMutationResult{}, ports.ErrInvalidArgument
	}
	return repository.mutatePracticeReaction(ctx, command, false)
}

func (repository *SocialFeedRepository) mutatePracticeReaction(ctx context.Context, command socialapp.ReactionCommand, set bool) (socialapp.ReactionMutationResult, error) {
	var result socialapp.ReactionMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialPathAudience(tx, command.Target.PathID); err != nil {
			return err
		}
		if err := lockPracticeReactionEvent(tx, command.Target.EventID); err != nil {
			return err
		}
		if err := lockSocialInteractionPairs(tx, command.ActorUserID, command.Target.EventID, ""); err != nil {
			return err
		}
		if set {
			if err := lockSocialInteractionOwner(tx, command.Target.OwnerUserID); err != nil {
				return err
			}
		}
		target, err := resolvePracticeReactionTarget(tx, command.ActorUserID, command.Target.EventID)
		if err != nil {
			return err
		}
		if target != command.Target {
			return ports.ErrNotFound
		}
		replay, found, err := readSocialReactionReplay(tx, command)
		if err != nil {
			return err
		}
		if found {
			if replay.SocialFeedEventID != command.Target.EventID {
				return ports.ErrIdempotencyConflict
			}
			result.Replayed = true
			result.Summary, err = socialReactionSummary(tx, command.Target.EventID, command.ActorUserID)
			return err
		}
		if set {
			enabled, err := socialInteractionEnabled(tx, target.OwnerUserID, "reactions_enabled")
			if err != nil {
				return err
			}
			if !enabled {
				return ports.ErrNotFound
			}
			reaction := socialReactionModel{SocialFeedEventID: command.Target.EventID, ActorUserID: command.ActorUserID, ReactionType: string(command.Reaction), CreatedAt: command.OccurredAt, UpdatedAt: command.OccurredAt}
			if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "social_feed_event_id"}, {Name: "actor_user_id"}}, DoUpdates: clause.Assignments(map[string]any{"reaction_type": string(command.Reaction), "updated_at": command.OccurredAt})}).Create(&reaction).Error; err != nil {
				return err
			}
			if command.ActorUserID != command.Target.OwnerUserID {
				if err := upsertReactionNotification(tx, command); err != nil {
					return err
				}
			}
		} else {
			if err := tx.Where("social_feed_event_id = ? AND actor_user_id = ?", command.Target.EventID, command.ActorUserID).Delete(&socialReactionModel{}).Error; err != nil {
				return err
			}
			if err := tx.Where("kind = 'practice_reaction' AND social_feed_event_id = ? AND actor_user_id = ? AND recipient_user_id = ?", command.Target.EventID, command.ActorUserID, command.Target.OwnerUserID).Delete(&socialReactionNotificationModel{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&socialReactionReplayModel{ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), SocialFeedEventID: command.Target.EventID, CreatedAt: command.OccurredAt}).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result.Summary, err = socialReactionSummary(tx, command.Target.EventID, command.ActorUserID)
		return err
	})
	return result, classifySocialReactionError(err)
}

func lockPracticeReactionEvent(tx *gorm.DB, eventID string) error {
	key := socialLockKey("social-practice-reaction-event", eventID)
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock
}

func resolvePracticeReactionTarget(tx *gorm.DB, viewer, eventID string) (socialapp.ReactionTarget, error) {
	var target socialapp.ReactionTarget
	query := tx.Table("social_feed_event_models AS event").
		Select("event.id AS event_id, event.path_id, event.participant_user_id AS owner_user_id").
		Joins("LEFT JOIN recorded_activity_models activity ON activity.id = event.source_activity_id AND activity.participant_id = event.participant_user_id AND activity.path_id = event.path_id").
		Joins("LEFT JOIN social_goal_achievement_models achievement ON achievement.id = event.achievement_id AND achievement.participant_user_id = event.participant_user_id AND achievement.path_id = event.path_id").
		Joins("JOIN user_models owner ON owner.id = event.participant_user_id AND owner.status = 'active'").
		Joins("JOIN path_models path ON path.id = event.path_id").
		Where("event.id = ?", eventID).
		Where("((event.source_activity_id IS NOT NULL AND event.achievement_id IS NULL AND activity.id IS NOT NULL) OR (event.source_activity_id IS NULL AND event.achievement_id IS NOT NULL AND achievement.id IS NOT NULL))").
		Where("EXISTS (SELECT 1 FROM user_models viewer WHERE viewer.id = ? AND viewer.status = 'active')", viewer).
		Where(`(path.owner_user_id = event.participant_user_id OR
  EXISTS (SELECT 1 FROM path_membership_models source_membership
          WHERE source_membership.path_id = path.id
            AND source_membership.user_id = event.participant_user_id
            AND source_membership.role IN ('administrator', 'participant')))`).
		Where(`NOT EXISTS (SELECT 1 FROM block_models block
WHERE (block.blocker_user_id = ? AND block.blocked_user_id = event.participant_user_id)
   OR (block.blocker_user_id = event.participant_user_id AND block.blocked_user_id = ?))`, viewer, viewer).
		Where(`(path.owner_user_id = ? OR
  EXISTS (SELECT 1 FROM path_membership_models visibility_member
          WHERE visibility_member.path_id = path.id
            AND visibility_member.user_id = ?) OR
  path.visibility = 'public' OR
  (path.visibility = 'followers' AND EXISTS (
    SELECT 1 FROM follow_models visibility_follow
    WHERE visibility_follow.follower_user_id = ?
      AND visibility_follow.following_user_id = path.owner_user_id
  )))`, viewer, viewer, viewer).
		Where(`(event.participant_user_id = ? OR
  EXISTS (SELECT 1 FROM follow_models follow WHERE follow.follower_user_id = ? AND follow.following_user_id = event.participant_user_id) OR
  ((path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models vm WHERE vm.path_id = path.id AND vm.user_id = ? AND vm.role IN ('administrator', 'participant')))
   AND (path.owner_user_id = event.participant_user_id OR EXISTS (SELECT 1 FROM path_membership_models om WHERE om.path_id = path.id AND om.user_id = event.participant_user_id AND om.role IN ('administrator', 'participant')))))`, viewer, viewer, viewer, viewer)
	if err := query.Take(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return socialapp.ReactionTarget{}, ports.ErrNotFound
		}
		return socialapp.ReactionTarget{}, err
	}
	return target, nil
}

func readSocialReactionReplay(tx *gorm.DB, command socialapp.ReactionCommand) (socialReactionReplayModel, bool, error) {
	var replay socialReactionReplayModel
	err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&replay).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return replay, false, nil
	}
	if err != nil {
		return replay, false, err
	}
	if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
		return replay, false, ports.ErrIdempotencyConflict
	}
	return replay, true, nil
}

func socialReactionSummary(tx *gorm.DB, eventID, viewer string) (socialdomain.ReactionSummary, error) {
	var owner string
	if err := tx.Table("social_feed_event_models").Select("participant_user_id").Where("id = ?", eventID).Scan(&owner).Error; err != nil {
		return socialdomain.ReactionSummary{}, err
	}
	enabled, err := socialInteractionEnabled(tx, owner, "reactions_enabled")
	if err != nil {
		return socialdomain.ReactionSummary{}, err
	}
	if !enabled {
		return socialdomain.ReactionSummary{Counts: socialdomain.ReactionCounts{}}, nil
	}
	var row struct {
		Heart, Applause, Fire, Strong, Celebrate int64
		ViewerReaction                           string
	}
	err = tx.Table("social_practice_reaction_models").Select(`
COUNT(*) FILTER (WHERE reaction_type = 'heart') AS heart,
COUNT(*) FILTER (WHERE reaction_type = 'applause') AS applause,
COUNT(*) FILTER (WHERE reaction_type = 'fire') AS fire,
COUNT(*) FILTER (WHERE reaction_type = 'strong') AS strong,
COUNT(*) FILTER (WHERE reaction_type = 'celebrate') AS celebrate,
COALESCE(MAX(reaction_type) FILTER (WHERE actor_user_id = ?), '') AS viewer_reaction`, viewer).
		Where("social_feed_event_id = ?", eventID).
		Where(`NOT EXISTS (SELECT 1 FROM block_models reaction_block
WHERE (reaction_block.blocker_user_id = ? AND reaction_block.blocked_user_id = social_practice_reaction_models.actor_user_id)
   OR (reaction_block.blocker_user_id = social_practice_reaction_models.actor_user_id AND reaction_block.blocked_user_id = ?))`, viewer, viewer).
		Scan(&row).Error
	return socialdomain.ReactionSummary{Counts: socialdomain.ReactionCounts{Heart: row.Heart, Applause: row.Applause, Fire: row.Fire, Strong: row.Strong, Celebrate: row.Celebrate}, ViewerReaction: socialdomain.Reaction(row.ViewerReaction)}, err
}

func upsertReactionNotification(tx *gorm.DB, command socialapp.ReactionCommand) error {
	result := tx.Table("notification_models").Where("kind = 'practice_reaction' AND social_feed_event_id = ? AND actor_user_id = ? AND recipient_user_id = ?", command.Target.EventID, command.ActorUserID, command.Target.OwnerUserID).Update("reaction_type", string(command.Reaction))
	if result.Error != nil || result.RowsAffected != 0 {
		return result.Error
	}
	digest := sha256.Sum256([]byte(command.Target.EventID + "\x00" + command.ActorUserID + "\x00" + command.Idempotency.Key))
	notificationID := "reaction:" + hex.EncodeToString(digest[:16])
	row := map[string]any{"id": notificationID, "recipient_user_id": command.Target.OwnerUserID, "actor_user_id": command.ActorUserID, "path_id": command.Target.PathID, "social_feed_event_id": command.Target.EventID, "reaction_type": string(command.Reaction), "kind": "practice_reaction", "presentation_class": "informational", "channel": "reactions", "created_at": command.NotificationEligibleAt}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	if err := tx.Table("notification_push_outbox_models").Create(map[string]any{"notification_id": notificationID, "created_at": command.NotificationEligibleAt}).Error; err != nil {
		return err
	}
	if err := tx.Exec(`INSERT INTO notification_push_delivery_models (
notification_id, installation_id, recipient_user_id, provider, platform, locale,
token_ciphertext, token_nonce, token_hash, available_at, created_at)
SELECT ?, installation.id, ?, installation.provider, installation.platform, installation.locale,
installation.token_ciphertext, installation.token_nonce, installation.token_hash, ?, ?
FROM push_installation_models installation
WHERE installation.owner_user_id = ? AND installation.deleted_at IS NULL
ON CONFLICT (notification_id, installation_id) DO NOTHING`, notificationID, command.Target.OwnerUserID, command.NotificationEligibleAt, command.NotificationEligibleAt, command.Target.OwnerUserID).Error; err != nil {
		return err
	}
	return tx.Exec(`UPDATE notification_push_outbox_models
SET suppressed_at = ?, failure_code = 'no_active_installation'
WHERE notification_id = ?
  AND NOT EXISTS (SELECT 1 FROM notification_push_delivery_models WHERE notification_id = ?)`, command.NotificationEligibleAt, notificationID, notificationID).Error
}

func validReactionCommand(repository *SocialFeedRepository, command socialapp.ReactionCommand, set bool) bool {
	wantOperation, wantAction := socialapp.RemovePracticeReactionOperation, audit.ResourceDeleted
	if set {
		wantOperation, wantAction = socialapp.SetPracticeReactionOperation, audit.ResourceUpdated
	}
	return repository != nil && repository.db != nil && command.ActorUserID != "" && command.Target.EventID != "" && command.Target.PathID != "" && command.Target.OwnerUserID != "" &&
		((set && command.Reaction.Valid()) || (!set && command.Reaction == "")) && !command.OccurredAt.IsZero() && command.NotificationEligibleAt.Equal(command.OccurredAt.Add(5*time.Second)) &&
		command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == wantOperation && command.Idempotency.Key != "" && len(command.Idempotency.RequestHash) == sha256.Size &&
		validMutationAudit(command.Audit, wantAction, "practice_reaction", command.Target.EventID, command.ActorUserID) && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.OccurredAt)
}

func classifySocialReactionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrIdempotencyConflict):
		return err
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		// A concurrently removed event, Path, participant, or actor must remain
		// indistinguishable from any other inaccessible reaction target.
		return ports.ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ports.ErrConflict
	default:
		return fmt.Errorf("social reaction persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
