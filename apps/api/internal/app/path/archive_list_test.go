package path

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/app/shared"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestListArchivedPathsUsesDistinctOwnerBoundPageAndCursor(t *testing.T) {
	var lists []repositoryList
	created := time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)
	archived := domain.Entity{ID: "archived", OwnerUserID: "member", CreatedAt: created, UpdatedAt: created.Add(time.Hour), ArchivedAt: created.Add(time.Hour)}
	dependencies := configuredDependencies()
	dependencies.Repository = controlledRepository{page: Page{Items: []domain.Entity{archived}, HasMore: true}, lists: &lists}

	items, cursor, err := New(dependencies).ListArchived(context.Background(), "Bearer valid", "", 25)
	if err != nil || len(items) != 1 || items[0].ID != "archived" || cursor == "" {
		t.Fatalf("ListArchived() = %+v, %q, %v", items, cursor, err)
	}
	if len(lists) != 1 || !lists[0].page.Archived {
		t.Fatalf("repository list = %+v", lists)
	}
	payload, err := shared.DecodeCursor(dependencies.CursorSigningKey, cursor)
	if err != nil || payload.Domain != "path-archived" {
		t.Fatalf("archived cursor = %+v, %v", payload, err)
	}
	if _, _, err := New(dependencies).List(context.Background(), "Bearer valid", cursor, 25); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("active list accepted archived cursor: %v", err)
	}
	activeCursor := mustCursor(t, dependencies.CursorSigningKey, "member", "path")
	if _, _, err := New(dependencies).ListArchived(context.Background(), "Bearer valid", activeCursor, 25); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("archived list accepted active cursor: %v", err)
	}
}
