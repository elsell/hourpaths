package path

import (
	"context"
	"errors"
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
