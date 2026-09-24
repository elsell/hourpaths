package gormstore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/progresslock"
	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	socialapp "github.com/elsell/hour-paths/apps/api/internal/app/social"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	pathdomain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type pathNudgePreferenceModel struct {
	PathID, UserID       string
	Audience             string
	Revision             int64
	CreatedAt, UpdatedAt time.Time
}

func (pathNudgePreferenceModel) TableName() string { return "path_nudge_preference_models" }

type pathNudgePreferenceReplayModel struct {
	ActorUserID, Operation, IdempotencyKey, PathID, ResultAudience string
	RequestHash                                                    []byte
	ResultRevision                                                 int64
	ResultUpdatedAt, CreatedAt                                     time.Time
}

func (pathNudgePreferenceReplayModel) TableName() string {
	return "path_nudge_preference_replay_models"
}

type socialNudgeModel struct {
	ID, SenderUserID, RecipientUserID, PathID, ContentKind, Preset string
	SentAt                                                         time.Time
	IntervalStartedAt, IntervalEndedAt                             *time.Time
}

func (socialNudgeModel) TableName() string { return "social_nudge_models" }

type socialNudgeReplayModel struct {
	ActorUserID, Operation, IdempotencyKey, NudgeID string
	RecipientUserID, PathID, Preset                 string
	RequestHash                                     []byte
	ResultNotificationCreated                       bool
	ResultSentAt, CreatedAt                         time.Time
}

func (socialNudgeReplayModel) TableName() string { return "social_nudge_replay_models" }

func (repository *SocialFeedRepository) GetNudgeAudience(ctx context.Context, actor, pathID string) (socialapp.NudgeAudiencePreference, error) {
	if !validOpaquePersistenceID(actor) || !validOpaquePersistenceID(pathID) || repository == nil || repository.db == nil {
		return socialapp.NudgeAudiencePreference{}, ports.ErrInvalidArgument
	}
	var row struct {
		Audience  *string
		Revision  *int64
		UpdatedAt *time.Time
	}
	err := repository.db.WithContext(ctx).Table("path_models AS path").
		Select("preference.audience, preference.revision, preference.updated_at").
		Joins("JOIN user_models actor ON actor.id = ? AND actor.status = 'active'", actor).
		Joins("LEFT JOIN path_membership_models membership ON membership.path_id = path.id AND membership.user_id = ?", actor).
		Joins("LEFT JOIN path_nudge_preference_models preference ON preference.path_id = path.id AND preference.user_id = ?", actor).
		Where("path.id = ? AND path.archived_at IS NULL AND (path.owner_user_id = ? OR membership.role IN ('administrator', 'participant'))", pathID, actor).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return socialapp.NudgeAudiencePreference{}, ports.ErrNotFound
	}
	if err != nil {
		return socialapp.NudgeAudiencePreference{}, classifyNudgePersistenceError(err)
	}
	if row.Audience == nil && row.Revision == nil && row.UpdatedAt == nil {
		return socialapp.NudgeAudiencePreference{PathID: pathID, UserID: actor, Audience: socialdomain.DefaultNudgeAudience}, nil
	}
	if row.Audience == nil || row.Revision == nil || row.UpdatedAt == nil {
		return socialapp.NudgeAudiencePreference{}, classifyNudgePersistenceError(errors.New("persisted nudge preference is partial"))
	}
	result := socialapp.NudgeAudiencePreference{PathID: pathID, UserID: actor, Audience: socialdomain.NudgeAudience(*row.Audience), Revision: *row.Revision, UpdatedAt: row.UpdatedAt.UTC()}
	if !result.Audience.Valid() || result.Revision < 1 {
		return socialapp.NudgeAudiencePreference{}, classifyNudgePersistenceError(errors.New("persisted nudge preference is invalid"))
	}
	return result, nil
}

func (repository *SocialFeedRepository) UpdateNudgeAudience(ctx context.Context, command socialapp.NudgePreferenceCommand) (socialapp.NudgeAudiencePreference, error) {
	if !validNudgePreferenceCommand(repository, command) {
		return socialapp.NudgeAudiencePreference{}, ports.ErrInvalidArgument
	}
	var result socialapp.NudgeAudiencePreference
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNudgePreference(tx, command.ActorUserID, command.PathID); err != nil {
			return err
		}
		var replay pathNudgePreferenceReplayModel
		err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) || replay.PathID != command.PathID {
				return ports.ErrIdempotencyConflict
			}
			result = socialapp.NudgeAudiencePreference{PathID: replay.PathID, UserID: replay.ActorUserID, Audience: socialdomain.NudgeAudience(replay.ResultAudience), Revision: replay.ResultRevision, UpdatedAt: replay.ResultUpdatedAt.UTC()}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		current, err := nudgeAudienceForUpdate(ctx, tx, command.ActorUserID, command.PathID)
		if err != nil {
			return err
		}
		if current.Revision != command.ExpectedRevision {
			return ports.ErrConflict
		}
		next := pathNudgePreferenceModel{PathID: command.PathID, UserID: command.ActorUserID, Audience: string(command.Audience), Revision: current.Revision + 1, CreatedAt: command.ChangedAt, UpdatedAt: command.ChangedAt}
		if current.Revision == 0 {
			if err := tx.Create(&next).Error; err != nil {
				return err
			}
		} else {
			updated := tx.Model(&pathNudgePreferenceModel{}).Where("path_id = ? AND user_id = ? AND revision = ?", command.PathID, command.ActorUserID, current.Revision).
				Updates(map[string]any{"audience": next.Audience, "revision": next.Revision, "updated_at": next.UpdatedAt})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ports.ErrConflict
			}
			next.CreatedAt = current.UpdatedAt
		}
		replay = pathNudgePreferenceReplayModel{ActorUserID: command.ActorUserID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), PathID: command.PathID, ResultAudience: next.Audience, ResultRevision: next.Revision, ResultUpdatedAt: next.UpdatedAt, CreatedAt: command.ChangedAt}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result = socialapp.NudgeAudiencePreference{PathID: command.PathID, UserID: command.ActorUserID, Audience: command.Audience, Revision: next.Revision, UpdatedAt: command.ChangedAt}
		return nil
	})
	return result, classifyNudgePersistenceError(err)
}

func nudgeAudienceForUpdate(ctx context.Context, tx *gorm.DB, actor, pathID string) (socialapp.NudgeAudiencePreference, error) {
	repository := &SocialFeedRepository{db: tx}
	return repository.GetNudgeAudience(ctx, actor, pathID)
}

func (repository *SocialFeedRepository) GetNudgeEligibility(ctx context.Context, sender, recipient, pathID string, at time.Time) (socialapp.NudgeEligibility, error) {
	if !validNudgeQuery(repository, sender, recipient, pathID, at) {
		return socialapp.NudgeEligibility{}, ports.ErrInvalidArgument
	}
	state, err := resolveNudgeState(repository.db.WithContext(ctx), sender, recipient, pathID, at)
	if err != nil {
		return socialapp.NudgeEligibility{}, classifyNudgePersistenceError(err)
	}
	if !state.AudienceAllowed {
		return socialapp.NudgeEligibility{}, socialapp.ErrNudgeAudienceDenied
	}
	return eligibilityFromNudgeState(state, pathID, recipient), nil
}

func (repository *SocialFeedRepository) SendNudge(ctx context.Context, command socialapp.NudgeCommand) (socialapp.NudgeMutationResult, error) {
	if !validNudgeCommand(repository, command) {
		return socialapp.NudgeMutationResult{}, ports.ErrInvalidArgument
	}
	var result socialapp.NudgeMutationResult
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nudge := command.Nudge
		if err := lockSocialPathAudience(tx, nudge.PathID); err != nil {
			return err
		}
		// Progress is the outermost aggregate lock. Activity mutations use the
		// same ordering, which prevents a nudge from holding social user rows
		// while waiting for an activity transaction that needs those rows for
		// foreign-key validation.
		if err := progresslock.Lock(tx, nudge.RecipientID, nudge.PathID); err != nil {
			return err
		}
		if err := lockNudgePair(tx, nudge.SenderID, nudge.RecipientID, nudge.PathID); err != nil {
			return err
		}
		var replay socialNudgeReplayModel
		err := tx.Where("actor_user_id = ? AND operation = ? AND idempotency_key = ?", nudge.SenderID, command.Idempotency.Operation, command.Idempotency.Key).Take(&replay).Error
		if err == nil {
			if !bytes.Equal(replay.RequestHash, command.Idempotency.RequestHash) || replay.PathID != nudge.PathID || replay.RecipientUserID != nudge.RecipientID || replay.Preset != string(nudge.Content.Preset) {
				return ports.ErrIdempotencyConflict
			}
			persisted, err := readPersistedNudge(tx, replay.NudgeID)
			if err != nil {
				return err
			}
			result = socialapp.NudgeMutationResult{Nudge: persisted, Replayed: true}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := lockNudgePathParticipants(tx, nudge.SenderID, nudge.RecipientID, nudge.PathID); err != nil {
			return err
		}
		state, err := resolveNudgeState(tx, nudge.SenderID, nudge.RecipientID, nudge.PathID, nudge.SentAt)
		if err != nil {
			return err
		}
		if !state.AudienceAllowed {
			return socialapp.ErrNudgeAudienceDenied
		}
		if state.GoalComplete {
			return socialapp.ErrNudgeGoalComplete
		}
		if state.RateLimited {
			return socialapp.ErrNudgeAlreadySent
		}
		row := socialNudgeModel{ID: nudge.ID, SenderUserID: nudge.SenderID, RecipientUserID: nudge.RecipientID, PathID: nudge.PathID, ContentKind: string(nudge.Content.Kind), Preset: string(nudge.Content.Preset), SentAt: nudge.SentAt, IntervalStartedAt: state.IntervalStartedAt, IntervalEndedAt: state.IntervalEndedAt}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		notificationCreated, err := createNudgeNotification(tx, nudge)
		if err != nil {
			return err
		}
		replay = socialNudgeReplayModel{ActorUserID: nudge.SenderID, Operation: command.Idempotency.Operation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), NudgeID: nudge.ID, RecipientUserID: nudge.RecipientID, PathID: nudge.PathID, Preset: string(nudge.Content.Preset), ResultNotificationCreated: notificationCreated, ResultSentAt: nudge.SentAt, CreatedAt: nudge.SentAt}
		if err := tx.Create(&replay).Error; err != nil {
			return err
		}
		if err := appendAuditEvent(tx, command.Audit); err != nil {
			return err
		}
		result.Nudge = nudge
		return nil
	})
	return result, classifyNudgePersistenceError(err)
}

type nudgeState struct {
	AudienceAllowed                    bool
	GoalComplete, RateLimited          bool
	IntervalStartedAt, IntervalEndedAt *time.Time
}

type nudgeStateRow struct {
	Audience                                                   *string
	CurrentTimeZone                                            *string
	TargetSeconds                                              *int64
	Recurrence                                                 *string
	StartMinute, StartHour, StartWeekday, StartDay, StartMonth *int16
	SenderIsPathMember, SenderFollowsRecipient                 bool
}

func resolveNudgeState(tx *gorm.DB, sender, recipient, pathID string, at time.Time) (nudgeState, error) {
	var row nudgeStateRow
	err := tx.Table("path_models AS path").Select(`preference.audience, recipient_preferences.current_time_zone,
path.interval_goal_target_seconds AS target_seconds, path.interval_goal_recurrence AS recurrence,
path.interval_goal_start_minute AS start_minute, path.interval_goal_start_hour AS start_hour,
path.interval_goal_start_weekday AS start_weekday, path.interval_goal_start_day AS start_day,
path.interval_goal_start_month AS start_month,
(path.owner_user_id = ? OR EXISTS (SELECT 1 FROM path_membership_models sender_membership WHERE sender_membership.path_id = path.id AND sender_membership.user_id = ?)) AS sender_is_path_member,
EXISTS (SELECT 1 FROM follow_models follow WHERE follow.follower_user_id = ? AND follow.following_user_id = ?) AS sender_follows_recipient`, sender, sender, sender, recipient).
		Joins("JOIN user_models sender ON sender.id = ? AND sender.status = 'active'", sender).
		Joins("JOIN user_models recipient ON recipient.id = ? AND recipient.status = 'active'", recipient).
		Joins("LEFT JOIN path_membership_models recipient_membership ON recipient_membership.path_id = path.id AND recipient_membership.user_id = ?", recipient).
		Joins("LEFT JOIN path_nudge_preference_models preference ON preference.path_id = path.id AND preference.user_id = ?", recipient).
		Joins("LEFT JOIN user_preference_models recipient_preferences ON recipient_preferences.user_id = ?", recipient).
		Where("path.id = ? AND path.archived_at IS NULL AND recipient_membership.role IN ('administrator', 'participant')", pathID).
		Where(`(path.owner_user_id = ? OR EXISTS (
SELECT 1 FROM path_membership_models sender_membership
WHERE sender_membership.path_id = path.id AND sender_membership.user_id = ?
) OR (
NOT EXISTS (SELECT 1 FROM block_models owner_block WHERE
(owner_block.blocker_user_id = ? AND owner_block.blocked_user_id = path.owner_user_id) OR
(owner_block.blocker_user_id = path.owner_user_id AND owner_block.blocked_user_id = ?))
AND (path.visibility = 'public' OR (path.visibility = 'followers' AND EXISTS (
SELECT 1 FROM follow_models owner_follow
WHERE owner_follow.follower_user_id = ? AND owner_follow.following_user_id = path.owner_user_id
)))
))`, sender, sender, sender, sender, sender).
		Where(`NOT EXISTS (SELECT 1 FROM block_models block WHERE
(block.blocker_user_id = ? AND block.blocked_user_id = ?) OR
(block.blocker_user_id = ? AND block.blocked_user_id = ?))`, sender, recipient, recipient, sender).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nudgeState{}, ports.ErrNotFound
	}
	if err != nil {
		return nudgeState{}, err
	}
	audience := socialdomain.DefaultNudgeAudience
	if row.Audience != nil {
		audience = socialdomain.NudgeAudience(*row.Audience)
	}
	if !audience.Valid() {
		return nudgeState{}, errors.New("persisted nudge audience is invalid")
	}
	state := nudgeState{}
	switch audience {
	case socialdomain.NudgeAudienceNobody:
	case socialdomain.NudgeAudiencePathMembers:
		state.AudienceAllowed = row.SenderIsPathMember
	case socialdomain.NudgeAudienceFollowers:
		state.AudienceAllowed = row.SenderIsPathMember || row.SenderFollowsRecipient
	case socialdomain.NudgeAudienceEveryone:
		state.AudienceAllowed = true
	}
	goal, err := nudgeIntervalGoal(row)
	if err != nil {
		return nudgeState{}, err
	}
	if goal == nil {
		var count int64
		err := tx.Table("social_nudge_models").Where("sender_user_id = ? AND recipient_user_id = ? AND path_id = ? AND interval_started_at IS NULL AND sent_at > ?", sender, recipient, pathID, at.Add(-24*time.Hour)).Count(&count).Error
		state.RateLimited = count > 0
		return state, err
	}
	if row.CurrentTimeZone == nil {
		return nudgeState{}, errors.New("persisted recipient time zone is missing")
	}
	window, err := activityapp.CurrentIntervalWindow(*goal, *row.CurrentTimeZone, at)
	if err != nil {
		return nudgeState{}, errors.New("persisted recipient time zone or interval goal is invalid")
	}
	startedAt, endedAt := window.StartedAt, window.EndedAt
	state.IntervalStartedAt, state.IntervalEndedAt = &startedAt, &endedAt
	var seconds int64
	err = tx.Table("recorded_activity_models").Select(`COALESCE(SUM(
FLOOR(EXTRACT(EPOCH FROM (LEAST(ended_at, ?) - started_at))) -
FLOOR(EXTRACT(EPOCH FROM (GREATEST(started_at, ?) - started_at)))
), 0)::bigint`, endedAt, startedAt).
		Where("participant_id = ? AND path_id = ? AND started_at < ? AND ended_at > ?", recipient, pathID, endedAt, startedAt).Scan(&seconds).Error
	if err != nil {
		return nudgeState{}, err
	}
	state.GoalComplete = seconds >= goal.TargetSeconds
	var count int64
	err = tx.Table("social_nudge_models").Where("sender_user_id = ? AND recipient_user_id = ? AND path_id = ? AND interval_started_at = ?", sender, recipient, pathID, startedAt).Count(&count).Error
	state.RateLimited = count > 0
	return state, err
}

func nudgeIntervalGoal(row nudgeStateRow) (*pathdomain.IntervalGoal, error) {
	absent := row.TargetSeconds == nil && row.Recurrence == nil && row.StartMinute == nil && row.StartHour == nil && row.StartWeekday == nil && row.StartDay == nil && row.StartMonth == nil
	if absent {
		return nil, nil
	}
	if row.TargetSeconds == nil || row.Recurrence == nil {
		return nil, errors.New("persisted interval goal is partial")
	}
	goal := pathdomain.IntervalGoal{Present: true, TargetSeconds: *row.TargetSeconds, Recurrence: pathdomain.Recurrence(*row.Recurrence)}
	switch goal.Recurrence {
	case pathdomain.RecurrenceHourly:
		if row.StartMinute == nil {
			return nil, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Minute = int(*row.StartMinute)
	case pathdomain.RecurrenceDaily:
		if row.StartHour == nil {
			return nil, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Hour = int(*row.StartHour)
	case pathdomain.RecurrenceWeekly:
		if row.StartWeekday == nil {
			return nil, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.ISOWeekday = int(*row.StartWeekday)
	case pathdomain.RecurrenceMonthly:
		if row.StartDay == nil {
			return nil, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Day = int(*row.StartDay)
	case pathdomain.RecurrenceYearly:
		if row.StartDay == nil || row.StartMonth == nil {
			return nil, errors.New("persisted interval goal is invalid")
		}
		goal.Alignment.Day, goal.Alignment.Month = int(*row.StartDay), int(*row.StartMonth)
	default:
		return nil, errors.New("persisted interval goal is invalid")
	}
	return &goal, nil
}

func eligibilityFromNudgeState(state nudgeState, pathID, recipient string) socialapp.NudgeEligibility {
	result := socialapp.NudgeEligibility{PathID: pathID, RecipientUserID: recipient, Eligible: true}
	if state.GoalComplete {
		result.Eligible = false
		result.Reason = socialapp.NudgeGoalCompleteReason
	} else if state.RateLimited {
		result.Eligible = false
		result.Reason = socialapp.NudgeRateLimitedReason
	}
	return result
}

func createNudgeNotification(tx *gorm.DB, nudge socialdomain.Nudge) (bool, error) {
	var enabled bool
	err := tx.Table("user_models AS recipient").Select("COALESCE(preference.enabled, true)").
		Joins("LEFT JOIN notification_channel_preference_models preference ON preference.user_id = recipient.id AND preference.channel = 'nudges'").
		Where("recipient.id = ? AND recipient.status = 'active'", nudge.RecipientID).Take(&enabled).Error
	if err != nil || !enabled {
		return false, err
	}
	notificationID := "nudge:" + nudge.ID
	row := map[string]any{"id": notificationID, "recipient_user_id": nudge.RecipientID, "actor_user_id": nudge.SenderID, "path_id": nudge.PathID, "nudge_id": nudge.ID, "kind": "nudge_received", "presentation_class": "informational", "channel": "nudges", "created_at": nudge.SentAt}
	if err := tx.Table("notification_models").Create(row).Error; err != nil {
		return false, err
	}
	if err := createSocialInteractionPush(tx, notificationID, nudge.RecipientID, nudge.SentAt); err != nil {
		return false, err
	}
	return true, nil
}

func readPersistedNudge(tx *gorm.DB, id string) (socialdomain.Nudge, error) {
	var row socialNudgeModel
	if err := tx.Where("id = ?", id).Take(&row).Error; err != nil {
		return socialdomain.Nudge{}, err
	}
	result := socialdomain.Nudge{ID: row.ID, SenderID: row.SenderUserID, RecipientID: row.RecipientUserID, PathID: row.PathID, Content: socialdomain.NudgeContent{Kind: socialdomain.NudgeContentKind(row.ContentKind), Preset: socialdomain.NudgePreset(row.Preset)}, SentAt: row.SentAt.UTC()}
	if !result.Valid() {
		return socialdomain.Nudge{}, errors.New("persisted nudge is invalid")
	}
	return result, nil
}

func lockNudgePair(tx *gorm.DB, sender, recipient, pathID string) error {
	return lockSocialPairs(tx, [][2]string{{sender, recipient}})
}

func lockNudgePathParticipants(tx *gorm.DB, sender, recipient, pathID string) error {
	var path struct{ OwnerUserID string }
	err := tx.Table("path_models").Select("owner_user_id").Where("id = ? AND archived_at IS NULL", pathID).
		Clauses(clause.Locking{Strength: "SHARE", Table: clause.Table{Name: "path_models"}}).Take(&path).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	if err != nil {
		return err
	}
	var recipientMembership struct{ UserID string }
	err = tx.Table("path_membership_models").Select("user_id").
		Where("path_id = ? AND user_id = ? AND role IN ('administrator', 'participant')", pathID, recipient).
		Clauses(clause.Locking{Strength: "SHARE"}).Take(&recipientMembership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ports.ErrNotFound
	}
	if err != nil {
		return err
	}
	if sender == path.OwnerUserID {
		return nil
	}
	var senderMembership struct{ UserID string }
	return tx.Table("path_membership_models").Select("user_id").
		Where("path_id = ? AND user_id = ?", pathID, sender).
		Clauses(clause.Locking{Strength: "SHARE"}).Find(&senderMembership).Error
}

func lockNudgePreference(tx *gorm.DB, actor, pathID string) error {
	return lockSocialInteractionOwner(tx, socialLockKey("nudge-preference", actor, pathID))
}

func validNudgeQuery(repository *SocialFeedRepository, sender, recipient, pathID string, at time.Time) bool {
	return repository != nil && repository.db != nil && validOpaquePersistenceID(sender) && validOpaquePersistenceID(recipient) && sender != recipient && validOpaquePersistenceID(pathID) && !at.IsZero() && at.Location() == time.UTC
}

func validNudgePreferenceCommand(repository *SocialFeedRepository, command socialapp.NudgePreferenceCommand) bool {
	return repository != nil && repository.db != nil && validOpaquePersistenceID(command.ActorUserID) && validOpaquePersistenceID(command.PathID) && command.Audience.Valid() && command.ExpectedRevision >= 0 && !command.ChangedAt.IsZero() && command.ChangedAt.Location() == time.UTC &&
		command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == socialapp.UpdateNudgeAudienceOperation && len(command.Idempotency.Key) >= 16 && len(command.Idempotency.RequestHash) == 32 &&
		validMutationAudit(command.Audit, audit.ResourceUpdated, "nudge_preference", command.PathID, command.ActorUserID) && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.ChangedAt)
}

func validNudgeCommand(repository *SocialFeedRepository, command socialapp.NudgeCommand) bool {
	nudge := command.Nudge
	return repository != nil && repository.db != nil && nudge.Valid() && command.Idempotency.PrincipalID == nudge.SenderID && command.Idempotency.Operation == socialapp.SendNudgeOperation && len(command.Idempotency.Key) >= 16 && len(command.Idempotency.RequestHash) == 32 &&
		validMutationAudit(command.Audit, audit.ResourceCreated, "nudge", nudge.ID, nudge.SenderID) && command.Audit.ActorUserID == nudge.SenderID && command.Audit.OccurredAt.Equal(nudge.SentAt)
}

func classifyNudgePersistenceError(err error) error {
	switch {
	case err == nil, errors.Is(err, ports.ErrInvalidArgument), errors.Is(err, ports.ErrNotFound), errors.Is(err, ports.ErrConflict), errors.Is(err, ports.ErrIdempotencyConflict), errors.Is(err, socialapp.ErrNudgeGoalComplete), errors.Is(err, socialapp.ErrNudgeAudienceDenied), errors.Is(err, socialapp.ErrNudgeAlreadySent):
		return err
	case errors.Is(err, gorm.ErrForeignKeyViolated):
		return ports.ErrNotFound
	default:
		return fmt.Errorf("social nudge persistence: %w: %v", ports.ErrUnavailable, err)
	}
}

var _ socialapp.NudgeRepository = (*SocialFeedRepository)(nil)
