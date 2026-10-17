package adminattention

import (
	"context"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/adminattention"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"github.com/google/uuid"
	"time"
)

type Administrator interface {
	AuthorizeAdministrator(context.Context, string) (string, error)
}
type Repository interface {
	Notifications(context.Context, string, time.Time) ([]domain.Notice, error)
	Read(context.Context, string, string, time.Time) (domain.Record, error)
}
type Service struct {
	Administrator Administrator
	Repository    Repository
	Clock         ports.Clock
}

func (s *Service) Notifications(ctx context.Context, authorization string) ([]domain.Notice, error) {
	if s == nil || s.Administrator == nil || s.Repository == nil || s.Clock == nil {
		return nil, ports.ErrUnavailable
	}
	actor, e := s.Administrator.AuthorizeAdministrator(ctx, authorization)
	if e != nil {
		return nil, e
	}
	return s.Repository.Notifications(ctx, actor, s.Clock.Now().UTC())
}
func (s *Service) Read(ctx context.Context, authorization, id string) (domain.Record, error) {
	if s == nil || s.Administrator == nil || s.Repository == nil || s.Clock == nil {
		return domain.Record{}, ports.ErrUnavailable
	}
	actor, e := s.Administrator.AuthorizeAdministrator(ctx, authorization)
	if e != nil {
		return domain.Record{}, e
	}
	if _, e = uuid.Parse(id); e != nil {
		return domain.Record{}, ports.ErrNotFound
	}
	return s.Repository.Read(ctx, actor, id, s.Clock.Now().UTC())
}
