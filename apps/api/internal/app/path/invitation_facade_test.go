package path

import (
	"context"
	"errors"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPathServiceInvitationFacadeUsesItsTypedRepository(t *testing.T) {
	dependencies := configuredDependencies()
	_, _, err := New(dependencies).ListPending(context.Background(), "Bearer valid", "", 25)
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("ListPending() error = %v, want repository sentinel", err)
	}
}
