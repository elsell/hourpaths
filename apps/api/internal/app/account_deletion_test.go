package app

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

type controlledAccountDeletion struct {
	commands []AccountDeletionCommand
	err      error
}

func (r *controlledAccountDeletion) DeleteAccount(_ context.Context, command AccountDeletionCommand) error {
	if r.err != nil {
		return r.err
	}
	r.commands = append(r.commands, command)
	return nil
}

func (r *controlledAccountDeletion) DeletionReceipt(_ context.Context, userID string, hash []byte, now time.Time) (bool, error) {
	for _, command := range r.commands {
		if command.UserID == userID && bytes.Equal(command.ReceiptHash, hash) && now.Before(command.DeletedAt.Add(30*24*time.Hour)) {
			return true, nil
		}
	}
	return false, r.err
}

func TestAccountDeletionRequiresConfirmationForTheCurrentlyReviewedAccount(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		confirmed bool
		reviewed  string
		denied    bool
		want      error
	}{
		{"unconfirmed", false, "owner", false, ports.ErrInvalidArgument},
		{"account switched after review", true, "previous-account", false, ports.ErrConflict},
		{"rate limited", true, "owner", true, ErrRateLimited},
		{"confirmed current account", true, "owner", false, nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			repository := &controlledAccountDeletion{}
			application := App{
				Auth:             fakeAuth{principal: ports.Principal{UserID: "owner", Scopes: []string{"api:user"}}},
				Users:            fakeUsers{user: identity.User{ID: "owner", Status: identity.StatusActive}},
				AccountDeletion:  repository,
				DeletionJournal:  repository,
				Clock:            fakeClock{now: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)},
				AuditRateLimiter: fakeAuditRateLimiter{denied: scenario.denied},
			}
			err := application.DeleteAccount(context.Background(), "Bearer current-session", AccountDeletionRequest{Confirmed: scenario.confirmed, ReviewedUserID: scenario.reviewed, ReceiptSecret: strings.Repeat("a", 64)})
			if !errors.Is(err, scenario.want) {
				t.Fatalf("error = %v; want %v", err, scenario.want)
			}
			if scenario.want != nil {
				if len(repository.commands) != 0 {
					t.Fatal("rejected deletion reached persistence")
				}
				return
			}
			if len(repository.commands) != 1 {
				t.Fatal("confirmed deletion not persisted")
			}
			command := repository.commands[0]
			if command.UserID != "owner" || !command.Audit.Valid() || command.Audit.Action != audit.ResourceDeleted || command.Audit.TargetType != "account" || command.Audit.ActorUserID != "owner" || command.Audit.OccurredAt != command.DeletedAt {
				t.Fatalf("deletion must carry owner-scoped atomic audit evidence: %+v", command)
			}
		})
	}
}

func TestDeletionReceiptProvesOnlyTheExactCompletedAccountWithoutAValidSession(t *testing.T) {
	secret := strings.Repeat("b", 64)
	repository := &controlledAccountDeletion{}
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	application := App{Auth: fakeAuth{}, Users: fakeUsers{user: identity.User{ID: "owner", Status: identity.StatusActive}}, AccountDeletion: repository, DeletionJournal: repository, Clock: fakeClock{now: now}, AuditRateLimiter: fakeAuditRateLimiter{}, Audits: fakeAudits{}}
	if err := application.DeleteAccount(context.Background(), "Bearer session", AccountDeletionRequest{Confirmed: true, ReviewedUserID: "owner", ReceiptSecret: secret}); err != nil {
		t.Fatal(err)
	}
	application.Auth = fakeAuth{err: ErrUnauthenticated}
	if err := application.ConfirmAccountDeletion(context.Background(), "owner", secret); err != nil {
		t.Fatalf("lost-response receipt failed: %v", err)
	}
	for _, input := range []struct{ owner, secret string }{{"other", secret}, {"owner", strings.Repeat("c", 64)}} {
		if err := application.ConfirmAccountDeletion(context.Background(), input.owner, input.secret); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("unproven receipt: %v", err)
		}
	}
	application.Clock = fakeClock{now: now.Add(30 * 24 * time.Hour)}
	if err := application.ConfirmAccountDeletion(context.Background(), "owner", secret); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expired receipt: %v", err)
	}
}

func (r *controlledAccountDeletion) Admit(_ context.Context, record DeletionRecord) (DeletionRecord, error) {
	return record, r.err
}
