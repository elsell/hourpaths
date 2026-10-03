package gormstore

import (
	"context"
	"encoding/hex"
	application "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

// ExportDeletionRecords is an offline operations boundary, not an HTTP list API.
func (s *Store) ExportDeletionRecords(ctx context.Context) ([]application.DeletionRecord, error) {
	if s == nil || s.DB == nil {
		return nil, ports.ErrUnavailable
	}
	var rows []accountDeletionModel
	if err := s.DB.WithContext(ctx).Order("deleted_at,user_id").Limit(100001).Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 100000 {
		return nil, ports.ErrUnavailable
	}
	records := make([]application.DeletionRecord, 0, len(rows))
	for _, row := range rows {
		records = append(records, application.DeletionRecord{UserID: row.UserID, DeletedAt: row.DeletedAt, AuditEventID: row.AuditEventID, ReceiptHash: hex.EncodeToString(row.ReceiptHash)})
	}
	return records, nil
}
