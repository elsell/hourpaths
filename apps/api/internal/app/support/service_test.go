package support

import (
	"context"
	"errors"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/support"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"strings"
	"testing"
	"time"
)

type memorySupport struct {
	tickets map[string]domain.Conversation
	admins  map[string]bool
	now     time.Time
	allowed bool
}

func (f *memorySupport) Now() time.Time               { return f.now }
func (f *memorySupport) Allow(string, time.Time) bool { return true }
func (f *memorySupport) Authenticate(_ context.Context, token string) (ports.Principal, error) {
	if token != "admin-session" {
		return ports.Principal{}, ports.ErrNotFound
	}
	return ports.Principal{UserID: "admin", Scopes: []string{"api:user"}}, nil
}
func (f *memorySupport) Check(context.Context, string, string, string, string) (bool, error) {
	return f.allowed, nil
}
func (f *memorySupport) WriteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *memorySupport) DeleteRelationship(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *memorySupport) Create(_ context.Context, c domain.Conversation) (domain.Conversation, error) {
	if old, ok := f.tickets[c.ID]; ok {
		if old.KeyHash != c.KeyHash || old.Subject != c.Subject || old.Messages[0].Body != c.Messages[0].Body {
			return domain.Conversation{}, ports.ErrIdempotencyConflict
		}
		return old, nil
	}
	f.tickets[c.ID] = c
	return c, nil
}
func (f *memorySupport) Get(_ context.Context, id, key, actor string, at time.Time) (domain.Conversation, error) {
	c, ok := f.tickets[id]
	if !ok || (!f.admins[actor] && key != c.KeyHash) || !at.Before(c.ExpiresAt) {
		return domain.Conversation{}, ports.ErrNotFound
	}
	return c, nil
}
func (f *memorySupport) IsAdmin(_ context.Context, id string) (bool, error) { return f.admins[id], nil }
func (f *memorySupport) Reply(_ context.Context, id, key, actor string, m domain.Message, at time.Time) (domain.Conversation, error) {
	c, e := f.Get(context.Background(), id, key, actor, at)
	if e != nil {
		return c, e
	}
	for _, old := range c.Messages {
		if old.ID == m.ID {
			if old.Body != m.Body || old.Operator != m.Operator {
				return c, ports.ErrIdempotencyConflict
			}
			return c, nil
		}
	}
	c.Messages = append(c.Messages, m)
	f.tickets[id] = c
	return c, nil
}
func (f *memorySupport) Inbox(context.Context, string, string, int, time.Time) ([]domain.Conversation, error) {
	return []domain.Conversation{}, nil
}
func (f *memorySupport) Notifications(context.Context, string, time.Time) ([]domain.Notice, error) {
	return []domain.Notice{}, nil
}
func fixture() (*Service, *memorySupport) {
	f := &memorySupport{tickets: map[string]domain.Conversation{}, admins: map[string]bool{"admin": true}, now: time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), allowed: true}
	return &Service{Repository: f, Auth: f, Authorizer: f, Limiter: f, Clock: f}, f
}
func TestRecoveryKeyScopeAuthorizationAndExpiry(t *testing.T) {
	s, f := fixture()
	ctx := context.Background()
	key := strings.Repeat("a", 64)
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	if _, e := s.Create(ctx, id, key, "Sign in", "Cannot sign in"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Read(ctx, id, strings.Repeat("b", 64)); !errors.Is(e, ports.ErrNotFound) {
		t.Fatalf("foreign key: %v", e)
	}
	f.allowed = false
	if _, e := s.Read(ctx, id, key); e == nil {
		t.Fatal("authorization denial ignored")
	}
	f.allowed = true
	f.now = f.now.Add(31 * 24 * time.Hour)
	if _, e := s.Read(ctx, id, key); !errors.Is(e, ports.ErrNotFound) {
		t.Fatalf("expired: %v", e)
	}
}
func TestRetryAndRevokedOperator(t *testing.T) {
	s, f := fixture()
	ctx := context.Background()
	key := strings.Repeat("a", 64)
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for i := 0; i < 2; i++ {
		if _, e := s.Create(ctx, id, key, "Help", "Something went wrong"); e != nil {
			t.Fatal(e)
		}
	}
	if len(f.tickets) != 1 {
		t.Fatal("duplicate")
	}
	if _, e := s.Create(ctx, id, key, "Changed", "Something went wrong"); !errors.Is(e, ports.ErrIdempotencyConflict) {
		t.Fatal(e)
	}
	if _, e := s.AdminRead(ctx, "admin-session", id); e != nil {
		t.Fatal(e)
	}
	delete(f.admins, "admin")
	if _, e := s.AdminRead(ctx, "admin-session", id); e == nil {
		t.Fatal("revoked administrator admitted")
	}
}

func (f *memorySupport) Denied(context.Context, string, time.Time) error { return nil }
