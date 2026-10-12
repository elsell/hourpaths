package social

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type unavailableTextModerator struct {
	text    string
	surface ports.TextSurface
}

func (m *unavailableTextModerator) Check(_ context.Context, surface ports.TextSurface, text string) error {
	m.surface, m.text = surface, text
	return errors.New("private processor diagnostic")
}

func TestTextModerationPreventsCommentWritesAfterAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 28, 16, 0, 0, 0, time.UTC)
	for _, edit := range []bool{false, true} {
		repository := &controlledComments{target: PracticeCommentTarget{EventID: "practice:activity", PathID: "path", OwnerUserID: "owner"}}
		repository.resolved = ResolvedPracticeComment{Target: repository.target, Comment: validComment("comment", "practice:activity", "viewer", 1, now.Add(-time.Minute), now.Add(-time.Minute), "Old")}
		service, authorizer, _ := commentService(repository)
		moderator := &unavailableTextModerator{}
		service.TextModerator = moderator
		call := func() error {
			if edit {
				_, err := service.EditPracticeComment(context.Background(), "Bearer session", "practice:activity", "comment", "  New text  ", 1, "moderation-edit-001")
				return err
			}
			_, err := service.CreatePracticeComment(context.Background(), "Bearer session", "practice:activity", "  New text  ", "moderation-create-001")
			return err
		}
		if err := call(); !errors.Is(err, ports.ErrUnavailable) {
			t.Fatalf("edit=%v: %v", edit, err)
		}
		if moderator.text != "New text" || moderator.surface != ports.CommentText || repository.created != nil || repository.edited != nil {
			t.Fatalf("text reached persistence or failed normalization: %+v", moderator)
		}
		moderator.text = ""
		authorizer.allowed["path"] = false
		if err := call(); err == nil || moderator.text != "" {
			t.Fatal("unauthorized text reached moderation")
		}
		authorizer.allowed["path"] = true
		service.TextModerator = nil
		if err := call(); !errors.Is(err, ports.ErrUnavailable) {
			t.Fatal("missing moderator accepted")
		}
	}
}
