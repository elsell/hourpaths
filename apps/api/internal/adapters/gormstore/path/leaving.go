package pathstore

import (
	"bytes"
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	activitystore "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/activity"
	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	shared "github.com/elsell/hour-paths/apps/api/internal/app/shared"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Repository) LeavePathReplay(ctx context.Context, actor string, pathID domain.ID, idempotency ports.Idempotency) (bool, error) {
	if r == nil || r.DB == nil || actor == "" || strings.TrimSpace(actor) != actor || pathID == "" || idempotency.PrincipalID != actor || idempotency.Operation != application.LeavePathOperation || strings.TrimSpace(idempotency.Key) == "" || len(idempotency.RequestHash) != 32 {
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
	if row.ResourceID != string(pathID) || !bytes.Equal(row.RequestHash, idempotency.RequestHash) {
		return false, ports.ErrIdempotencyConflict
	}
	return true, nil
}

type leavePathRow struct{ OwnerUserID, Role string }

func (r *Repository) LeavePath(ctx context.Context, command application.LeavePathCommand) (application.LeavePathResult, error) {
	if !validLeavePathCommand(r, command) {
		return application.LeavePathResult{}, ports.ErrInvalidArgument
	}
	var result application.LeavePathResult
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		reservation := idempotencyModel{PrincipalID: command.ActorUserID, Operation: command.Idempotency.Operation, Key: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ResourceID: string(command.PathID), CreatedAt: command.LeftAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&reservation)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			var existing idempotencyModel
			if err := tx.Where("principal_id = ? AND operation = ? AND key = ?", command.ActorUserID, command.Idempotency.Operation, command.Idempotency.Key).Take(&existing).Error; err != nil {
				return err
			}
			if existing.ResourceID != string(command.PathID) || !bytes.Equal(existing.RequestHash, command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = application.LeavePathResult{PathID: command.PathID, Left: true, ActivityRetained: command.RetainActivity, Replayed: true}
			return nil
		}
		if command.RetainActivity {
			if err := lockHiddenFeedEventOwner(tx, command.ActorUserID); err != nil {
				return err
			}
		}

		if command.RetainActivity {
			timerAudit := shared.NewAuditEvent(ctx, fixedClock{command.LeftAt}, command.ActorUserID, command.ActorUserID, audit.ActivityTimerStopped, "timer", string(command.PathID), audit.Succeeded)
			timerAudit.ID = command.NewID()
			if _, err := activitystore.New(tx).StopParticipantTimerForLeave(ctx, string(command.PathID), command.ActorUserID, command.LeftAt, command.NewID(), timerAudit); err != nil {
				return err
			}
		}

		var membership leavePathRow
		if err := tx.Table("public.leave_path_membership(?, ?, ?) AS removal(owner_user_id, removed_role, removed_activity_count)", string(command.PathID), command.ActorUserID, command.RetainActivity).Select("owner_user_id, removed_role AS role").Take(&membership).Error; err != nil {
			return err
		}
		if membership.OwnerUserID == "" || membership.OwnerUserID == command.ActorUserID || (membership.Role != "administrator" && membership.Role != "participant" && membership.Role != "supporter") {
			return ports.ErrNotFound
		}
		if err := retireInaccessiblePathTargetNotifications(tx, string(command.PathID), command.ActorUserID, command.LeftAt); err != nil {
			return err
		}
		if command.RetainActivity {
			if err := retireHiddenFeedTargetNotifications(tx, string(command.PathID), command.ActorUserID, command.LeftAt); err != nil {
				return err
			}
		}

		recipients := map[string]struct{}{membership.OwnerUserID: {}}
		var administratorIDs []string
		if err := tx.Table("path_membership_models").Where("path_id = ? AND role = ? AND user_id <> ?", command.PathID, "administrator", command.ActorUserID).Pluck("user_id", &administratorIDs).Error; err != nil {
			return err
		}
		for _, recipient := range administratorIDs {
			recipients[recipient] = struct{}{}
		}
		ordered := make([]string, 0, len(recipients))
		for recipient := range recipients {
			ordered = append(ordered, recipient)
		}
		sort.Strings(ordered)
		for _, recipient := range ordered {
			notificationID := command.NewID()
			row := map[string]any{"id": notificationID, "recipient_user_id": recipient, "actor_user_id": command.ActorUserID, "path_id": string(command.PathID), "kind": string(application.NotificationPathMemberLeft), "presentation_class": string(application.NotificationInformational), "channel": invitationNotificationChannel, "created_at": command.LeftAt}
			if err := tx.Table("notification_models").Create(row).Error; err != nil {
				return err
			}
			if err := createNotificationPushDelivery(tx, notificationID, recipient, command.LeftAt); err != nil {
				return err
			}
		}

		change := ports.AuthorizationChange{ID: command.NewID(), ResourceType: "path", ResourceID: string(command.PathID), Relation: membership.Role, SubjectType: "user", SubjectID: command.ActorUserID, OwnerUserID: membership.OwnerUserID, ActorUserID: command.ActorUserID, Operation: ports.AuthorizationDelete, LockedBy: command.AuthorizationWorker, Lease: command.AuthorizationLease}
		outbox := authorizationOutboxModel{ID: change.ID, ResourceType: change.ResourceType, ResourceID: change.ResourceID, Relation: change.Relation, SubjectType: change.SubjectType, SubjectID: change.SubjectID, OwnerUserID: change.OwnerUserID, ActorUserID: change.ActorUserID, Operation: change.Operation, LockedBy: change.LockedBy, CreatedAt: command.LeftAt}
		if err := tx.Create(&outbox).Error; err != nil {
			return err
		}
		if err := tx.Model(&outbox).Update("locked_until", gorm.Expr("CURRENT_TIMESTAMP + (? * INTERVAL '1 millisecond')", command.AuthorizationLease.Milliseconds())).Error; err != nil {
			return err
		}
		if err := tx.Create(fromAudit(command.Audit)).Error; err != nil {
			return err
		}
		result = application.LeavePathResult{PathID: command.PathID, Left: true, ActivityRetained: command.RetainActivity, AuthorizationChanges: []ports.AuthorizationChange{change}}
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func validLeavePathCommand(r *Repository, command application.LeavePathCommand) bool {
	entity := domain.Entity{ID: command.PathID, OwnerUserID: command.ActorUserID}
	return r != nil && r.DB != nil && command.ActorUserID != "" && strings.TrimSpace(command.ActorUserID) == command.ActorUserID && command.PathID != "" && !command.LeftAt.IsZero() && command.Idempotency.PrincipalID == command.ActorUserID && command.Idempotency.Operation == application.LeavePathOperation && command.Idempotency.Key != "" && len(command.Idempotency.RequestHash) == 32 && validEvent(command.Audit, audit.ResourceUpdated, entity) && command.Audit.ActorUserID == command.ActorUserID && command.Audit.OccurredAt.Equal(command.LeftAt) && command.NewID != nil && command.AuthorizationWorker != "" && command.AuthorizationLease > 0
}
