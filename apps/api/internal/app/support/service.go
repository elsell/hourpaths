package support

import (
	"context"
	platform "github.com/elsell/hour-paths/apps/api/internal/app"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/support"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"time"
)

type Repository interface {
	Create(context.Context, domain.Conversation) (domain.Conversation, error)
	Get(context.Context, string, string, string, time.Time) (domain.Conversation, error)
	Reply(context.Context, string, string, string, domain.Message, time.Time) (domain.Conversation, error)
	IsAdmin(context.Context, string) (bool, error)
	Denied(context.Context, string, time.Time) error
	Inbox(context.Context, string, string, int, time.Time) ([]domain.Conversation, error)
	Notifications(context.Context, string, time.Time) ([]domain.Notice, error)
}
type Service struct {
	Repository Repository
	Auth       ports.Authenticator
	Authorizer ports.Authorizer
	Limiter    ports.AuditRateLimiter
	Clock      ports.Clock
}

func (s *Service) ready() error {
	if s == nil || s.Repository == nil || s.Auth == nil || s.Authorizer == nil || s.Limiter == nil || s.Clock == nil {
		return ports.ErrUnavailable
	}
	return nil
}
func (s *Service) key(id, key string) (string, error) {
	if e := s.ready(); e != nil {
		return "", e
	}
	if !domain.ValidTicketID(id) {
		return "", ports.ErrNotFound
	}
	hash, e := domain.KeyDigest(key)
	if e != nil {
		return "", ports.ErrNotFound
	}
	if !s.Limiter.Allow("support:"+hash, s.Clock.Now()) {
		return "", platform.ErrRateLimited
	}
	return hash, nil
}
func (s *Service) Create(ctx context.Context, id, key, subject, body string) (domain.Conversation, error) {
	hash, e := s.key(id, key)
	if e != nil {
		return domain.Conversation{}, e
	}
	subject, e = domain.SubjectText(subject)
	if e != nil {
		return domain.Conversation{}, ports.ErrInvalidArgument
	}
	body, e = domain.MessageText(body)
	if e != nil {
		return domain.Conversation{}, ports.ErrInvalidArgument
	}
	now := s.Clock.Now().UTC()
	c, e := s.Repository.Create(ctx, domain.Conversation{ID: id, KeyHash: hash, Subject: subject, CreatedAt: now, ExpiresAt: now.Add(domain.Lifetime), Messages: []domain.Message{{ID: id, Body: body, CreatedAt: now}}})
	if e != nil {
		return c, e
	}
	// A failed authorization publication leaves a private persisted request. Retrying
	// this same immutable request repairs the grant without duplicating content.
	if e = s.Authorizer.WriteRelationship(ctx, "resource", "support-"+id, "owner", "user", hash); e != nil {
		return domain.Conversation{}, ports.ErrUnavailable
	}
	return c, nil
}
func (s *Service) Read(ctx context.Context, id, key string) (domain.Conversation, error) {
	hash, e := s.key(id, key)
	if e != nil {
		return domain.Conversation{}, e
	}
	return s.read(ctx, id, hash, "")
}
func (s *Service) read(ctx context.Context, id, hash, admin string) (domain.Conversation, error) {
	if admin == "" {
		ok, e := s.Authorizer.Check(ctx, "resource", "support-"+id, "view", hash)
		if e != nil {
			return domain.Conversation{}, ports.ErrUnavailable
		}
		if !ok {
			return domain.Conversation{}, s.denied(ctx, "requester")
		}
	}
	return s.Repository.Get(ctx, id, hash, admin, s.Clock.Now().UTC())
}

func (s *Service) Reply(ctx context.Context, id, key, messageID, body string) (domain.Conversation, error) {
	hash, e := s.key(id, key)
	if e != nil {
		return domain.Conversation{}, e
	}
	if _, e = s.read(ctx, id, hash, ""); e != nil {
		return domain.Conversation{}, e
	}
	return s.reply(ctx, id, hash, "", messageID, body)
}
func (s *Service) reply(ctx context.Context, id, hash, admin, messageID, body string) (domain.Conversation, error) {
	if _, e := uuid.Parse(messageID); e != nil {
		return domain.Conversation{}, ports.ErrInvalidArgument
	}
	body, e := domain.MessageText(body)
	if e != nil {
		return domain.Conversation{}, ports.ErrInvalidArgument
	}
	now := s.Clock.Now().UTC()
	return s.Repository.Reply(ctx, id, hash, admin, domain.Message{ID: messageID, Body: body, Operator: admin != "", CreatedAt: now}, now)
}
func (s *Service) admin(ctx context.Context, authorization string) (string, error) {
	if e := s.ready(); e != nil {
		return "", e
	}
	p, e := s.Auth.Authenticate(ctx, authorization)
	if e != nil {
		return "", e
	}
	if p.UserID == "" || len(p.Scopes) != 1 || p.Scopes[0] != "api:user" {
		if p.UserID != "" {
			if !s.Limiter.Allow(p.UserID, s.Clock.Now()) {
				return "", platform.ErrRateLimited
			}
			if e := s.Repository.Denied(ctx, p.UserID, s.Clock.Now().UTC()); e != nil {
				return "", ports.ErrUnavailable
			}
		}
		return "", platform.ErrUnauthenticated
	}
	if !s.Limiter.Allow(p.UserID, s.Clock.Now()) {
		return "", platform.ErrRateLimited
	}
	allowed, e := s.Repository.IsAdmin(ctx, p.UserID)
	if e != nil {
		return "", ports.ErrUnavailable
	}
	if !allowed {
		return "", s.denied(ctx, p.UserID)
	}
	allowed, e = s.Authorizer.Check(ctx, "resource", domain.AdminResource, "view", p.UserID)
	if e != nil {
		return "", ports.ErrUnavailable
	}
	if !allowed {
		return "", s.denied(ctx, p.UserID)
	}
	return p.UserID, nil
}
func (s *Service) AdminRead(ctx context.Context, authorization, id string) (domain.Conversation, error) {
	actor, e := s.admin(ctx, authorization)
	if e != nil {
		return domain.Conversation{}, e
	}
	return s.read(ctx, id, "", actor)
}
func (s *Service) AdminReply(ctx context.Context, authorization, id, messageID, body string) (domain.Conversation, error) {
	actor, e := s.admin(ctx, authorization)
	if e != nil {
		return domain.Conversation{}, e
	}
	return s.reply(ctx, id, "", actor, messageID, body)
}
func (s *Service) Inbox(ctx context.Context, authorization, after string, limit int) ([]domain.Conversation, error) {
	actor, e := s.admin(ctx, authorization)
	if e != nil {
		return nil, e
	}
	if limit < 1 || limit > 50 {
		return nil, ports.ErrInvalidArgument
	}
	return s.Repository.Inbox(ctx, actor, after, limit, s.Clock.Now().UTC())
}
func (s *Service) Notifications(ctx context.Context, authorization string) ([]domain.Notice, error) {
	actor, e := s.admin(ctx, authorization)
	if e != nil {
		return nil, e
	}
	return s.Repository.Notifications(ctx, actor, s.Clock.Now().UTC())
}

// AuthorizeAdministrator is the shared current platform-role decision; it never
// treats invitation administration or profile data as platform privileges.
func (s *Service) AuthorizeAdministrator(ctx context.Context, authorization string) (string, error) {
	return s.admin(ctx, authorization)
}

func (s *Service) denied(ctx context.Context, actor string) error {
	if e := s.Repository.Denied(ctx, actor, s.Clock.Now().UTC()); e != nil {
		return ports.ErrUnavailable
	}
	return ports.ErrNotFound
}
