package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SocialRelationshipRepository atomically persists relationships, notifications, audits, and authorization.
type SocialRelationshipRepository struct {
	db                  *gorm.DB
	newID               func() string
	authorizationWorker string
	authorizationLease  time.Duration
}

func NewSocialRelationshipRepository(
	db *gorm.DB,
	newID func() string,
	authorizationWorker string,
	authorizationLease time.Duration,
) *SocialRelationshipRepository {
	return &SocialRelationshipRepository{
		db: db, newID: newID, authorizationWorker: authorizationWorker,
		authorizationLease: authorizationLease,
	}
}

type socialFollowModel struct {
	FollowerUserID, FollowingUserID string
	ActivityNotificationsEnabled    bool
	AuthorizationChangeID           *string
	CreatedAt                       time.Time
}

func (socialFollowModel) TableName() string { return "follow_models" }

type socialFollowRequestModel struct {
	ID, RequesterUserID, TargetUserID  string
	CreatedAt                          time.Time
	AcceptedAt, RejectedAt, CanceledAt *time.Time
}

func (socialFollowRequestModel) TableName() string { return "follow_request_models" }

type socialRelationshipReplayModel struct {
	ActorUserID, Operation, IdempotencyKey string
	RequestHash                            []byte
	TargetUserID                           string
	FollowRequestID                        *string
	AuthorizationChangeID                  *string
	Changed                                bool
	ResultState                            string
	CreatedAt                              time.Time
}

func (socialRelationshipReplayModel) TableName() string {
	return "social_relationship_replay_models"
}

type socialRelationshipUser struct {
	ID                string
	Username          *string
	DisplayName       string
	ProfileVisibility *identity.ProfileVisibility
	Status            identity.Status
}

type relationshipProfileRow struct {
	ID, Username, DisplayName, ProfilePictureURL, Description string
	FollowerCount, FollowingCount                             int64
	Relationship                                              domain.RelationshipState
}

type socialAuthorizationChangeStateRow struct {
	ID             string
	LockedBy       string
	LockedUntil    *time.Time
	CompletedAt    *time.Time
	DeadLetteredAt *time.Time
	LockActive     bool `gorm:"column:lock_active"`
}

var (
	errInvalidSocialRelationshipRepository = errors.New("social relationship repository dependencies are invalid")
	errSocialRelationshipUnavailable       = errors.New("social relationship is unavailable")
)

var (
	_ socialapp.RelationshipRepository          = (*SocialRelationshipRepository)(nil)
	_ socialapp.AuthorizationChangeStatusReader = (*SocialRelationshipRepository)(nil)
)

func (repository *SocialRelationshipRepository) Follow(ctx context.Context, command socialapp.RelationshipCommand) (socialapp.RelationshipResult, error) {
	if !repository.validCommand(command, socialapp.FollowOperation, true) {
		return socialapp.RelationshipResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.RelationshipResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialIdempotency(tx, command); err != nil {
			return err
		}
		target, err := lockVisibleSocialTarget(tx, command.ActorUserID, command.TargetUsername)
		if err != nil {
			return err
		}
		if replay, found, err := socialReplay(tx, command); err != nil {
			return err
		} else if found {
			profile, err := socialRelationshipProfile(tx, command.ActorUserID, target.ID)
			if err != nil {
				return err
			}
			profile.Relationship = domain.RelationshipState(replay.ResultState)
			change, err := replayAuthorizationChange(tx, replay, command.OccurredAt, repository.authorizationWorker, repository.authorizationLease)
			if err != nil {
				return err
			}
			result = socialapp.RelationshipResult{Target: profile, RequestID: replayRequestID(replay), Changed: replay.Changed, AuthorizationChange: change, Replayed: true}
			return nil
		}

		var existingFollow int64
		if err := tx.Model(&socialFollowModel{}).
			Where("follower_user_id = ? AND following_user_id = ?", command.ActorUserID, target.ID).
			Count(&existingFollow).Error; err != nil {
			return err
		}
		var pending socialFollowRequestModel
		pendingRead := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("requester_user_id = ? AND target_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL", command.ActorUserID, target.ID).
			First(&pending)
		if pendingRead.Error != nil && !errors.Is(pendingRead.Error, gorm.ErrRecordNotFound) {
			return pendingRead.Error
		}

		state, requestID, changed := domain.RelationshipFollowing, "", false
		var authorizationChange ports.AuthorizationChange
		switch {
		case existingFollow == 1:
		case pendingRead.Error == nil:
			state, requestID = domain.RelationshipRequested, pending.ID
		case target.ProfileVisibility == nil:
			return errInvalidSocialRelationshipRepository
		case *target.ProfileVisibility == identity.ProfileVisibilityPrivate:
			requestID = repository.nextID()
			if requestID == "" {
				return errInvalidSocialRelationshipRepository
			}
			request := socialFollowRequestModel{
				ID: requestID, RequesterUserID: command.ActorUserID,
				TargetUserID: target.ID, CreatedAt: command.OccurredAt,
			}
			if err := tx.Create(&request).Error; err != nil {
				return err
			}
			if err := repository.createSocialNotification(tx, "follow_request_received", target.ID, command.ActorUserID, command.ActorUserID, requestID, command.OccurredAt, true); err != nil {
				return err
			}
			state = domain.RelationshipRequested
			changed = true
		case *target.ProfileVisibility == identity.ProfileVisibilityPublic:
			changeID := repository.nextID()
			if changeID == "" {
				return errInvalidSocialRelationshipRepository
			}
			authorizationChange, err = repository.enqueueFollowerAuthorization(tx, changeID, target.ID, command.ActorUserID, command.ActorUserID, ports.AuthorizationTouch, command.OccurredAt)
			if err != nil {
				return err
			}
			if err := tx.Create(&socialFollowModel{
				FollowerUserID: command.ActorUserID, FollowingUserID: target.ID,
				AuthorizationChangeID: &changeID, CreatedAt: command.OccurredAt,
			}).Error; err != nil {
				return err
			}
			if err := repository.createSocialNotification(tx, "new_follower", target.ID, command.ActorUserID, command.ActorUserID, "", command.OccurredAt, true); err != nil {
				return err
			}
			changed = true
		default:
			return errInvalidSocialRelationshipRepository
		}

		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		if err := createSocialReplay(tx, command, target.ID, requestID, state, changed, authorizationChange.ID); err != nil {
			return err
		}
		profile, err := socialRelationshipProfile(tx, command.ActorUserID, target.ID)
		if err != nil {
			return err
		}
		profile.Relationship = state
		result = socialapp.RelationshipResult{Target: profile, RequestID: requestID, Changed: changed, AuthorizationChange: authorizationChange}
		return nil
	})
	return result, classifySocialRelationshipError(err)
}

func (repository *SocialRelationshipRepository) CancelRequest(ctx context.Context, command socialapp.RelationshipCommand) (socialapp.RelationshipResult, error) {
	return repository.endByTarget(ctx, command, socialapp.CancelFollowRequestOperation)
}

func (repository *SocialRelationshipRepository) Unfollow(ctx context.Context, command socialapp.RelationshipCommand) (socialapp.RelationshipResult, error) {
	return repository.endByTarget(ctx, command, socialapp.UnfollowOperation)
}

func (repository *SocialRelationshipRepository) endByTarget(ctx context.Context, command socialapp.RelationshipCommand, operation string) (socialapp.RelationshipResult, error) {
	if !repository.validCommand(command, operation, true) {
		return socialapp.RelationshipResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.RelationshipResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialIdempotency(tx, command); err != nil {
			return err
		}
		target, err := lockVisibleSocialTarget(tx, command.ActorUserID, command.TargetUsername)
		if err != nil {
			return err
		}
		if replay, found, err := socialReplay(tx, command); err != nil {
			return err
		} else if found {
			profile, err := socialRelationshipProfile(tx, command.ActorUserID, target.ID)
			if err != nil {
				return err
			}
			profile.Relationship = domain.RelationshipNone
			change, err := replayAuthorizationChange(tx, replay, command.OccurredAt, repository.authorizationWorker, repository.authorizationLease)
			if err != nil {
				return err
			}
			result = socialapp.RelationshipResult{Target: profile, RequestID: replayRequestID(replay), Changed: replay.Changed, AuthorizationChange: change, Replayed: true}
			return nil
		}

		requestID := ""
		var authorizationChange ports.AuthorizationChange
		if operation == socialapp.CancelFollowRequestOperation {
			var request socialFollowRequestModel
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where(
				"requester_user_id = ? AND target_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
				command.ActorUserID, target.ID,
			).First(&request).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errSocialRelationshipUnavailable
				}
				return err
			}
			updated := tx.Model(&socialFollowRequestModel{}).Where(
				"id = ? AND requester_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
				request.ID, command.ActorUserID,
			).Update("canceled_at", command.OccurredAt)
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return errSocialRelationshipUnavailable
			}
			requestID = request.ID
		} else {
			deleted := tx.Where("follower_user_id = ? AND following_user_id = ?", command.ActorUserID, target.ID).
				Delete(&socialFollowModel{})
			if deleted.Error != nil {
				return deleted.Error
			}
			if deleted.RowsAffected != 1 {
				return errSocialRelationshipUnavailable
			}
			changeID := repository.nextID()
			if changeID == "" {
				return errInvalidSocialRelationshipRepository
			}
			authorizationChange, err = repository.enqueueFollowerAuthorization(tx, changeID, target.ID, command.ActorUserID, command.ActorUserID, ports.AuthorizationDelete, command.OccurredAt)
			if err != nil {
				return err
			}
		}

		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		if err := createSocialReplay(tx, command, target.ID, requestID, domain.RelationshipNone, true, authorizationChange.ID); err != nil {
			return err
		}
		profile, err := socialRelationshipProfile(tx, command.ActorUserID, target.ID)
		if err != nil {
			return err
		}
		profile.Relationship = domain.RelationshipNone
		result = socialapp.RelationshipResult{Target: profile, RequestID: requestID, Changed: true, AuthorizationChange: authorizationChange}
		return nil
	})
	return result, classifySocialRelationshipError(err)
}

func (repository *SocialRelationshipRepository) AcceptRequest(ctx context.Context, command socialapp.RelationshipCommand) (socialapp.ReviewResult, error) {
	return repository.review(ctx, command, socialapp.AcceptFollowRequestOperation)
}

func (repository *SocialRelationshipRepository) RejectRequest(ctx context.Context, command socialapp.RelationshipCommand) (socialapp.ReviewResult, error) {
	return repository.review(ctx, command, socialapp.RejectFollowRequestOperation)
}

func (repository *SocialRelationshipRepository) review(ctx context.Context, command socialapp.RelationshipCommand, operation string) (socialapp.ReviewResult, error) {
	if !repository.validCommand(command, operation, false) {
		return socialapp.ReviewResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.ReviewResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockSocialIdempotency(tx, command); err != nil {
			return err
		}
		request, err := lockVisibleIncomingSocialRequest(tx, command.ActorUserID, command.RequestID)
		if err != nil {
			return err
		}
		if replay, found, err := socialReplay(tx, command); err != nil {
			return err
		} else if found {
			projection, err := socialFollowRequestProjection(tx, command.ActorUserID, request)
			if err != nil {
				return err
			}
			decision := socialapp.FollowRequestAccepted
			if operation == socialapp.RejectFollowRequestOperation {
				decision = socialapp.FollowRequestRejected
			}
			change, err := replayAuthorizationChange(tx, replay, command.OccurredAt, repository.authorizationWorker, repository.authorizationLease)
			if err != nil {
				return err
			}
			result = socialapp.ReviewResult{Request: projection, Decision: decision, AuthorizationChange: change, Replayed: true}
			return nil
		}
		if request.AcceptedAt != nil || request.RejectedAt != nil || request.CanceledAt != nil {
			return errSocialRelationshipUnavailable
		}

		column := "rejected_at"
		if operation == socialapp.AcceptFollowRequestOperation {
			column = "accepted_at"
		}
		updated := tx.Model(&socialFollowRequestModel{}).Where(
			"id = ? AND target_user_id = ? AND accepted_at IS NULL AND rejected_at IS NULL AND canceled_at IS NULL",
			request.ID, command.ActorUserID,
		).Update(column, command.OccurredAt)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errSocialRelationshipUnavailable
		}

		state := domain.RelationshipNone
		var authorizationChange ports.AuthorizationChange
		if operation == socialapp.AcceptFollowRequestOperation {
			changeID := repository.nextID()
			if changeID == "" {
				return errInvalidSocialRelationshipRepository
			}
			authorizationChange, err = repository.enqueueFollowerAuthorization(tx, changeID, request.TargetUserID, request.RequesterUserID, command.ActorUserID, ports.AuthorizationTouch, command.OccurredAt)
			if err != nil {
				return err
			}
			if err := tx.Create(&socialFollowModel{
				FollowerUserID: request.RequesterUserID, FollowingUserID: request.TargetUserID,
				AuthorizationChangeID: &changeID, CreatedAt: command.OccurredAt,
			}).Error; err != nil {
				return err
			}
			if err := repository.createSocialNotification(tx, "follow_request_accepted", request.RequesterUserID, command.ActorUserID, request.TargetUserID, request.ID, command.OccurredAt, true); err != nil {
				return err
			}
			state = domain.RelationshipFollowing
		}

		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		if err := createSocialReplay(tx, command, request.TargetUserID, request.ID, state, operation == socialapp.AcceptFollowRequestOperation, authorizationChange.ID); err != nil {
			return err
		}
		request, err = socialFollowRequestByID(tx, command.ActorUserID, request.ID, false)
		if err != nil {
			return err
		}
		projection, err := socialFollowRequestProjection(tx, command.ActorUserID, request)
		if err != nil {
			return err
		}
		decision := socialapp.FollowRequestAccepted
		if operation == socialapp.RejectFollowRequestOperation {
			decision = socialapp.FollowRequestRejected
		}
		result = socialapp.ReviewResult{Request: projection, Decision: decision, AuthorizationChange: authorizationChange}
		return nil
	})
	return result, classifySocialRelationshipError(err)
}

func (repository *SocialRelationshipRepository) ListIncoming(ctx context.Context, viewer string, page ports.PageRequest) (socialapp.FollowRequestPage, error) {
	if !repository.ready() || strings.TrimSpace(viewer) == "" || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() ||
		(page.AfterID == "") != page.AfterCreated.IsZero() || (!page.AfterCreated.IsZero() && page.AfterCreated.After(page.Snapshot)) {
		return socialapp.FollowRequestPage{}, ports.ErrInvalidArgument
	}
	query := repository.db.WithContext(ctx).Table("follow_request_models AS requests").
		Select("requests.*").
		Joins("JOIN user_models AS requester ON requester.id = requests.requester_user_id AND requester.status = ?", identity.StatusActive).
		Joins("JOIN user_models AS recipient ON recipient.id = requests.target_user_id AND recipient.status = ?", identity.StatusActive).
		Where("requests.target_user_id = ? AND requests.created_at <= ?", viewer, page.Snapshot).
		Where("requests.accepted_at IS NULL AND requests.rejected_at IS NULL AND requests.canceled_at IS NULL").
		Where(`NOT EXISTS (SELECT 1 FROM block_models relationship_block
			WHERE (relationship_block.blocker_user_id = requests.requester_user_id AND relationship_block.blocked_user_id = requests.target_user_id)
			   OR (relationship_block.blocker_user_id = requests.target_user_id AND relationship_block.blocked_user_id = requests.requester_user_id))`)
	if page.AfterID != "" {
		query = query.Where("requests.created_at < ? OR (requests.created_at = ? AND requests.id < ?)", page.AfterCreated, page.AfterCreated, page.AfterID)
	}
	var rows []socialFollowRequestModel
	if err := query.Order("requests.created_at DESC, requests.id DESC").Limit(page.Limit + 1).Find(&rows).Error; err != nil {
		return socialapp.FollowRequestPage{}, fmt.Errorf("list incoming follow requests: %w: %v", ports.ErrUnavailable, err)
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	requests := make([]domain.FollowRequest, len(rows))
	for index, row := range rows {
		projection, err := socialFollowRequestProjection(repository.db.WithContext(ctx), viewer, row)
		if err != nil {
			return socialapp.FollowRequestPage{}, err
		}
		requests[index] = projection
	}
	return socialapp.FollowRequestPage{Requests: requests, HasMore: hasMore}, nil
}

func (repository *SocialRelationshipRepository) ready() bool {
	return repository != nil && repository.db != nil && repository.newID != nil &&
		strings.TrimSpace(repository.authorizationWorker) != "" && repository.authorizationLease > 0
}

func (repository *SocialRelationshipRepository) validCommand(command socialapp.RelationshipCommand, operation string, targetByUsername bool) bool {
	if !repository.ready() || strings.TrimSpace(command.ActorUserID) == "" || command.OccurredAt.IsZero() || command.OccurredAt.Location() != time.UTC ||
		command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != operation ||
		strings.TrimSpace(command.Idempotency.Key) == "" || len(command.Idempotency.RequestHash) != 32 || !command.Audit.Valid() ||
		command.Audit.OwnerUserID != command.ActorUserID || command.Audit.ActorUserID != command.ActorUserID ||
		command.Audit.Outcome != "succeeded" || !command.Audit.OccurredAt.Equal(command.OccurredAt) {
		return false
	}
	if targetByUsername {
		return command.TargetUsername != "" && strings.TrimSpace(command.TargetUsername) == command.TargetUsername && command.RequestID == ""
	}
	return command.RequestID != "" && strings.TrimSpace(command.RequestID) == command.RequestID && command.TargetUsername == ""
}

func (repository *SocialRelationshipRepository) nextID() string {
	id := repository.newID()
	if strings.TrimSpace(id) != id {
		return ""
	}
	return id
}

func lockSocialIdempotency(tx *gorm.DB, command socialapp.RelationshipCommand) error {
	return lockSocialMutationIdempotency(tx, "social-idempotency", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key)
}

func lockSocialMutationIdempotency(tx *gorm.DB, namespace string, parts ...string) error {
	key := socialLockKey(namespace, parts...)
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock
}

func lockSocialPair(tx *gorm.DB, first, second string) error {
	ids := []string{first, second}
	sort.Strings(ids)
	if err := lockSocialInteractionOwners(tx, ids); err != nil {
		return err
	}
	key := socialLockKey("social-user-pair", ids[0], ids[1])
	if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Error; err != nil { // hourpaths-direct-sql: allow deterministic PostgreSQL transaction advisory lock
		return err
	}
	var locked []socialRelationshipUser
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_models").Where("id IN ? AND status = ?", ids, identity.StatusActive).Order("id ASC").Find(&locked).Error; err != nil {
		return err
	}
	if len(locked) != 2 {
		return ports.ErrNotFound
	}
	return nil
}

func lockVisibleSocialTarget(tx *gorm.DB, actor, username string) (socialRelationshipUser, error) {
	var candidate socialRelationshipUser
	if err := tx.Table("user_models").Select("id").Where("status = ? AND lower(username) = ?", identity.StatusActive, username).Take(&candidate).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return socialRelationshipUser{}, ports.ErrNotFound
		}
		return socialRelationshipUser{}, err
	}
	if candidate.ID == actor {
		return socialRelationshipUser{}, ports.ErrNotFound
	}
	if err := lockSocialPair(tx, actor, candidate.ID); err != nil {
		return socialRelationshipUser{}, err
	}
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Table("user_models").
		Where("id = ? AND status = ? AND lower(username) = ?", candidate.ID, identity.StatusActive, username).
		Take(&candidate).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return socialRelationshipUser{}, ports.ErrNotFound
		}
		return socialRelationshipUser{}, err
	}
	blocked, err := socialPairBlocked(tx, actor, candidate.ID)
	if err != nil {
		return socialRelationshipUser{}, err
	}
	if blocked || candidate.Username == nil || candidate.ProfileVisibility == nil {
		return socialRelationshipUser{}, ports.ErrNotFound
	}
	return candidate, nil
}

func socialPairBlocked(tx *gorm.DB, first, second string) (bool, error) {
	var count int64
	err := tx.Table("block_models").Where(
		"(blocker_user_id = ? AND blocked_user_id = ?) OR (blocker_user_id = ? AND blocked_user_id = ?)",
		first, second, second, first,
	).Count(&count).Error
	return count != 0, err
}

func socialReplay(tx *gorm.DB, command socialapp.RelationshipCommand) (socialRelationshipReplayModel, bool, error) {
	var replay socialRelationshipReplayModel
	err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).First(&replay).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return socialRelationshipReplayModel{}, false, nil
	}
	if err != nil {
		return socialRelationshipReplayModel{}, false, err
	}
	if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) {
		return socialRelationshipReplayModel{}, false, ports.ErrIdempotencyConflict
	}
	return replay, true, nil
}

func createSocialReplay(tx *gorm.DB, command socialapp.RelationshipCommand, targetID, requestID string, state domain.RelationshipState, changed bool, authorizationChangeID string) error {
	row := socialRelationshipReplayModel{
		ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation,
		IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...),
		TargetUserID: targetID, ResultState: string(state), Changed: changed, CreatedAt: command.OccurredAt,
	}
	if requestID != "" {
		row.FollowRequestID = &requestID
	}
	if authorizationChangeID != "" {
		row.AuthorizationChangeID = &authorizationChangeID
	}
	return tx.Create(&row).Error
}

func replayRequestID(replay socialRelationshipReplayModel) string {
	if replay.FollowRequestID == nil {
		return ""
	}
	return *replay.FollowRequestID
}

func replayAuthorizationChange(tx *gorm.DB, replay socialRelationshipReplayModel, occurredAt time.Time, worker string, lease time.Duration) (ports.AuthorizationChange, error) {
	if replay.AuthorizationChangeID == nil {
		return ports.AuthorizationChange{}, nil
	}
	var row authorizationOutboxModel
	if err := tx.Where("id = ?", *replay.AuthorizationChangeID).First(&row).Error; err != nil {
		return ports.AuthorizationChange{}, err
	}
	return socialAuthorizationChange(row, occurredAt, worker, lease), nil
}

func socialRelationshipProfile(tx *gorm.DB, viewer, targetID string) (domain.PublicProfile, error) {
	var row relationshipProfileRow
	result := tx.Table("user_models AS u").Select(fmt.Sprintf(`u.id, u.username, u.display_name,
  COALESCE(u.profile_picture_url, '') AS profile_picture_url,
  COALESCE(u.description, '') AS description,
  %s,
  CASE
    WHEN u.id = ? THEN 'self'
    WHEN EXISTS (SELECT 1 FROM follow_models viewer_follow WHERE viewer_follow.follower_user_id = ? AND viewer_follow.following_user_id = u.id) THEN 'following'
    WHEN EXISTS (SELECT 1 FROM follow_request_models viewer_request WHERE viewer_request.requester_user_id = ? AND viewer_request.target_user_id = u.id AND viewer_request.accepted_at IS NULL AND viewer_request.rejected_at IS NULL AND viewer_request.canceled_at IS NULL) THEN 'requested'
    ELSE 'none'
	  END AS relationship`, socialCountsSQL), viewer, viewer, viewer).
		Where("u.id = ? AND u.status = ?", targetID, identity.StatusActive).
		Where(`NOT EXISTS (SELECT 1 FROM block_models viewer_block
			WHERE (viewer_block.blocker_user_id = ? AND viewer_block.blocked_user_id = u.id)
			   OR (viewer_block.blocker_user_id = u.id AND viewer_block.blocked_user_id = ?))`, viewer, viewer).
		Take(&row)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return domain.PublicProfile{}, ports.ErrNotFound
		}
		return domain.PublicProfile{}, result.Error
	}
	profile := domain.PublicProfile{
		ID: row.ID, Username: row.Username, DisplayName: row.DisplayName,
		ProfilePictureURL: row.ProfilePictureURL, Description: row.Description,
		FollowerCount: row.FollowerCount, FollowingCount: row.FollowingCount,
		Relationship: row.Relationship,
	}
	if !profile.Valid() {
		return domain.PublicProfile{}, errInvalidSocialRelationshipRepository
	}
	return profile, nil
}

func lockVisibleIncomingSocialRequest(tx *gorm.DB, recipient, requestID string) (socialFollowRequestModel, error) {
	request, err := socialFollowRequestByID(tx, recipient, requestID, false)
	if err != nil {
		return socialFollowRequestModel{}, err
	}
	if err := lockSocialPair(tx, request.RequesterUserID, request.TargetUserID); err != nil {
		return socialFollowRequestModel{}, ports.ErrNotFound
	}
	request, err = socialFollowRequestByID(tx, recipient, requestID, true)
	if err != nil {
		return socialFollowRequestModel{}, err
	}
	blocked, err := socialPairBlocked(tx, request.RequesterUserID, request.TargetUserID)
	if err != nil {
		return socialFollowRequestModel{}, err
	}
	if blocked {
		return socialFollowRequestModel{}, ports.ErrNotFound
	}
	return request, nil
}

func socialFollowRequestByID(tx *gorm.DB, recipient, requestID string, lock bool) (socialFollowRequestModel, error) {
	query := tx.Table("follow_request_models AS requests").Select("requests.*").
		Joins("JOIN user_models requester ON requester.id = requests.requester_user_id AND requester.status = ?", identity.StatusActive).
		Joins("JOIN user_models target_user ON target_user.id = requests.target_user_id AND target_user.status = ?", identity.StatusActive).
		Where("requests.id = ? AND requests.target_user_id = ?", requestID, recipient)
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE", Table: clause.Table{Name: "requests"}})
	}
	var request socialFollowRequestModel
	if err := query.Take(&request).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return socialFollowRequestModel{}, ports.ErrNotFound
		}
		return socialFollowRequestModel{}, err
	}
	return request, nil
}

func socialFollowRequestProjection(tx *gorm.DB, viewer string, row socialFollowRequestModel) (domain.FollowRequest, error) {
	profile, err := socialRelationshipProfile(tx, viewer, row.RequesterUserID)
	if err != nil {
		return domain.FollowRequest{}, err
	}
	request := domain.FollowRequest{ID: row.ID, RecipientUserID: row.TargetUserID, Requester: profile, CreatedAt: row.CreatedAt.UTC()}
	if !request.Valid() {
		return domain.FollowRequest{}, errInvalidSocialRelationshipRepository
	}
	return request, nil
}

func (repository *SocialRelationshipRepository) enqueueFollowerAuthorization(tx *gorm.DB, id, target, follower, actor string, operation ports.AuthorizationOperation, occurredAt time.Time) (ports.AuthorizationChange, error) {
	row := authorizationOutboxModel{
		ID: id, ResourceType: "user", ResourceID: target, Relation: "follower",
		SubjectType: "user", SubjectID: follower, OwnerUserID: target,
		ActorUserID: actor, Operation: operation, LockedBy: repository.authorizationWorker,
		CreatedAt: occurredAt,
	}
	if err := tx.Create(&row).Error; err != nil {
		return ports.AuthorizationChange{}, err
	}
	if err := tx.Model(&row).Update("locked_until", gorm.Expr(
		"CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", repository.authorizationLease.Milliseconds(),
	)).Error; err != nil {
		return ports.AuthorizationChange{}, err
	}
	return socialAuthorizationChange(row, occurredAt, repository.authorizationWorker, repository.authorizationLease), nil
}

func socialAuthorizationChange(row authorizationOutboxModel, occurredAt time.Time, worker string, lease time.Duration) ports.AuthorizationChange {
	return ports.AuthorizationChange{
		ID: row.ID, ResourceType: row.ResourceType, ResourceID: row.ResourceID,
		Relation: row.Relation, SubjectType: row.SubjectType, SubjectID: row.SubjectID,
		OwnerUserID: row.OwnerUserID, ActorUserID: row.ActorUserID, Operation: row.Operation,
		LockedBy: worker, LockedUntil: occurredAt.Add(lease), Lease: lease,
	}
}

func (repository *SocialRelationshipRepository) AuthorizationChangeState(ctx context.Context, id string) (socialapp.AuthorizationChangeState, error) {
	if !repository.ready() || strings.TrimSpace(id) == "" {
		return "", ports.ErrInvalidArgument
	}
	var row socialAuthorizationChangeStateRow
	if err := repository.db.WithContext(ctx).Table("authorization_outbox_models").
		Select("id, locked_by, locked_until, completed_at, dead_lettered_at, (locked_until > CURRENT_TIMESTAMP) AS lock_active").
		Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ports.ErrNotFound
		}
		return "", err
	}
	switch {
	case row.DeadLetteredAt != nil:
		return socialapp.AuthorizationChangeDeadLettered, nil
	case row.CompletedAt != nil:
		return socialapp.AuthorizationChangeCompleted, nil
	case row.LockedBy != "" && row.LockedUntil != nil && row.LockActive:
		return socialapp.AuthorizationChangeLocked, nil
	default:
		return socialapp.AuthorizationChangePending, nil
	}
}

func (repository *SocialRelationshipRepository) createSocialNotification(tx *gorm.DB, kind, recipient, actor, subject, requestID string, createdAt time.Time, push bool) error {
	notificationID := repository.nextID()
	if notificationID == "" {
		return errInvalidSocialRelationshipRepository
	}
	presentation := "informational"
	if kind == "follow_request_received" {
		presentation = "actionable"
	}
	row := map[string]any{
		"id": notificationID, "recipient_user_id": recipient, "actor_user_id": actor,
		"path_id": nil, "path_invitation_id": nil, "path_ownership_transfer_id": nil,
		"follow_request_id": nil, "follow_subject_user_id": subject,
		"kind": kind, "presentation_class": presentation, "channel": "following",
		"offered_role": nil, "created_at": createdAt,
	}
	if requestID != "" {
		row["follow_request_id"] = requestID
	}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return err
	}
	if !push {
		return nil
	}
	if err := tx.Table("notification_push_outbox_models").Create(map[string]any{
		"notification_id": notificationID, "created_at": createdAt,
	}).Error; err != nil {
		return err
	}
	if err := tx.Exec(`
INSERT INTO notification_push_delivery_models (
  notification_id, installation_id, recipient_user_id,
  provider, platform, locale, token_ciphertext, token_nonce, token_hash,
  available_at, created_at
)
SELECT ?, i.id, ?, i.provider, i.platform, i.locale,
       i.token_ciphertext, i.token_nonce, i.token_hash, ?, ?
FROM push_installation_models i
WHERE i.owner_user_id = ? AND i.deleted_at IS NULL
ON CONFLICT (notification_id, installation_id) DO NOTHING`,
		notificationID, recipient, createdAt, createdAt, recipient).Error; err != nil { // hourpaths-direct-sql: allow transactional notification fanout
		return err
	}
	return tx.Exec(`
UPDATE notification_push_outbox_models
SET suppressed_at = ?, failure_code = 'no_active_installation'
WHERE notification_id = ?
  AND NOT EXISTS (
    SELECT 1 FROM notification_push_delivery_models
    WHERE notification_id = ?
  )`, createdAt, notificationID, notificationID).Error // hourpaths-direct-sql: allow transactional notification suppression
}

func classifySocialRelationshipError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, ports.ErrInvalidArgument):
		return err
	case errors.Is(err, errSocialRelationshipUnavailable), errors.Is(err, gorm.ErrDuplicatedKey), errors.Is(err, gorm.ErrForeignKeyViolated):
		return ports.ErrConflict
	default:
		return fmt.Errorf("social relationship persistence: %w: %v", ports.ErrUnavailable, err)
	}
}
