package path

import (
	"context"
	"errors"
	"testing"
	"time"

	platformapp "github.com/elsell/hour-paths/apps/api/internal/app"
	"github.com/elsell/hour-paths/apps/api/internal/domain/audit"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestReviewInvitationRecipientReturnsOnlyCanonicalPublicIdentityAfterAuthorization(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	var checks []authorizationCall
	var usernames []string
	var gets []repositoryGet
	var events []audit.Event
	dependencies := invitationDependencies(now)
	dependencies.Authorizer = controlledAuthorizer{allowed: true, calls: &checks}
	dependencies.Paths = controlledRepository{entity: invitationTestPath(now), gets: &gets}
	dependencies.Directory = controlledInvitationDirectory{
		recipient: InvitationRecipient{
			UserID: "recipient-id", Username: "Reader.One", DisplayName: "Reader One",
			ProfileVisibility: identity.ProfileVisibilityPrivate,
		},
		usernames: &usernames,
	}
	dependencies.Audits = controlledAudits{events: &events}

	recipient, err := NewInvitationService(dependencies).ReviewRecipient(
		context.Background(), "Bearer valid", "path-1", "reader.one",
	)
	if err != nil {
		t.Fatal(err)
	}
	if recipient != (InvitationRecipientReview{
		UserID: "recipient-id", Username: "Reader.One", DisplayName: "Reader One",
	}) {
		t.Fatalf("review recipient = %+v", recipient)
	}
	if len(checks) != 1 || checks[0] != (authorizationCall{"path", "path-1", "manage_members", "creator"}) {
		t.Fatalf("authorization checks = %+v", checks)
	}
	if len(gets) != 1 || gets[0] != (repositoryGet{userID: "creator", id: "path-1"}) ||
		len(usernames) != 1 || usernames[0] != "reader.one" {
		t.Fatalf("path gets=%+v username lookups=%+v", gets, usernames)
	}
	if len(events) != 1 || events[0].Action != audit.UserViewed ||
		events[0].OwnerUserID != "recipient-id" || events[0].ActorUserID != "creator" ||
		events[0].TargetType != "user" || events[0].TargetID != "recipient-id" ||
		events[0].Outcome != audit.Succeeded || !events[0].OccurredAt.Equal(now) {
		t.Fatalf("review audit = %+v", events)
	}
}

func TestReviewInvitationRecipientConcealsDeniedAndRejectsArchivedOrRateLimitedReads(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*InvitationDependencies, *[]string, *[]audit.Event)
		want   error
	}{
		{
			name: "denied",
			mutate: func(dependencies *InvitationDependencies, _ *[]string, events *[]audit.Event) {
				dependencies.Authorizer = controlledAuthorizer{allowed: false}
				dependencies.Audits = controlledAudits{events: events}
			},
			want: platformapp.ErrForbidden,
		},
		{
			name: "archived",
			mutate: func(dependencies *InvitationDependencies, _ *[]string, _ *[]audit.Event) {
				path := invitationTestPath(now)
				path.ArchivedAt, path.UpdatedAt = now, now
				dependencies.Paths = controlledRepository{entity: path}
			},
			want: ports.ErrConflict,
		},
		{
			name: "rate limited",
			mutate: func(dependencies *InvitationDependencies, _ *[]string, _ *[]audit.Event) {
				dependencies.AuditRateLimiter = controlledLimiter{denied: true}
			},
			want: platformapp.ErrRateLimited,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var usernames []string
			var events []audit.Event
			dependencies := invitationDependencies(now)
			dependencies.Directory = controlledInvitationDirectory{
				recipient: InvitationRecipient{
					UserID: "recipient", Username: "reader", DisplayName: "Reader",
					ProfileVisibility: identity.ProfileVisibilityPublic,
				},
				usernames: &usernames,
			}
			test.mutate(&dependencies, &usernames, &events)
			_, err := NewInvitationService(dependencies).ReviewRecipient(
				context.Background(), "Bearer valid", "path-1", "reader",
			)
			if !errors.Is(err, test.want) {
				t.Fatalf("ReviewRecipient error = %v, want %v", err, test.want)
			}
			if len(usernames) != 0 {
				t.Fatalf("rejected review reached directory: %v", usernames)
			}
			if test.name == "denied" {
				if len(events) != 1 || events[0].Action != audit.ResourceAccessDenied ||
					events[0].Outcome != audit.Denied || events[0].TargetID != "path-1" {
					t.Fatalf("denied audit = %+v", events)
				}
			}
		})
	}
}

func TestReviewInvitationRecipientFailsClosedOnMalformedDirectoryProjectionOrAuditFailure(t *testing.T) {
	now := time.Date(2026, 7, 23, 20, 0, 0, 0, time.UTC)
	for _, malformed := range []InvitationRecipient{
		{Username: "reader", DisplayName: "Reader", ProfileVisibility: identity.ProfileVisibilityPublic},
		{UserID: "recipient", Username: "different", DisplayName: "Reader", ProfileVisibility: identity.ProfileVisibilityPublic},
		{UserID: "recipient", Username: "reader", ProfileVisibility: identity.ProfileVisibilityPublic},
		{UserID: "recipient", Username: "reader", DisplayName: "Reader", ProfileVisibility: "followers"},
	} {
		dependencies := invitationDependencies(now)
		dependencies.Directory = controlledInvitationDirectory{recipient: malformed}
		if _, err := NewInvitationService(dependencies).ReviewRecipient(
			context.Background(), "Bearer valid", "path-1", "reader",
		); !errors.Is(err, errInvalidInvitationDependencies) {
			t.Fatalf("malformed projection %+v error = %v", malformed, err)
		}
	}

	dependencies := invitationDependencies(now)
	dependencies.Directory = controlledInvitationDirectory{recipient: InvitationRecipient{
		UserID: "recipient", Username: "reader", DisplayName: "Reader",
		ProfileVisibility: identity.ProfileVisibilityPublic,
	}}
	dependencies.Audits = controlledAudits{err: errors.New("audit unavailable")}
	if recipient, err := NewInvitationService(dependencies).ReviewRecipient(
		context.Background(), "Bearer valid", "path-1", "reader",
	); err == nil || recipient != (InvitationRecipientReview{}) {
		t.Fatalf("audit failure returned recipient %+v, %v", recipient, err)
	}
}
