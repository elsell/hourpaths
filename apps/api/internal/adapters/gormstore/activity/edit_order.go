package activitystore

import (
	"errors"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/activity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type editOrderModel struct {
	ActivityID  string `gorm:"primaryKey"`
	AuthoredAt  time.Time
	Counter     int64
	OperationID string
}

func (editOrderModel) TableName() string { return "activity_edit_order_models" }

// Callers hold the participant/Path progress lock and the activity row lock.
func loadEditOrder(tx *gorm.DB, entry domain.RecordedActivity) (domain.ActivityEditOrder, error) {
	var row editOrderModel
	err := tx.Where("activity_id = ?", entry.ID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.NewActivityEditOrder(entry.UpdatedAt, 0, entry.ID)
	}
	if err != nil {
		return domain.ActivityEditOrder{}, err
	}
	return domain.NewActivityEditOrder(row.AuthoredAt, row.Counter, row.OperationID)
}
func saveEditOrder(tx *gorm.DB, id string, order domain.ActivityEditOrder) error {
	row := editOrderModel{ActivityID: id, AuthoredAt: order.AuthoredAt, Counter: order.Counter, OperationID: order.OperationID}
	return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "activity_id"}}, DoUpdates: clause.AssignmentColumns([]string{"authored_at", "counter", "operation_id"})}).Create(&row).Error
}

// Only owner-visible activity identities may acquire private causal metadata.
func loadOwnerEditOrders(tx *gorm.DB, owner string, ids []string) (map[string]domain.ActivityEditOrder, error) {
	result := map[string]domain.ActivityEditOrder{}
	if len(ids) == 0 {
		return result, nil
	}
	var rows []editOrderModel
	err := tx.Table("activity_edit_order_models AS edit_order").Select("edit_order.*").Joins("JOIN recorded_activity_models AS activity ON activity.id = edit_order.activity_id").Where("activity.participant_id = ? AND activity.id IN ?", owner, ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		order, err := domain.NewActivityEditOrder(row.AuthoredAt, row.Counter, row.OperationID)
		if err != nil {
			return nil, err
		}
		result[row.ActivityID] = order
	}
	return result, nil
}
