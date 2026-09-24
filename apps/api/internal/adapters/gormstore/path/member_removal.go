package pathstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	activityapp "github.com/elsell/hour-paths/apps/api/internal/app/activity"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) ListMembers(ctx context.Context, query application.MemberListQuery, page application.MemberPageRequest) (application.MemberPage, error) {
	if r == nil || r.DB == nil || query.PathID == "" || query.ActorUserID == "" || page.Limit < 1 || page.Limit > 100 || page.Snapshot.IsZero() {
		return application.MemberPage{}, ports.ErrInvalidArgument
	}
	var result application.MemberPage
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		snapshotRepository := *r
		snapshotRepository.DB = tx
		var err error
		result, err = snapshotRepository.listMembersInSnapshot(ctx, query, page)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func (r *Repository) listMembersInSnapshot(ctx context.Context, query application.MemberListQuery, page application.MemberPageRequest) (application.MemberPage, error) {
	var pathRow model
	if err := r.DB.WithContext(ctx).Where("id = ?", query.PathID).Take(&pathRow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return application.MemberPage{}, ports.ErrNotFound
		}
		return application.MemberPage{}, err
	}
	path, err := toEntity(pathRow)
	if err != nil {
		return application.MemberPage{}, err
	}
	projectionFingerprint, err := r.memberProjectionFingerprint(ctx, path, page.Snapshot)
	if err != nil {
		return application.MemberPage{}, err
	}
	var rows []struct {
		UserID, Username, DisplayName, Role                 string
		SessionCount, TotalTrackedSeconds                   int64
		CurrentTimeZone                                     *string
		BlockedByViewer, CanRemove, CanChangeRole, CanLeave bool
		CanGrantAdministrator, CanRevokeAdministrator       bool
		CanStepDownAdministrator                            bool
		CreatedAt                                           time.Time
	}
	db := r.DB.WithContext(ctx).Table("path_models AS path").Select(`membership.user_id, target.username, target.display_name,
        CASE WHEN membership.user_id = path.owner_user_id THEN 'creator' ELSE membership.role END AS role,
        membership.joined_at AS created_at,
		preferences.current_time_zone,
        count(activity.id) AS session_count,
        COALESCE(sum(EXTRACT(EPOCH FROM (activity.ended_at - activity.started_at))), 0)::bigint AS total_tracked_seconds,
        EXISTS (SELECT 1 FROM block_models viewer_block WHERE viewer_block.blocker_user_id = ? AND viewer_block.blocked_user_id = membership.user_id) AS blocked_by_viewer,
        (path.archived_at IS NULL AND membership.user_id <> ? AND membership.user_id <> path.owner_user_id AND membership.role IN ('participant', 'supporter') AND (path.owner_user_id = ? OR manager.user_id IS NOT NULL)) AS can_remove,
		(path.archived_at IS NULL AND membership.user_id <> ? AND membership.user_id <> path.owner_user_id AND membership.role IN ('participant', 'supporter') AND (path.owner_user_id = ? OR manager.user_id IS NOT NULL)) AS can_change_role,
		(path.archived_at IS NULL AND membership.user_id <> path.owner_user_id AND membership.role = 'participant' AND path.owner_user_id = ?) AS can_grant_administrator,
		(path.archived_at IS NULL AND membership.user_id <> ? AND membership.user_id <> path.owner_user_id AND membership.role = 'administrator' AND path.owner_user_id = ?) AS can_revoke_administrator,
		(path.archived_at IS NULL AND membership.user_id = ? AND membership.role = 'administrator') AS can_step_down_administrator,
		(path.archived_at IS NULL AND membership.user_id = ? AND membership.user_id <> path.owner_user_id) AS can_leave`, query.ActorUserID, query.ActorUserID, query.ActorUserID, query.ActorUserID, query.ActorUserID, query.ActorUserID, query.ActorUserID, query.ActorUserID, query.ActorUserID, query.ActorUserID).
		Joins("JOIN path_membership_models AS membership ON membership.path_id = path.id").Joins("JOIN user_models AS target ON target.id = membership.user_id AND target.username IS NOT NULL").
		Joins("LEFT JOIN user_preference_models AS preferences ON preferences.user_id = membership.user_id").
		Joins("JOIN path_membership_models AS viewer ON viewer.path_id = path.id AND viewer.user_id = ?", query.ActorUserID).
		Joins("LEFT JOIN path_membership_models AS manager ON manager.path_id = path.id AND manager.user_id = ? AND manager.role = 'administrator'", query.ActorUserID).
		Joins("LEFT JOIN recorded_activity_models AS activity ON activity.path_id = path.id AND activity.participant_id = membership.user_id AND activity.created_at <= ? AND (membership.user_id = path.owner_user_id OR membership.role IN ('administrator', 'participant'))", page.Snapshot).
		Where("path.id = ? AND membership.joined_at <= ?", query.PathID, page.Snapshot).
		Group("path.id, membership.user_id, target.id, target.username, target.display_name, membership.role, membership.joined_at, manager.user_id, preferences.current_time_zone")
	if !page.AfterCreated.IsZero() {
		db = db.Where("(membership.joined_at, target.id) > (?, ?)", page.AfterCreated, page.AfterUserID)
	}
	if err := db.Order("membership.joined_at ASC, target.id ASC").Limit(page.Limit + 1).Find(&rows).Error; err != nil {
		return application.MemberPage{}, err
	}
	if len(rows) == 0 {
		var count int64
		if err := r.DB.WithContext(ctx).Table("path_models AS path").Joins("JOIN path_membership_models AS viewer ON viewer.path_id = path.id AND viewer.user_id = ?", query.ActorUserID).Where("path.id = ?", query.PathID).Count(&count).Error; err != nil {
			return application.MemberPage{}, err
		}
		if count == 0 {
			return application.MemberPage{}, ports.ErrNotFound
		}
	}
	hasMore := len(rows) > page.Limit
	if hasMore {
		rows = rows[:page.Limit]
	}
	items := make([]application.Member, 0, len(rows))
	for _, row := range rows {
		interval, overall, err := r.memberGoalProgress(ctx, path, row.UserID, row.Role, row.CurrentTimeZone, row.TotalTrackedSeconds, page.Snapshot)
		if err != nil {
			return application.MemberPage{}, err
		}
		items = append(items, application.Member{UserID: row.UserID, Username: row.Username, DisplayName: row.DisplayName, Role: row.Role, SessionCount: row.SessionCount, TotalTrackedSeconds: row.TotalTrackedSeconds, IntervalProgress: interval, OverallProgress: overall, BlockedByViewer: row.BlockedByViewer, CanRemove: row.CanRemove, CanChangeRole: row.CanChangeRole, CanGrantAdministrator: row.CanGrantAdministrator, CanRevokeAdministrator: row.CanRevokeAdministrator, CanStepDownAdministrator: row.CanStepDownAdministrator, CanLeave: row.CanLeave, CreatedAt: row.CreatedAt})
	}
	return application.MemberPage{Items: items, ProjectionFingerprint: projectionFingerprint, HasMore: hasMore}, nil
}

func (r *Repository) memberProjectionFingerprint(ctx context.Context, path domain.Entity, snapshot time.Time) (string, error) {
	type memberProjectionIdentity struct {
		UserID, Role string
		TimeZone     *string
	}
	var members []memberProjectionIdentity
	if err := r.DB.WithContext(ctx).Table("path_membership_models AS membership").
		Select("membership.user_id, membership.role, preferences.current_time_zone AS time_zone").
		Joins("LEFT JOIN user_preference_models AS preferences ON preferences.user_id = membership.user_id").
		Where("membership.path_id = ? AND membership.joined_at <= ?", path.ID, snapshot).
		Order("membership.user_id ASC").Scan(&members).Error; err != nil {
		return "", err
	}
	payload := struct {
		OwnerUserID   string
		IntervalGoal  domain.IntervalGoal
		OverallTarget domain.OverallTarget
		Members       []memberProjectionIdentity
	}{OwnerUserID: path.OwnerUserID, IntervalGoal: path.IntervalGoal, OverallTarget: path.OverallTarget, Members: members}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func (r *Repository) memberGoalProgress(ctx context.Context, path domain.Entity, userID, role string, timeZone *string, total int64, snapshot time.Time) (*application.GoalProgress, *application.GoalProgress, error) {
	if role == "supporter" {
		return nil, nil, nil
	}
	var overall *application.GoalProgress
	if path.OverallTarget.Present {
		overall = &application.GoalProgress{AccumulatedSeconds: total, TargetSeconds: path.OverallTarget.TargetSeconds}
	}
	if !path.IntervalGoal.Present {
		return nil, overall, nil
	}
	if timeZone == nil {
		return nil, nil, errors.New("persisted participant time zone is missing")
	}
	window, err := activityapp.CurrentIntervalWindow(path.IntervalGoal, *timeZone, snapshot)
	if err != nil {
		return nil, nil, errors.New("persisted participant time zone or interval goal is invalid")
	}
	var accumulated int64
	err = r.DB.WithContext(ctx).Table("recorded_activity_models").Select(`COALESCE(SUM(CASE WHEN started_at < ? AND ended_at > ? THEN
		FLOOR(EXTRACT(EPOCH FROM (LEAST(ended_at, ?) - started_at))) -
		FLOOR(EXTRACT(EPOCH FROM (GREATEST(started_at, ?) - started_at)))
		ELSE 0 END), 0)::bigint`, window.EndedAt, window.StartedAt, window.EndedAt, window.StartedAt).
		Where("participant_id = ? AND path_id = ? AND created_at <= ?", userID, path.ID, snapshot).Scan(&accumulated).Error
	if err != nil {
		return nil, nil, err
	}
	if accumulated < 0 {
		return nil, nil, errors.New("persisted interval activity duration is invalid")
	}
	return &application.GoalProgress{AccumulatedSeconds: accumulated, TargetSeconds: path.IntervalGoal.TargetSeconds}, overall, nil
}

func (r *Repository) ReviewMemberRemoval(ctx context.Context, query application.MemberRemovalQuery) (application.MemberRemovalReview, error) {
	if r == nil || r.DB == nil || query.PathID == "" || query.ActorUserID == "" || query.TargetUserID == "" || strings.TrimSpace(query.ActorUserID) != query.ActorUserID || strings.TrimSpace(query.TargetUserID) != query.TargetUserID || query.ActorUserID == query.TargetUserID {
		return application.MemberRemovalReview{}, ports.ErrInvalidArgument
	}
	var row struct {
		UserID, Username, DisplayName, Role string
		SessionCount, TotalTrackedSeconds   int64
		RunningTimer                        bool
	}
	err := r.DB.WithContext(ctx).Table("path_models AS path").
		Select(`target_membership.user_id, target.username, target.display_name, target_membership.role,
          count(activity.id) AS session_count,
          COALESCE(sum(EXTRACT(EPOCH FROM (activity.ended_at - activity.started_at))), 0)::bigint AS total_tracked_seconds,
          EXISTS (SELECT 1 FROM running_timer_models timer WHERE timer.path_id = path.id AND timer.participant_id = target_membership.user_id) AS running_timer`).
		Joins("JOIN path_membership_models AS target_membership ON target_membership.path_id = path.id AND target_membership.user_id = ? AND target_membership.role IN ?", query.TargetUserID, []string{"participant", "supporter"}).
		Joins("JOIN user_models AS target ON target.id = target_membership.user_id AND target.username IS NOT NULL").
		Joins("LEFT JOIN path_membership_models AS manager ON manager.path_id = path.id AND manager.user_id = ? AND manager.role = 'administrator'", query.ActorUserID).
		Joins("LEFT JOIN recorded_activity_models AS activity ON activity.path_id = path.id AND activity.participant_id = target_membership.user_id").
		Where("path.id = ? AND path.archived_at IS NULL AND (path.owner_user_id = ? OR manager.user_id IS NOT NULL)", query.PathID, query.ActorUserID).
		Group("path.id, target_membership.user_id, target.username, target.display_name, target_membership.role").Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return application.MemberRemovalReview{}, ports.ErrNotFound
	}
	if err != nil {
		return application.MemberRemovalReview{}, err
	}
	return application.MemberRemovalReview{UserID: row.UserID, Username: row.Username, DisplayName: row.DisplayName, Role: domain.MembershipRole(row.Role), SessionCount: row.SessionCount, TotalTrackedSeconds: row.TotalTrackedSeconds, RunningTimer: row.RunningTimer}, nil
}

func removalResourceID(pathID domain.ID, target string) string { return string(pathID) + ":" + target }

func (r *Repository) RemoveMemberReplay(ctx context.Context, actor string, pathID domain.ID, target string, idempotency ports.Idempotency) (bool, error) {
	if r == nil || r.DB == nil || actor == "" || pathID == "" || target == "" || idempotency.PrincipalID != actor || idempotency.Operation != application.RemoveMemberOperation || strings.TrimSpace(idempotency.Key) == "" || len(idempotency.RequestHash) != 32 {
		return false, ports.ErrInvalidArgument
	}
	var row idempotencyModel
	err := r.DB.WithContext(ctx).Where("principal_id = ? AND operation = ? AND key = ?", actor, idempotency.Operation, idempotency.Key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.ResourceID != removalResourceID(pathID, target) || !bytes.Equal(row.RequestHash, idempotency.RequestHash) {
		return false, ports.ErrIdempotencyConflict
	}
	return true, nil
}

func (r *Repository) RemoveMember(ctx context.Context, command application.RemoveMemberCommand) (application.RemoveMemberResult, error) {
	if !validRemoveMemberCommand(r, command) {
		return application.RemoveMemberResult{}, ports.ErrInvalidArgument
	}
	var result application.RemoveMemberResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resourceID := removalResourceID(command.PathID, command.TargetUserID)
		reservation := idempotencyModel{PrincipalID: command.ActorUserID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ResourceID: resourceID, CreatedAt: command.RemovedAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existing idempotencyModel
			if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&existing).Error; err != nil {
				return err
			}
			if existing.ResourceID != resourceID || !bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = application.RemoveMemberResult{PathID: command.PathID, UserID: command.TargetUserID, Removed: true, ActivityDeleted: command.ExpectedRole == domain.RoleParticipant, Replayed: true}
			return nil
		}

		var removal struct {
			OwnerUserID, RemovedRole string
			RemovedActivityCount     int64
		}
		err := tx.Table("public.remove_path_member_data(?, ?, ?, ?) AS removal(owner_user_id, removed_role, removed_activity_count)", string(command.PathID), command.ActorUserID, command.TargetUserID, string(command.ExpectedRole)).Take(&removal).Error
		if err != nil {
			return err
		}
		if removal.OwnerUserID == "" || removal.RemovedRole != string(command.ExpectedRole) {
			return ports.ErrNotFound
		}
		change := ports.AuthorizationChange{ID: command.NewID(), ResourceType: "path", ResourceID: string(command.PathID), Relation: removal.RemovedRole, SubjectType: "user", SubjectID: command.TargetUserID, OwnerUserID: removal.OwnerUserID, ActorUserID: command.ActorUserID, Operation: ports.AuthorizationDelete, LockedBy: command.AuthorizationWorker, Lease: command.AuthorizationLease}
		outbox := authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: command.RemovedAt}
		if err := tx.Create(&outbox).Error; err != nil {
			return err
		}
		if err := tx.Model(&outbox).Update("locked_until", gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", command.AuthorizationLease.Milliseconds())).Error; err != nil {
			return err
		}
		if err := retireInaccessiblePathTargetNotifications(tx, string(command.PathID), command.TargetUserID, command.RemovedAt); err != nil {
			return err
		}
		if err := createMemberAccessNotification(tx, command.Notification, false); err != nil {
			return err
		}
		persistedAudit := command.Audit
		persistedAudit.OwnerUserID = removal.OwnerUserID
		if err := tx.Create(fromAudit(persistedAudit)).Error; err != nil {
			return err
		}
		result = application.RemoveMemberResult{PathID: command.PathID, UserID: command.TargetUserID, Removed: true, ActivityDeleted: command.ExpectedRole == domain.RoleParticipant, AuthorizationChanges: []ports.AuthorizationChange{change}}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validRemoveMemberCommand(r *Repository, c application.RemoveMemberCommand) bool {
	return r != nil && r.DB != nil && c.PathID != "" && c.ActorUserID != "" && c.TargetUserID != "" && c.ActorUserID != c.TargetUserID && strings.TrimSpace(c.ActorUserID) == c.ActorUserID && strings.TrimSpace(c.TargetUserID) == c.TargetUserID && c.ExpectedRole.Valid() && !c.RemovedAt.IsZero() && c.Idempotency.PrincipalID == c.ActorUserID && c.Idempotency.Operation == application.RemoveMemberOperation && strings.TrimSpace(c.Idempotency.Key) != "" && len(c.Idempotency.RequestHash) == 32 && c.Audit.Valid() && c.Audit.Action == audit.ResourceUpdated && c.Audit.Outcome == audit.Succeeded && c.Audit.TargetType == "path_member" && c.Audit.TargetID == removalResourceID(c.PathID, c.TargetUserID) && c.Audit.OwnerUserID == c.ActorUserID && c.Audit.ActorUserID == c.ActorUserID && c.Audit.OccurredAt.Equal(c.RemovedAt) && c.Notification.ID != "" && c.Notification.RecipientUserID == c.TargetUserID && c.Notification.ActorUserID == c.ActorUserID && c.Notification.PathID == c.PathID && c.Notification.Kind == application.NotificationPathMemberRemoved && c.Notification.Role == c.ExpectedRole && c.Notification.CreatedAt.Equal(c.RemovedAt) && c.NewID != nil && c.AuthorizationWorker != "" && c.AuthorizationLease > 0
}

func (r *Repository) ChangeMemberRoleReplay(ctx context.Context, actor string, pathID domain.ID, target string, idempotency ports.Idempotency) (bool, error) {
	if r == nil || r.DB == nil || actor == "" || pathID == "" || target == "" || idempotency.PrincipalID != actor || idempotency.Operation != application.ChangeMemberRoleOperation || strings.TrimSpace(idempotency.Key) == "" || len(idempotency.RequestHash) != 32 {
		return false, ports.ErrInvalidArgument
	}
	var row idempotencyModel
	err := r.DB.WithContext(ctx).Where("principal_id = ? AND operation = ? AND key = ?", actor, idempotency.Operation, idempotency.Key).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if row.ResourceID != removalResourceID(pathID, target) || !bytes.Equal(row.RequestHash, idempotency.RequestHash) {
		return false, ports.ErrIdempotencyConflict
	}
	return true, nil
}

func (r *Repository) ChangeMemberRole(ctx context.Context, command application.ChangeMemberRoleCommand) (application.ChangeMemberRoleResult, error) {
	if !validChangeMemberRoleCommand(r, command) {
		return application.ChangeMemberRoleResult{}, ports.ErrInvalidArgument
	}
	var result application.ChangeMemberRoleResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		resourceID := removalResourceID(command.PathID, command.TargetUserID)
		reservation := idempotencyModel{PrincipalID: command.ActorUserID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ResourceID: resourceID, CreatedAt: command.ChangedAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existing idempotencyModel
			if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&existing).Error; err != nil {
				return err
			}
			if existing.ResourceID != resourceID || !bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = application.ChangeMemberRoleResult{PathID: command.PathID, UserID: command.TargetUserID, Role: command.Role, ActivityDeleted: command.ExpectedRole == domain.RoleParticipant && command.Role == domain.RoleSupporter, Replayed: true}
			return nil
		}

		var changed struct {
			OwnerUserID, PreviousRole, ResultingRole string
			RemovedActivityCount                     int64
		}
		if err := tx.Table("public.change_path_member_role(?, ?, ?, ?, ?) AS changed(owner_user_id, previous_role, resulting_role, removed_activity_count)", string(command.PathID), command.ActorUserID, command.TargetUserID, string(command.ExpectedRole), string(command.Role)).Take(&changed).Error; err != nil {
			return err
		}
		if changed.OwnerUserID == "" || changed.PreviousRole != string(command.ExpectedRole) || changed.ResultingRole != string(command.Role) {
			return ports.ErrNotFound
		}

		changes := []ports.AuthorizationChange{
			{ID: command.NewID(), ResourceType: "path", ResourceID: string(command.PathID), Relation: changed.PreviousRole, SubjectType: "user", SubjectID: command.TargetUserID, OwnerUserID: changed.OwnerUserID, ActorUserID: command.ActorUserID, Operation: ports.AuthorizationDelete, LockedBy: command.AuthorizationWorker, Lease: command.AuthorizationLease},
			{ID: command.NewID(), ResourceType: "path", ResourceID: string(command.PathID), Relation: changed.ResultingRole, SubjectType: "user", SubjectID: command.TargetUserID, OwnerUserID: changed.OwnerUserID, ActorUserID: command.ActorUserID, Operation: ports.AuthorizationTouch, LockedBy: command.AuthorizationWorker, Lease: command.AuthorizationLease},
		}
		for _, change := range changes {
			outbox := authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: command.ChangedAt}
			if err := tx.Create(&outbox).Error; err != nil {
				return err
			}
			if err := tx.Model(&outbox).Update("locked_until", gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", command.AuthorizationLease.Milliseconds())).Error; err != nil {
				return err
			}
		}
		selfStepDown := command.ActorUserID == command.TargetUserID && command.ExpectedRole == domain.RoleAdministrator && command.Role == domain.RoleParticipant
		if err := createMemberAccessNotification(tx, command.Notification, selfStepDown); err != nil {
			return err
		}
		persistedAudit := command.Audit
		persistedAudit.OwnerUserID = changed.OwnerUserID
		if err := tx.Create(fromAudit(persistedAudit)).Error; err != nil {
			return err
		}
		result = application.ChangeMemberRoleResult{PathID: command.PathID, UserID: command.TargetUserID, Role: command.Role, ActivityDeleted: command.ExpectedRole == domain.RoleParticipant && command.Role == domain.RoleSupporter, AuthorizationChanges: changes}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validChangeMemberRoleCommand(r *Repository, c application.ChangeMemberRoleCommand) bool {
	return r != nil && r.DB != nil && c.PathID != "" && c.ActorUserID != "" && c.TargetUserID != "" && strings.TrimSpace(c.ActorUserID) == c.ActorUserID && strings.TrimSpace(c.TargetUserID) == c.TargetUserID && validPersistedMemberRoleTransition(c.ActorUserID, c.TargetUserID, c.ExpectedRole, c.Role) && !c.ChangedAt.IsZero() && c.Idempotency.PrincipalID == c.ActorUserID && c.Idempotency.Operation == application.ChangeMemberRoleOperation && strings.TrimSpace(c.Idempotency.Key) != "" && len(c.Idempotency.RequestHash) == 32 && c.Audit.Valid() && c.Audit.Action == audit.ResourceUpdated && c.Audit.Outcome == audit.Succeeded && c.Audit.TargetType == "path_member" && c.Audit.TargetID == removalResourceID(c.PathID, c.TargetUserID) && c.Audit.OwnerUserID == c.ActorUserID && c.Audit.ActorUserID == c.ActorUserID && c.Audit.OccurredAt.Equal(c.ChangedAt) && c.Notification.ID != "" && c.Notification.RecipientUserID == c.TargetUserID && c.Notification.ActorUserID == c.ActorUserID && c.Notification.PathID == c.PathID && c.Notification.Kind == application.NotificationPathMemberRoleChanged && c.Notification.Role == c.Role && c.Notification.CreatedAt.Equal(c.ChangedAt) && c.NewID != nil && c.AuthorizationWorker != "" && c.AuthorizationLease > 0
}

func validPersistedMemberRoleTransition(actor, target string, from, to domain.MembershipRole) bool {
	if !from.ValidPathMemberRole() || !to.ValidPathMemberRole() || from == to {
		return false
	}
	if actor == target {
		return from == domain.RoleAdministrator && to == domain.RoleParticipant
	}
	return from.Valid() && to.Valid() || from == domain.RoleParticipant && to == domain.RoleAdministrator || from == domain.RoleAdministrator && to == domain.RoleParticipant
}
