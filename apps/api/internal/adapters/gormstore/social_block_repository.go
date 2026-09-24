package gormstore

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type socialBlockModel struct {
	BlockerUserID, BlockedUserID string
	CreatedAt                    time.Time
}

func (socialBlockModel) TableName() string { return "block_models" }

type socialBlockReplayModel struct {
	ActorUserID, Operation, IdempotencyKey string
	RequestHash                            []byte
	TargetUserID, TargetUsername           string
	TargetDisplayName                      string
	ResultBlocked, Changed                 bool
	AuthorizationChangeIDs                 pq.StringArray `gorm:"type:text[]"`
	CreatedAt                              time.Time
}

func (socialBlockReplayModel) TableName() string { return "social_block_replay_models" }

var _ socialapp.BlockRepository = (*SocialRelationshipRepository)(nil)

func (repository *SocialRelationshipRepository) Review(ctx context.Context, actor, username string) (socialapp.BlockReview, error) {
	if !repository.ready() || strings.TrimSpace(actor) == "" || strings.TrimSpace(username) == "" {
		return socialapp.BlockReview{}, ports.ErrInvalidArgument
	}
	var target socialRelationshipUser
	var shared []domain.SharedPath
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		target, err = socialBlockTargetByUsername(tx, actor, username, false)
		if err != nil {
			return err
		}
		blocked, err := socialPairBlocked(tx, actor, target.ID)
		if err != nil {
			return err
		}
		shared, err = socialSharedPaths(tx, actor, target.ID)
		if err != nil {
			return err
		}
		if blocked && len(shared) == 0 {
			return ports.ErrNotFound
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return socialapp.BlockReview{}, classifySocialBlockError(err)
	}
	return socialapp.BlockReview{Target: blockTargetFromUser(target), SharedPaths: shared}, nil
}

func (repository *SocialRelationshipRepository) Block(ctx context.Context, command socialapp.BlockCommand) (socialapp.BlockMutationResult, error) {
	if !repository.validBlockCommand(command, socialapp.BlockOperation, true) {
		return socialapp.BlockMutationResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.BlockMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockBlockIdempotency(tx, command); err != nil {
			return err
		}
		if replay, found, err := readBlockReplay(tx, command); err != nil {
			return err
		} else if found {
			result, err = repository.blockReplayResult(tx, replay)
			return err
		}
		if !command.OccurredAt.Before(command.ReviewExpiresAt) {
			return socialapp.ErrBlockReviewRequired
		}
		target, err := socialBlockTargetByUsername(tx, command.ActorUserID, command.TargetUsername, true)
		if err != nil {
			return err
		}
		if target.ID != command.TargetUserID {
			return socialapp.ErrBlockReviewRequired
		}
		shared, err := socialSharedPaths(tx, command.ActorUserID, target.ID)
		if err != nil {
			return err
		}
		currentSharedPathIDs := make([]string, len(shared))
		for index, path := range shared {
			currentSharedPathIDs[index] = path.ID
		}
		sort.Strings(currentSharedPathIDs)
		if !slices.Equal(currentSharedPathIDs, command.ExpectedSharedPathIDs) {
			return socialapp.ErrBlockReviewRequired
		}
		blocked, err := socialPairBlocked(tx, command.ActorUserID, target.ID)
		if err != nil {
			return err
		}
		if blocked {
			var authored int64
			if err := tx.Model(&socialBlockModel{}).Where("blocker_user_id = ? AND blocked_user_id = ?", command.ActorUserID, target.ID).Count(&authored).Error; err != nil {
				return err
			}
			shared, err := socialSharedPaths(tx, command.ActorUserID, target.ID)
			if err != nil {
				return err
			}
			if authored == 0 && len(shared) == 0 {
				return ports.ErrNotFound
			}
		}

		created := false
		var authored int64
		if err := tx.Model(&socialBlockModel{}).Where("blocker_user_id = ? AND blocked_user_id = ?", command.ActorUserID, target.ID).Count(&authored).Error; err != nil {
			return err
		}
		if authored == 0 {
			if err := tx.Create(&socialBlockModel{BlockerUserID: command.ActorUserID, BlockedUserID: target.ID, CreatedAt: command.OccurredAt}).Error; err != nil {
				return err
			}
			created = true
		}

		changes, err := repository.removeSocialPairRelationships(tx, command.ActorUserID, target.ID, command.OccurredAt)
		if err != nil {
			return err
		}
		if err := tx.Model(&socialFollowRequestModel{}).
			Where("((requester_user_id = ? AND target_user_id = ?) OR (requester_user_id = ? AND target_user_id = ?)) AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL", command.ActorUserID, target.ID, target.ID, command.ActorUserID).
			Update("canceled_at", command.OccurredAt).Error; err != nil {
			return err
		}
		if err := suppressBlockedPairPush(tx, command.ActorUserID, target.ID, command.OccurredAt); err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		ids := make(pq.StringArray, len(changes))
		for index := range changes {
			ids[index] = changes[index].ID
		}
		if err := createBlockReplay(tx, command, target, true, created || len(changes) > 0, ids); err != nil {
			return err
		}
		result = socialapp.BlockMutationResult{Target: blockTargetFromUser(target), Blocked: true, Changed: created || len(changes) > 0, AuthorizationChanges: changes}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
	return result, classifySocialBlockError(err)
}

func (repository *SocialRelationshipRepository) Unblock(ctx context.Context, command socialapp.BlockCommand) (socialapp.BlockMutationResult, error) {
	if !repository.validBlockCommand(command, socialapp.UnblockOperation, false) {
		return socialapp.BlockMutationResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.BlockMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockBlockIdempotency(tx, command); err != nil {
			return err
		}
		if replay, found, err := readBlockReplay(tx, command); err != nil {
			return err
		} else if found {
			result, err = repository.blockReplayResult(tx, replay)
			return err
		}
		var target socialRelationshipUser
		if err := tx.Table("user_models AS target").Select("target.id, target.username, target.display_name, target.status").
			Joins("JOIN block_models authored ON authored.blocked_user_id = target.id AND authored.blocker_user_id = ?", command.ActorUserID).
			Where("target.id = ? AND target.status = ? AND target.username IS NOT NULL", command.TargetUserID, identity.StatusActive).Take(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ports.ErrNotFound
			}
			return err
		}
		if err := lockSocialPair(tx, command.ActorUserID, target.ID); err != nil {
			return err
		}
		deleted := tx.Where("blocker_user_id = ? AND blocked_user_id = ?", command.ActorUserID, target.ID).Delete(&socialBlockModel{})
		if deleted.Error != nil {
			return deleted.Error
		}
		if deleted.RowsAffected != 1 {
			return ports.ErrNotFound
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		if err := createBlockReplay(tx, command, target, false, true, pq.StringArray{}); err != nil {
			return err
		}
		result = socialapp.BlockMutationResult{Target: blockTargetFromUser(target), Blocked: false, Changed: true}
		return nil
	})
	return result, classifySocialBlockError(err)
}

func (repository *SocialRelationshipRepository) ListAuthored(ctx context.Context, actor string, page ports.PageRequest) (socialapp.BlockedAccountPage, error) {
	if !repository.ready() || strings.TrimSpace(actor) == "" || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() ||
		(page.AfterID == "") != page.AfterCreated.IsZero() || (!page.AfterCreated.IsZero() && page.AfterCreated.After(page.Snapshot)) {
		return socialapp.BlockedAccountPage{}, ports.ErrInvalidArgument
	}
	type row struct {
		UserID, Username, DisplayName string
		BlockedAt                     time.Time
	}
	var rows []row
	query := repository.db.WithContext(ctx).Table("block_models AS authored").Select("target.id AS user_id, target.username, target.display_name, authored.created_at AS blocked_at").
		Joins("JOIN user_models target ON target.id = authored.blocked_user_id AND target.status = ? AND target.username IS NOT NULL", identity.StatusActive).
		Where("authored.blocker_user_id = ? AND authored.created_at <= ?", actor, page.Snapshot)
	if page.AfterID != "" {
		query = query.Where("authored.created_at < ? OR (authored.created_at = ? AND authored.blocked_user_id < ?)", page.AfterCreated, page.AfterCreated, page.AfterID)
	}
	if err := query.Order("authored.created_at DESC, authored.blocked_user_id DESC").Limit(page.Limit + 1).Scan(&rows).Error; err != nil {
		return socialapp.BlockedAccountPage{}, fmt.Errorf("list blocked accounts: %w: %v", ports.ErrUnavailable, err)
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	accounts := make([]domain.BlockedAccount, len(rows))
	for index, row := range rows {
		accounts[index] = domain.BlockedAccount{Target: domain.BlockTarget{UserID: row.UserID, Username: row.Username, DisplayName: row.DisplayName}, BlockedAt: row.BlockedAt.UTC()}
	}
	return socialapp.BlockedAccountPage{Accounts: accounts, HasMore: hasMore}, nil
}

func (repository *SocialRelationshipRepository) validBlockCommand(command socialapp.BlockCommand, operation string, byUsername bool) bool {
	if !repository.ready() || command.ActorUserID == "" || command.OccurredAt.IsZero() || command.OccurredAt.Location() != time.UTC ||
		command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != operation || command.Idempotency.Key == "" || len(command.Idempotency.RequestHash) != 32 ||
		!command.Audit.Valid() || command.Audit.OwnerUserID != command.ActorUserID || command.Audit.ActorUserID != command.ActorUserID || command.Audit.Outcome != "succeeded" || !command.Audit.OccurredAt.Equal(command.OccurredAt) {
		return false
	}
	if byUsername {
		if command.TargetUsername == "" || command.TargetUserID == "" || command.ReviewExpiresAt.IsZero() || command.ReviewExpiresAt.Location() != time.UTC || !sort.StringsAreSorted(command.ExpectedSharedPathIDs) {
			return false
		}
		for index, id := range command.ExpectedSharedPathIDs {
			if strings.TrimSpace(id) == "" || (index > 0 && command.ExpectedSharedPathIDs[index-1] == id) {
				return false
			}
		}
		return true
	}
	return command.TargetUserID != "" && command.TargetUsername == ""
}

func socialBlockTargetByUsername(tx *gorm.DB, actor, username string, lock bool) (socialRelationshipUser, error) {
	var target socialRelationshipUser
	if err := tx.Table("user_models").Where("status = ? AND lower(username) = ?", identity.StatusActive, username).Take(&target).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return target, ports.ErrNotFound
		}
		return target, err
	}
	if target.ID == actor || target.Username == nil {
		return socialRelationshipUser{}, ports.ErrNotFound
	}
	if lock {
		if err := lockSocialPair(tx, actor, target.ID); err != nil {
			return socialRelationshipUser{}, err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_models").Where("id = ? AND status = ? AND lower(username) = ?", target.ID, identity.StatusActive, username).Take(&target).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return socialRelationshipUser{}, ports.ErrNotFound
			}
			return socialRelationshipUser{}, err
		}
	}
	return target, nil
}

func socialSharedPaths(tx *gorm.DB, first, second string) ([]domain.SharedPath, error) {
	var paths []domain.SharedPath
	err := tx.Table("path_models AS path").Select("path.id, path.name").
		Where("(path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models first_member WHERE first_member.path_id = path.id AND first_member.user_id = ?))", first, first).
		Where("(path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models second_member WHERE second_member.path_id = path.id AND second_member.user_id = ?))", second, second).
		Order("path.name ASC, path.id ASC").Scan(&paths).Error
	return paths, err
}

func (repository *SocialRelationshipRepository) removeSocialPairRelationships(tx *gorm.DB, actor, target string, at time.Time) ([]ports.AuthorizationChange, error) {
	var follows []socialFollowModel
	if err := tx.Where("(follower_user_id = ? AND following_user_id = ?) OR (follower_user_id = ? AND following_user_id = ?)", actor, target, target, actor).Order("follower_user_id, following_user_id").Find(&follows).Error; err != nil {
		return nil, err
	}
	changes := make([]ports.AuthorizationChange, 0, len(follows))
	for _, follow := range follows {
		id := repository.nextID()
		if id == "" {
			return nil, errInvalidSocialRelationshipRepository
		}
		change, err := repository.enqueueFollowerAuthorization(tx, id, follow.FollowingUserID, follow.FollowerUserID, actor, ports.AuthorizationDelete, at)
		if err != nil {
			return nil, err
		}
		changes = append(changes, change)
	}
	if len(follows) > 0 {
		if err := tx.Where("(follower_user_id = ? AND following_user_id = ?) OR (follower_user_id = ? AND following_user_id = ?)", actor, target, target, actor).Delete(&socialFollowModel{}).Error; err != nil {
			return nil, err
		}
	}
	return changes, nil
}

func suppressBlockedPairPush(tx *gorm.DB, first, second string, at time.Time) error {
	notificationIDs := tx.Table("notification_models").Select("id").Where(
		"(recipient_user_id = ? AND actor_user_id = ?) OR (recipient_user_id = ? AND actor_user_id = ?)",
		first, second, second, first,
	)
	pending := tx.Table("notification_push_delivery_models AS pending").Select("1").
		Where("pending.notification_id = notification_push_outbox_models.notification_id").
		Where("pending.delivered_at IS NULL AND pending.suppressed_at IS NULL AND pending.permanently_failed_at IS NULL")
	if err := tx.Table("notification_push_delivery_models").
		Where("notification_id IN (?)", notificationIDs).
		Where("delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL").
		Updates(map[string]any{"suppressed_at": at, "failure_code": "blocked_relationship", "locked_by": nil, "locked_until": nil,
			"token_ciphertext": nil, "token_nonce": nil, "token_hash": nil}).Error; err != nil {
		return err
	}
	return tx.Table("notification_push_outbox_models").Where("notification_id IN (?)", notificationIDs).
		Where("delivered_at IS NULL AND suppressed_at IS NULL AND permanently_failed_at IS NULL").
		Where("NOT EXISTS (?)", pending).
		Updates(map[string]any{"suppressed_at": at, "failure_code": "blocked_relationship", "locked_by": nil, "locked_until": nil}).Error
}

func lockBlockIdempotency(tx *gorm.DB, command socialapp.BlockCommand) error {
	return lockSocialMutationIdempotency(tx, "social-block-idempotency", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key)
}

func readBlockReplay(tx *gorm.DB, command socialapp.BlockCommand) (socialBlockReplayModel, bool, error) {
	var replay socialBlockReplayModel
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

func createBlockReplay(tx *gorm.DB, command socialapp.BlockCommand, target socialRelationshipUser, blocked, changed bool, changeIDs pq.StringArray) error {
	return tx.Create(&socialBlockReplayModel{ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key,
		RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), TargetUserID: target.ID, TargetUsername: *target.Username,
		TargetDisplayName: target.DisplayName, ResultBlocked: blocked, Changed: changed, AuthorizationChangeIDs: changeIDs, CreatedAt: command.OccurredAt}).Error
}

func (repository *SocialRelationshipRepository) blockReplayResult(tx *gorm.DB, replay socialBlockReplayModel) (socialapp.BlockMutationResult, error) {
	changes := make([]ports.AuthorizationChange, 0, len(replay.AuthorizationChangeIDs))
	for _, id := range replay.AuthorizationChangeIDs {
		var row authorizationOutboxModel
		if err := tx.Where("id = ?", id).Take(&row).Error; err != nil {
			return socialapp.BlockMutationResult{}, err
		}
		changes = append(changes, socialAuthorizationChange(row, replay.CreatedAt, repository.authorizationWorker, repository.authorizationLease))
	}
	return socialapp.BlockMutationResult{Target: domain.BlockTarget{UserID: replay.TargetUserID, Username: replay.TargetUsername, DisplayName: replay.TargetDisplayName}, Blocked: replay.ResultBlocked, Changed: replay.Changed, AuthorizationChanges: changes, Replayed: true}, nil
}

func blockTargetFromUser(user socialRelationshipUser) domain.BlockTarget {
	return domain.BlockTarget{UserID: user.ID, Username: *user.Username, DisplayName: user.DisplayName}
}

func classifySocialBlockError(err error) error {
	if err == nil || errors.Is(err, ports.ErrNotFound) || errors.Is(err, ports.ErrInvalidArgument) || errors.Is(err, ports.ErrIdempotencyConflict) || errors.Is(err, socialapp.ErrBlockReviewRequired) {
		return err
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ports.ErrConflict
	}
	return fmt.Errorf("social block persistence: %w: %v", ports.ErrUnavailable, err)
}
