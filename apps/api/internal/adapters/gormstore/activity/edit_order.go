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
