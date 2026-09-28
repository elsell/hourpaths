package social

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestReactionRosterBindsCursorAndFailsClosed(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	repository := &controlledReactions{target: ReactionTarget{EventID: "practice:event", PathID: "path", OwnerUserID: "owner"}, roster: ReactionRosterPage{Items: []ReactionRosterItem{{Profile: domain.PublicProfile{ID: "reactor", Username: "reactor", DisplayName: "Reactor", Relationship: domain.RelationshipNone}, ReactedAt: now.Add(-time.Minute)}}, HasMore: true}}
	service, authorizer := reactionService(repository)
	service.Clock = fixedClock{now: now}
	audits := &controlledAudits{}
	service.Audits = audits
	profiles, cursor, err := service.ListPracticeReactions(context.Background(), "Bearer session", "practice:event", domain.ReactionHeart, "", 1)
	if err != nil || len(profiles) != 1 || cursor == "" || len(audits.events) != 1 || audits.events[0].Action != audit.ResourceListed {
		t.Fatalf("profiles=%+v cursor=%q err=%v audit=%+v", profiles, cursor, err, audits.events)
	}
	for _, test := range []struct {
		event    string
		reaction domain.Reaction
	}{{"practice:other", domain.ReactionHeart}, {"practice:event", domain.ReactionFire}} {
		if _, _, err := service.ListPracticeReactions(context.Background(), "Bearer session", test.event, test.reaction, cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("cross-scope cursor accepted: %v", err)
		}
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "other", Scopes: []string{"api:user"}}}
	if _, _, err := service.ListPracticeReactions(context.Background(), "Bearer other", "practice:event", domain.ReactionHeart, cursor, 1); !errors.Is(err, ports.ErrInvalidArgument) {
		t.Fatalf("cross-user cursor accepted: %v", err)
	}
	service.Auth = controlledAuth{principal: ports.Principal{UserID: "viewer", Scopes: []string{"api:user"}}}
	authorizer.allowed["path"] = false
	if got, _, err := service.ListPracticeReactions(context.Background(), "Bearer session", "practice:event", domain.ReactionHeart, "", 1); !errors.Is(err, ports.ErrNotFound) || got != nil || audits.events[len(audits.events)-1].Action != audit.ResourceAccessDenied {
		t.Fatalf("policy denial leaked: %+v %v", got, err)
	}
	authorizer.allowed["path"] = true
	audits.err = errors.New("audit unavailable")
	if got, _, err := service.ListPracticeReactions(context.Background(), "Bearer session", "practice:event", domain.ReactionHeart, "", 1); err == nil || got != nil {
		t.Fatalf("audit failure leaked: %+v %v", got, err)
	}
}
