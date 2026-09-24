package pathstore

import (
	"context"
	"errors"
	"sort"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type homePathPreferenceModel struct {
	UserID, PathID string
	PinnedPosition *int64
	ManualPosition int64
}

func (homePathPreferenceModel) TableName() string { return "home_path_preference_models" }

type homePreferenceMutationModel struct {
	UserID, Operation, IdempotencyKey string
	RequestHash                       []byte
	ResultOrderMethod                 domain.HomeOrderMethod
	ResultRevision                    int64
	ResultPinnedPathIDs               pq.StringArray `gorm:"type:text[]"`
	ResultManualPathIDs               pq.StringArray `gorm:"type:text[]"`
	ResultUpdatedAt, CreatedAt        time.Time
}

func (homePreferenceMutationModel) TableName() string { return "home_preference_mutation_models" }

type homePreferenceRow struct {
	OrderMethod domain.HomeOrderMethod `gorm:"column:home_order_method"`
	Revision    int64                  `gorm:"column:home_order_revision"`
	UpdatedAt   *time.Time             `gorm:"column:home_order_updated_at"`
}

type homeOrganizationRow struct {
	PathID           string
	Role             string
	MemberCount      int64
	PinnedPosition   *int64
	ManualPosition   *int64
	RecentActivityAt *time.Time
}

func (r *Repository) ProjectHome(ctx context.Context, userID string, pathIDs []domain.ID) (application.HomeOrganizationProjection, error) {
	if r == nil || r.DB == nil || userID == "" {
		return application.HomeOrganizationProjection{}, ports.ErrInvalidArgument
	}
	preferences, err := r.readHomePreferences(r.DB.WithContext(ctx), userID)
	if err != nil {
		return application.HomeOrganizationProjection{}, err
	}
	projection := application.HomeOrganizationProjection{Preferences: preferences, Organization: make(map[domain.ID]application.HomeOrganization, len(pathIDs))}
	if len(pathIDs) == 0 {
		return projection, nil
	}
	ids := make([]string, len(pathIDs))
	seen := make(map[domain.ID]struct{}, len(pathIDs))
	for index, id := range pathIDs {
		if id == "" {
			return application.HomeOrganizationProjection{}, ports.ErrInvalidArgument
		}
		if _, duplicate := seen[id]; duplicate {
			return application.HomeOrganizationProjection{}, ports.ErrInvalidArgument
		}
		seen[id] = struct{}{}
		ids[index] = string(id)
	}
	var rows []homeOrganizationRow
	err = r.DB.WithContext(ctx).Table("path_models AS path").Select(`path.id AS path_id,
	       CASE WHEN path.owner_user_id = ? THEN 'participant' ELSE member.role END AS role,
	       1 + (SELECT count(*) FROM path_membership_models peers
	              WHERE peers.path_id = path.id AND peers.user_id <> path.owner_user_id) AS member_count,
	       preference.pinned_position, preference.manual_position,
	       (SELECT max(activity.ended_at) FROM recorded_activity_models activity
	         WHERE activity.path_id = path.id AND activity.participant_id = ?) AS recent_activity_at`, userID, userID).
		Joins("LEFT JOIN path_membership_models member ON member.path_id = path.id AND member.user_id = ?", userID).
		Joins("LEFT JOIN home_path_preference_models preference ON preference.path_id = path.id AND preference.user_id = ?", userID).
		Where("path.id IN ? AND (path.owner_user_id = ? OR member.user_id = ?)", ids, userID, userID).Scan(&rows).Error
	if err != nil {
		return application.HomeOrganizationProjection{}, err
	}
	for _, row := range rows {
		classification := application.HomeSolo
		if row.Role == "supporter" {
			classification = application.HomeSupporting
		} else if row.MemberCount > 1 {
			classification = application.HomeShared
		}
		if row.Role != "supporter" && row.Role != "participant" && row.Role != "administrator" {
			return application.HomeOrganizationProjection{}, errInvalidPersistedPath
		}
		pathID := domain.ID(row.PathID)
		projection.Organization[pathID] = application.HomeOrganization{PathID: pathID, Classification: classification, PinnedPosition: row.PinnedPosition, ManualPosition: row.ManualPosition, RecentActivityAt: utcPointer(row.RecentActivityAt)}
	}
	return projection, nil
}

func (r *Repository) readHomePreferences(db *gorm.DB, userID string) (domain.HomePreferences, error) {
	var row homePreferenceRow
	if err := db.Table("user_preference_models").Select("home_order_method, home_order_revision, home_order_updated_at").Where("user_id = ?", userID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.HomePreferences{}, ports.ErrNotFound
		}
		return domain.HomePreferences{}, err
	}
	var positions []homePathPreferenceModel
	if err := db.Where("user_id = ?", userID).Order("manual_position, path_id").Find(&positions).Error; err != nil {
		return domain.HomePreferences{}, err
	}
	preferences := domain.HomePreferences{OrderMethod: row.OrderMethod, Revision: row.Revision}
	if row.UpdatedAt != nil {
		preferences.UpdatedAt = row.UpdatedAt.UTC()
	}
	for _, position := range positions {
		preferences.ManualPathIDs = append(preferences.ManualPathIDs, domain.ID(position.PathID))
	}
	sort.SliceStable(positions, func(i, j int) bool {
		if positions[i].PinnedPosition == nil {
			return false
		}
		if positions[j].PinnedPosition == nil {
			return true
		}
		return *positions[i].PinnedPosition < *positions[j].PinnedPosition
	})
	for _, position := range positions {
		if position.PinnedPosition != nil {
			preferences.PinnedPathIDs = append(preferences.PinnedPathIDs, domain.ID(position.PathID))
		}
	}
	if !preferences.Valid() {
		return domain.HomePreferences{}, errInvalidPersistedPath
	}
	return preferences, nil
}

func (r *Repository) UpdateHomePreferences(ctx context.Context, command application.UpdateHomePreferencesCommand) (domain.HomePreferences, error) {
	if r == nil || r.DB == nil || command.ActorUserID == "" || command.ExpectedRevision < 0 || !command.OrderMethod.Valid() || command.UpdatedAt.IsZero() ||
		command.Idempotency.PrincipalID != command.ActorUserID || command.Idempotency.Operation != application.UpdateHomePreferencesOperation || command.Idempotency.Key == "" || len(command.Idempotency.RequestHash) != 32 ||
		!validHomeAudit(command.Audit, command.ActorUserID, command.UpdatedAt) {
		return domain.HomePreferences{}, ports.ErrInvalidArgument
	}
	var result domain.HomePreferences
	err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked homePreferenceRow
		if err := tx.Table("user_preference_models").Clauses(clause.Locking{Strength: "UPDATE"}).Select("home_order_method, home_order_revision, home_order_updated_at").Where("user_id = ?", command.ActorUserID).Take(&locked).Error; err != nil {
			return err
		}
		var replay homePreferenceMutationModel
		replayErr := tx.Where("user_id = ? AND operation = ? AND idempotency_key = ?", command.ActorUserID, application.UpdateHomePreferencesOperation, command.Idempotency.Key).Take(&replay).Error
		if replayErr == nil {
			if string(replay.RequestHash) != string(command.Idempotency.RequestHash) {
				return ports.ErrIdempotencyConflict
			}
			result = preferencesFromReplay(replay)
			return nil
		}
		if !errors.Is(replayErr, gorm.ErrRecordNotFound) {
			return replayErr
		}
		if locked.Revision != command.ExpectedRevision {
			return ports.ErrConflict
		}
		var current []string
		if err := tx.Table("path_models AS path").Select("DISTINCT path.id").
			Joins("LEFT JOIN path_membership_models member ON member.path_id = path.id AND member.user_id = ?", command.ActorUserID).
			Where("path.owner_user_id = ? OR member.user_id = ?", command.ActorUserID, command.ActorUserID).
			Order("path.id").Pluck("path.id", &current).Error; err != nil {
			return err
		}
		if !samePathSet(current, command.ManualPathIDs) || !pinsAreSubset(command.PinnedPathIDs, command.ManualPathIDs) {
			return ports.ErrConflict
		}
		if err := tx.Where("user_id = ?", command.ActorUserID).Delete(&homePathPreferenceModel{}).Error; err != nil {
			return err
		}
		pinPositions := make(map[domain.ID]int64, len(command.PinnedPathIDs))
		for index, id := range command.PinnedPathIDs {
			pinPositions[id] = int64(index)
		}
		for index, id := range command.ManualPathIDs {
			row := homePathPreferenceModel{UserID: command.ActorUserID, PathID: string(id), ManualPosition: int64(index)}
			if position, pinned := pinPositions[id]; pinned {
				row.PinnedPosition = &position
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		nextRevision := locked.Revision + 1
		updated := tx.Table("user_preference_models").Where("user_id = ? AND home_order_revision = ?", command.ActorUserID, command.ExpectedRevision).Updates(map[string]any{"home_order_method": command.OrderMethod, "home_order_revision": nextRevision, "home_order_updated_at": command.UpdatedAt})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ports.ErrConflict
		}
		result = domain.HomePreferences{OrderMethod: command.OrderMethod, Revision: nextRevision, PinnedPathIDs: append([]domain.ID(nil), command.PinnedPathIDs...), ManualPathIDs: append([]domain.ID(nil), command.ManualPathIDs...), UpdatedAt: command.UpdatedAt}
		receipt := homePreferenceMutationModel{UserID: command.ActorUserID, Operation: application.UpdateHomePreferencesOperation, IdempotencyKey: command.Idempotency.Key, RequestHash: append([]byte(nil), command.Idempotency.RequestHash...), ResultOrderMethod: result.OrderMethod, ResultRevision: result.Revision, ResultPinnedPathIDs: pathStrings(result.PinnedPathIDs), ResultManualPathIDs: pathStrings(result.ManualPathIDs), ResultUpdatedAt: result.UpdatedAt, CreatedAt: command.UpdatedAt}
		if err := tx.Create(&receipt).Error; err != nil {
			return err
		}
		return tx.Create(fromAudit(command.Audit)).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ports.ErrNotFound
	}
	return result, err
}

func validHomeAudit(event audit.Event, actor string, at time.Time) bool {
	return event.Valid() && event.Action == audit.ResourceUpdated && event.Outcome == audit.Succeeded && event.OwnerUserID == actor && event.ActorUserID == actor && event.TargetType == "home_preferences" && event.TargetID == actor && event.OccurredAt.Equal(at)
}
func utcPointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
func pathStrings(ids []domain.ID) pq.StringArray {
	values := make(pq.StringArray, len(ids))
	for i, id := range ids {
		values[i] = string(id)
	}
	return values
}
func preferencesFromReplay(row homePreferenceMutationModel) domain.HomePreferences {
	result := domain.HomePreferences{OrderMethod: row.ResultOrderMethod, Revision: row.ResultRevision, UpdatedAt: row.ResultUpdatedAt.UTC()}
	for _, id := range row.ResultPinnedPathIDs {
		result.PinnedPathIDs = append(result.PinnedPathIDs, domain.ID(id))
	}
	for _, id := range row.ResultManualPathIDs {
		result.ManualPathIDs = append(result.ManualPathIDs, domain.ID(id))
	}
	return result
}
func samePathSet(current []string, requested []domain.ID) bool {
	if len(current) != len(requested) {
		return false
	}
	values := make([]string, len(requested))
	for i, id := range requested {
		if id == "" {
			return false
		}
		values[i] = string(id)
	}
	sort.Strings(values)
	for i := range current {
		if current[i] != values[i] || (i > 0 && values[i] == values[i-1]) {
			return false
		}
	}
	return true
}
func pinsAreSubset(pins, manual []domain.ID) bool {
	members := make(map[domain.ID]struct{}, len(manual))
	for _, id := range manual {
		members[id] = struct{}{}
	}
	seen := map[domain.ID]struct{}{}
	for _, id := range pins {
		if _, ok := members[id]; !ok {
			return false
		}
		if _, duplicate := seen[id]; duplicate {
			return false
		}
		seen[id] = struct{}{}
	}
	return true
}
