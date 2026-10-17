package gormstore

import (
	"context"
	"encoding/json"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/adminattention"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"time"
)

type AdminAttentionRepository struct{ DB *gorm.DB }

func (r AdminAttentionRepository) Notifications(ctx context.Context, actor string, at time.Time) ([]domain.Notice, error) {
	items := []domain.Notice{}
	e := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, e := supportAdmin(tx, actor)
		if e != nil {
			return e
		}
		if !ok {
			return ports.ErrNotFound
		}
		var rows []struct {
			ID, CaseID, EnforcementID string
			CreatedAt                 time.Time
			ReadAt                    *time.Time
		}
		if e = tx.Table("admin_attention_models").Where("recipient_id=?", actor).Order("created_at DESC,id DESC").Limit(50).Find(&rows).Error; e != nil {
			return e
		}
		for _, row := range rows {
			kind, target := "report", row.CaseID
			if row.EnforcementID != "" {
				kind, target = "appeal", row.EnforcementID
			}
			items = append(items, domain.Notice{ID: row.ID, Kind: kind, TargetID: target, CreatedAt: row.CreatedAt, Read: row.ReadAt != nil})
		}
		return supportAudit(tx, "moderation-notices", actor, "notices", at)
	})
	return items, e
}
func (r AdminAttentionRepository) Read(ctx context.Context, actor, id string, at time.Time) (domain.Record, error) {
	var raw *string
	if e := r.DB.WithContext(ctx).Raw("SELECT public.read_admin_attention(?,?::uuid,?)::text", actor, id, at).Scan(&raw).Error; e != nil {
		return domain.Record{}, e
	}
	if raw == nil {
		return domain.Record{}, ports.ErrNotFound
	}
	var row struct {
		ID, Kind, Reason, Explanation, State string
		CreatedAt                            time.Time `json:"createdAt"`
	}
	if e := json.Unmarshal([]byte(*raw), &row); e != nil {
		return domain.Record{}, e
	}
	return domain.Record{ID: row.ID, Kind: row.Kind, Reason: row.Reason, Explanation: row.Explanation, State: row.State, CreatedAt: row.CreatedAt}, nil
}
