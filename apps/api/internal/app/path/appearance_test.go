package path

import (
	"context"
	"errors"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
	"testing"
)

func (r controlledRepository) ReadAppearance(context.Context, string, domain.ID) (domain.Appearance, error) {
	return domain.Appearance{}, r.err
}
func (r controlledRepository) SaveAppearance(_ context.Context, command AppearanceCommand) (domain.Appearance, error) {
	value := command.Appearance
	value.Revision++
	return value, r.err
}

func TestPersonalAppearanceRequiresSessionAndViewPermission(t *testing.T) {
	for _, authorization := range []string{"", "Bearer malformed", "Bearer expired"} {
		service := New(configuredDependencies())
		if _, err := service.ReadAppearance(context.Background(), authorization, "path"); err == nil {
			t.Fatal("accepted invalid session")
		}
	}
	for _, permission := range []controlledAuthorizer{{allowed: false}, {err: errors.New("authorization unavailable")}} {
		dependencies := configuredDependencies()
		dependencies.Authorizer = permission
		if _, err := New(dependencies).SaveAppearance(context.Background(), "Bearer valid", "appearance-key-0001", "path", domain.Appearance{Color: "mint", Emoji: "🧑🏽‍💻"}); err == nil {
			t.Fatal("accepted denied or failed authorization")
		}
	}
	dependencies := configuredDependencies()
	result, err := New(dependencies).SaveAppearance(context.Background(), "Bearer valid", "appearance-key-0001", "path", domain.Appearance{Color: "mint", Emoji: "🧑🏽‍💻"})
	if err != nil || result.Revision != 1 || result.Color != "mint" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := New(dependencies).SaveAppearance(context.Background(), "Bearer valid", "appearance-key-0002", "path", domain.Appearance{Color: "mint", Emoji: "not emoji"}); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("invalid emoji err=%v", err)
	}
}

func TestAppearanceMembershipDenialIsAuditedAfterPublicViewPermission(t *testing.T) {
	for _, save := range []bool{false, true} {
		dependencies := configuredDependencies()
		dependencies.Repository = controlledRepository{err: ports.ErrNotFound}
		var events []audit.Event
		dependencies.Audits = controlledAudits{events: &events}
		service := New(dependencies)
		var err error
		if save {
			_, err = service.SaveAppearance(context.Background(), "Bearer valid", "appearance-public-001", "public-path", domain.Appearance{Color: "mint", Emoji: "🌱"})
		} else {
			_, err = service.ReadAppearance(context.Background(), "Bearer valid", "public-path")
		}
		if !errors.Is(err, ports.ErrNotFound) || len(events) != 1 || events[0].Action != audit.ResourceAccessDenied {
			t.Fatalf("save=%v err=%v events=%+v", save, err, events)
		}
	}
}
