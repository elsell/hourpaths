package adminattention

import (
	"context"
	"errors"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/adminattention"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
	"time"
)

type controlled struct {
	allowed bool
	reads   int
}

func (f *controlled) AuthorizeAdministrator(context.Context, string) (string, error) {
	if !f.allowed {
		return "", ports.ErrNotFound
	}
	return "admin", nil
}
func (f *controlled) Now() time.Time { return time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC) }
func (f *controlled) Notifications(context.Context, string, time.Time) ([]domain.Notice, error) {
	f.reads++
	return []domain.Notice{{Kind: "report"}}, nil
}
func (f *controlled) Read(context.Context, string, string, time.Time) (domain.Record, error) {
	f.reads++
	return domain.Record{Kind: "report"}, nil
}
func TestRevocationPreventsNotificationAndRecordReads(t *testing.T) {
	f := &controlled{allowed: true}
	s := Service{f, f, f}
	ctx := context.Background()
	if _, e := s.Notifications(ctx, "session"); e != nil {
		t.Fatal(e)
	}
	f.allowed = false
	if _, e := s.Notifications(ctx, "session"); !errors.Is(e, ports.ErrNotFound) {
		t.Fatal(e)
	}
	if _, e := s.Read(ctx, "session", "6de631a7-a5be-438e-9200-2650a4e8891d"); !errors.Is(e, ports.ErrNotFound) {
		t.Fatal(e)
	}
	if f.reads != 1 {
		t.Fatal("repository accessed after revocation")
	}
}
