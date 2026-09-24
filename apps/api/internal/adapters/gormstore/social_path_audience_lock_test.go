package gormstore

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	sociallock "github.com/elsell/hour-paths/apps/api/internal/adapters/gormstore/sociallock"
)

func TestSocialPathAudienceKeyIsSharedAndLengthDelimited(t *testing.T) {
	if got, want := sociallock.PathAudienceKey("path"), socialLockKey("social-path-audience", "path"); got != want {
		t.Fatalf("PathAudienceKey()=%q writer key=%q", got, want)
	}
}

func TestEveryTargetNotificationWriterTakesPathAudienceLockBeforeMutation(t *testing.T) {
	_, current, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source")
	}
	root := filepath.Dir(current)
	for _, source := range []string{
		"social_reaction_repository.go", "social_comment_repository.go",
		"social_comment_heart_repository.go", "social_nudge_repository.go",
	} {
		content, err := os.ReadFile(filepath.Join(root, source))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "lockSocialPathAudience(tx,") {
			t.Fatalf("%s does not take the shared Path audience lock", source)
		}
	}
}
