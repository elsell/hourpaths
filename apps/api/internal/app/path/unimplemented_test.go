package path

import (
	"context"
	"errors"
	"testing"

	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestUnimplementedPathOperationsRemainPolicyUnavailable(t *testing.T) {
	service := New(configuredDependencies())
	for _, run := range []func() error{
		func() error {
			_, err := service.Update(context.Background(), "Bearer valid", "path-id", domain.Attributes{})
			return err
		},
	} {
		if err := run(); !errors.Is(err, ports.ErrAuthorizationPolicyNotConfigured) {
			t.Fatalf("unimplemented operation returned %v", err)
		}
	}
}
