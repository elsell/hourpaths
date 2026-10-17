package gormstore

import (
	"context"
	"errors"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/support"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type SupportRepository struct{ DB *gorm.DB }
type supportRequest struct {
	ID, KeyHash, Subject string
	CreatedAt, ExpiresAt time.Time
}

func (supportRequest) TableName() string { return "support_request_models" }

type supportMessage struct {
	ID, RequestID, Body string
	Operator            bool
	CreatedAt           time.Time
}

func (supportMessage) TableName() string { return "support_message_models" }

type supportNotice struct {
	ID, RecipientID, RequestID string
	CreatedAt                  time.Time
	ReadAt                     *time.Time
}

func (supportNotice) TableName() string { return "support_notice_models" }
func supportAudit(tx *gorm.DB, id, actor, operation string, at time.Time) error {
	if actor == "" {
		actor = "requester"
	}
	return tx.Table("support_audit_models").Create(map[string]any{"request_id": id, "actor": actor, "operation": operation, "occurred_at": at}).Error
}
func supportAdmin(tx *gorm.DB, actor string) (bool, error) {
	var count int64
	e := tx.Table("platform_admin_models a").Joins("JOIN user_models u ON u.id=a.user_id AND u.status='active'").Where("a.user_id=?", actor).Count(&count).Error
	return count == 1, e
}
func (r SupportRepository) IsAdmin(ctx context.Context, actor string) (bool, error) {
	return supportAdmin(r.DB.WithContext(ctx), actor)
}
func supportLoad(tx *gorm.DB, id, key, actor string, at time.Time) (domain.Conversation, error) {
	q := tx.Where("id=? AND expires_at>?", id, at)
	if actor != "" {
		ok, e := supportAdmin(tx, actor)
		if e != nil {
			return domain.Conversation{}, e
		}
		if !ok {
			return domain.Conversation{}, ports.ErrNotFound
		}
	} else {
		q = q.Where("key_hash=?", key)
	}
	var row supportRequest
	if e := q.Take(&row).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			e = ports.ErrNotFound
		}
		return domain.Conversation{}, e
	}
	var messages []supportMessage
	if e := tx.Where("request_id=?", id).Order("created_at,id").Limit(101).Find(&messages).Error; e != nil {
		return domain.Conversation{}, e
	}
	if len(messages) > 100 {
		return domain.Conversation{}, ports.ErrUnavailable
	}
	c := domain.Conversation{ID: row.ID, KeyHash: row.KeyHash, Subject: row.Subject, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt, Messages: []domain.Message{}}
	for _, m := range messages {
		c.Messages = append(c.Messages, domain.Message{ID: m.ID, Body: m.Body, Operator: m.Operator, CreatedAt: m.CreatedAt})
	}
	return c, nil
}
func supportNotify(tx *gorm.DB, id, request string, at time.Time) error {
	var admins []string
	if e := tx.Table("platform_admin_models a").Joins("JOIN user_models u ON u.id=a.user_id AND u.status='active'").Pluck("a.user_id", &admins).Error; e != nil {
		return e
	}
	for _, admin := range admins {
		if e := tx.Create(&supportNotice{ID: id, RecipientID: admin, RequestID: request, CreatedAt: at}).Error; e != nil {
			return e
		}
	}
	return nil
}
func (r SupportRepository) Create(ctx context.Context, c domain.Conversation) (domain.Conversation, error) {
	var result domain.Conversation
	e := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := supportRequest{ID: c.ID, KeyHash: c.KeyHash, Subject: c.Subject, CreatedAt: c.CreatedAt, ExpiresAt: c.ExpiresAt}
		created := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
		if created.Error != nil {
			return created.Error
		}
		if created.RowsAffected == 0 {
			old, e := supportLoad(tx, c.ID, c.KeyHash, "", c.CreatedAt)
			if e != nil {
				return ports.ErrIdempotencyConflict
			}
			initialMatches := false
			for _, message := range old.Messages {
				if message.ID == c.ID && !message.Operator && message.Body == c.Messages[0].Body {
					initialMatches = true
				}
			}
			if old.Subject != c.Subject || !initialMatches {
				return ports.ErrIdempotencyConflict
			}
			result = old
			return supportAudit(tx, c.ID, "", "read", c.CreatedAt)
		}
		m := c.Messages[0]
		if e := tx.Create(&supportMessage{ID: m.ID, RequestID: c.ID, Body: m.Body, CreatedAt: m.CreatedAt}).Error; e != nil {
			return e
		}
		if e := supportNotify(tx, m.ID, c.ID, m.CreatedAt); e != nil {
			return e
		}
		result = c
		return supportAudit(tx, c.ID, "", "create", c.CreatedAt)
	})
	return result, e
}
func (r SupportRepository) Get(ctx context.Context, id, key, actor string, at time.Time) (domain.Conversation, error) {
	var c domain.Conversation
	e := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var e error
		c, e = supportLoad(tx, id, key, actor, at)
		if e != nil {
			return e
		}
		if actor != "" {
			if e = tx.Model(&supportNotice{}).Where("recipient_id=? AND request_id=? AND read_at IS NULL", actor, id).Update("read_at", at).Error; e != nil {
				return e
			}
		}
		return supportAudit(tx, id, actor, "read", at)
	})
	return c, e
}
func (r SupportRepository) Reply(ctx context.Context, id, key, actor string, m domain.Message, at time.Time) (domain.Conversation, error) {
	var c domain.Conversation
	e := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Serialize appends without granting mutation rights on immutable request
		// ownership or recovery credentials.
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?,0))", "support:"+id).Error; e != nil {
			return e
		}
		var e error
		c, e = supportLoad(tx, id, key, actor, at)
		if e != nil {
			return e
		}
		for _, old := range c.Messages {
			if old.ID == m.ID {
				if old.Body != m.Body || old.Operator != m.Operator {
					return ports.ErrIdempotencyConflict
				}
				return supportAudit(tx, id, actor, "read", at)
			}
		}
		if len(c.Messages) >= 100 {
			return ports.ErrConflict
		}
		if e = tx.Create(&supportMessage{ID: m.ID, RequestID: id, Body: m.Body, Operator: m.Operator, CreatedAt: at}).Error; e != nil {
			return e
		}
		if !m.Operator {
			if e = supportNotify(tx, m.ID, id, at); e != nil {
				return e
			}
		}
		c.Messages = append(c.Messages, m)
		return supportAudit(tx, id, actor, "reply", at)
	})
	return c, e
}
func (r SupportRepository) Inbox(ctx context.Context, actor, after string, limit int, at time.Time) ([]domain.Conversation, error) {
	items := []domain.Conversation{}
	e := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, e := supportAdmin(tx, actor)
		if e != nil {
			return e
		}
		if !ok {
			return ports.ErrNotFound
		}
		var rows []supportRequest
		q := tx.Where("expires_at>?", at)
		if after != "" {
			q = q.Where("id>?", after)
		}
		if e = q.Order("id").Limit(limit).Find(&rows).Error; e != nil {
			return e
		}
		for _, row := range rows {
			items = append(items, domain.Conversation{ID: row.ID, Subject: row.Subject, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt})
		}
		return supportAudit(tx, "inbox", actor, "list", at)
	})
	return items, e
}
func (r SupportRepository) Notifications(ctx context.Context, actor string, at time.Time) ([]domain.Notice, error) {
	items := []domain.Notice{}
	e := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ok, e := supportAdmin(tx, actor)
		if e != nil {
			return e
		}
		if !ok {
			return ports.ErrNotFound
		}
		var rows []supportNotice
		if e = tx.Joins("JOIN support_request_models r ON r.id=support_notice_models.request_id AND r.expires_at>?", at).Where("recipient_id=? AND read_at IS NULL", actor).Order("support_notice_models.created_at DESC").Limit(50).Find(&rows).Error; e != nil {
			return e
		}
		for _, row := range rows {
			items = append(items, domain.Notice{ID: row.ID, RequestID: row.RequestID, CreatedAt: row.CreatedAt, Read: row.ReadAt != nil})
		}
		return supportAudit(tx, "notices", actor, "notices", at)
	})
	return items, e
}

func (r SupportRepository) Denied(ctx context.Context, actor string, at time.Time) error {
	return supportAudit(r.DB.WithContext(ctx), "access", actor, "deny", at)
}
