package path

import (
	"context"
	"errors"
	"testing"
	"time"

	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func practiceReactionNotification(createdAt time.Time) InvitationNotificationProjection {
	return InvitationNotificationProjection{
		ID: "reaction-notification", Kind: NotificationPracticeReaction,
		Presentation: NotificationInformational, CreatedAt: createdAt,
		Actor:  InvitationPublicIdentity{UserID: "actor", Username: "alice", DisplayName: "Alice"},
		PathID: "path-1", PathName: "Piano", SocialFeedEventID: "practice:activity-1", Reaction: "fire",
	}
}

func practiceCommentNotification(createdAt time.Time) InvitationNotificationProjection {
	return InvitationNotificationProjection{
		ID: "comment-notification", Kind: NotificationPracticeComment,
		Presentation: NotificationInformational, CreatedAt: createdAt,
		Actor:  InvitationPublicIdentity{UserID: "actor", Username: "alice", DisplayName: "Alice"},
		PathID: "path-1", PathName: "Piano", SocialFeedEventID: "practice:activity-1", CommentID: "comment-1",
	}
}

func commentHeartNotification(createdAt time.Time) InvitationNotificationProjection {
	return InvitationNotificationProjection{
		ID: "heart-notification", Kind: NotificationCommentHeart,
		Presentation: NotificationInformational, CreatedAt: createdAt,
		Actor:  InvitationPublicIdentity{UserID: "actor", Username: "alice", DisplayName: "Alice"},
		PathID: "path-1", PathName: "Piano", SocialFeedEventID: "practice:activity-1", CommentID: "comment-1",
	}
}

func nudgeNotification(createdAt time.Time) InvitationNotificationProjection {
	return InvitationNotificationProjection{
		ID: "nudge-notification", Kind: NotificationNudgeReceived,
		Presentation: NotificationInformational, CreatedAt: createdAt,
		Actor:    InvitationPublicIdentity{UserID: "actor", Username: "alice", DisplayName: "Alice"},
		PathID:   "path-1",
		PathName: "Piano",
		NudgeContent: socialdomain.NudgeContent{
			Kind: socialdomain.NudgeContentPreset, Preset: socialdomain.NudgeKeepItGoing,
		},
	}
}

func TestNotificationPageAcceptsOnlyPresetNudgePathSubjects(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	valid := nudgeNotification(now.Add(-time.Second))
	if !validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{valid}}, 25, now) {
		t.Fatal("valid nudge notification rejected")
	}
	for name, mutate := range map[string]func(*InvitationNotificationProjection){
		"actionable":      func(item *InvitationNotificationProjection) { item.Presentation = NotificationActionable },
		"missing path":    func(item *InvitationNotificationProjection) { item.PathID = "" },
		"missing content": func(item *InvitationNotificationProjection) { item.NudgeContent = socialdomain.NudgeContent{} },
		"unknown preset":  func(item *InvitationNotificationProjection) { item.NudgeContent.Preset = "shame" },
		"foreign subject": func(item *InvitationNotificationProjection) { item.CommentID = "comment" },
		"custom kind":     func(item *InvitationNotificationProjection) { item.NudgeContent.Kind = "custom" },
	} {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			if validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}}, 25, now) {
				t.Fatalf("invalid nudge notification accepted: %+v", item)
			}
		})
	}
}

func TestNotificationPageAcceptsDedicatedCommentHeartSubject(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	item := commentHeartNotification(now.Add(-time.Second))
	if !validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}}, 25, now) {
		t.Fatal("valid comment-heart notification rejected")
	}
	item.Reaction = "heart"
	if validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{item}}, 25, now) {
		t.Fatal("comment-heart notification accepted unrelated event reaction")
	}
}

func TestNotificationPageAcceptsOnlyPrivacySafeDisabledInteractionSubjects(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	comment := practiceCommentNotification(now.Add(-time.Second))
	comment.Actor = InvitationPublicIdentity{UserID: "owner", Username: "owner", DisplayName: "Owner"}
	comment.CommentID = ""
	comment.InteractionDisabled = InteractionDisabledComments
	if !validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{comment}}, 1, now) {
		t.Fatal("privacy-safe disabled comment notification rejected")
	}

	reaction := practiceReactionNotification(now.Add(-time.Second))
	reaction.Actor = comment.Actor
	reaction.Reaction = ""
	reaction.InteractionDisabled = InteractionDisabledReactions
	if !validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{reaction}}, 1, now) {
		t.Fatal("privacy-safe disabled reaction notification rejected")
	}

	for name, mutate := range map[string]func(*InvitationNotificationProjection){
		"retained comment": func(item *InvitationNotificationProjection) { item.CommentID = "comment-1" },
		"crossed reason":   func(item *InvitationNotificationProjection) { item.InteractionDisabled = InteractionDisabledReactions },
		"foreign subject":  func(item *InvitationNotificationProjection) { item.FollowRequestID = "request-1" },
	} {
		t.Run(name, func(t *testing.T) {
			invalid := comment
			mutate(&invalid)
			if validNotificationPage(NotificationPage{Items: []InvitationNotificationProjection{invalid}}, 1, now) {
				t.Fatalf("invalid disabled interaction accepted: %+v", invalid)
			}
		})
	}
}

func TestNotificationPageAcceptsBoundedSocialSubjects(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	actor := InvitationPublicIdentity{UserID: "actor", Username: "alice", DisplayName: "Alice"}
	page := NotificationPage{Items: []InvitationNotificationProjection{
		{ID: "newer", Kind: NotificationFollowRequestReceived, Presentation: NotificationActionable, CreatedAt: now.Add(-time.Minute), Actor: actor, FollowRequestID: "request-1"},
		{ID: "older", Kind: NotificationNewFollower, Presentation: NotificationInformational, CreatedAt: now.Add(-time.Hour), Actor: actor},
	}}
	if !validNotificationPage(page, 25, now) {
		t.Fatalf("valid social page rejected: %+v", page)
	}
	page.Items[0].PathID = "invented-path"
	if validNotificationPage(page, 25, now) {
		t.Fatal("social notification accepted invented Path context")
	}
}

func TestNotificationPageAcceptsPathDeletionSubject(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	page := NotificationPage{Items: []InvitationNotificationProjection{{
		ID: "notification-deleted", Kind: NotificationPathDeleted, Presentation: NotificationInformational,
		PathName: "Reading", Actor: InvitationPublicIdentity{UserID: "owner", Username: "owner", DisplayName: "Owner"},
		CreatedAt: now.Add(-time.Minute),
	}}}
	if !validNotificationPage(page, 25, now) {
		t.Fatalf("valid path deletion page rejected: %+v", page)
	}
}

func TestListNotificationsAcceptsOnlyStrictEligiblePracticeReactionSubjects(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	valid := practiceReactionNotification(now.Add(-time.Second))
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{valid}, UnreadCount: 1}}

	items, _, unread, err := NewInvitationService(dependencies).ListNotifications(context.Background(), "Bearer valid", "", 25)
	if err != nil || len(items) != 1 || items[0] != valid || unread != 1 {
		t.Fatalf("reaction notifications=%+v unread=%d err=%v", items, unread, err)
	}

	for name, mutate := range map[string]func(*InvitationNotificationProjection){
		"actionable":          func(item *InvitationNotificationProjection) { item.Presentation = NotificationActionable },
		"missing path":        func(item *InvitationNotificationProjection) { item.PathID = "" },
		"missing event":       func(item *InvitationNotificationProjection) { item.SocialFeedEventID = "" },
		"invalid reaction":    func(item *InvitationNotificationProjection) { item.Reaction = "discourage" },
		"foreign subject":     func(item *InvitationNotificationProjection) { item.FollowRequestID = "request-1" },
		"future notification": func(item *InvitationNotificationProjection) { item.CreatedAt = now.Add(time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			invalid := dependencies
			invalid.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{item}}}
			if _, _, _, err := NewInvitationService(invalid).ListNotifications(context.Background(), "Bearer valid", "", 25); !errors.Is(err, errInvalidInvitationDependencies) {
				t.Fatalf("invalid reaction notification error=%v", err)
			}
		})
	}
}

func TestListNotificationsAcceptsOnlyStrictEligiblePracticeCommentSubjects(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	valid := practiceCommentNotification(now.Add(-time.Second))
	dependencies := invitationDependencies(now)
	dependencies.Auth = controlledAuthenticator{principal: ports.Principal{UserID: "recipient", Scopes: []string{"api:user"}}}
	dependencies.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{valid}, UnreadCount: 1}}

	items, _, unread, err := NewInvitationService(dependencies).ListNotifications(context.Background(), "Bearer valid", "", 25)
	if err != nil || len(items) != 1 || items[0] != valid || unread != 1 {
		t.Fatalf("comment notifications=%+v unread=%d err=%v", items, unread, err)
	}

	for name, mutate := range map[string]func(*InvitationNotificationProjection){
		"actionable":          func(item *InvitationNotificationProjection) { item.Presentation = NotificationActionable },
		"missing path":        func(item *InvitationNotificationProjection) { item.PathID = "" },
		"missing event":       func(item *InvitationNotificationProjection) { item.SocialFeedEventID = "" },
		"missing comment":     func(item *InvitationNotificationProjection) { item.CommentID = "" },
		"reaction subject":    func(item *InvitationNotificationProjection) { item.Reaction = "heart" },
		"foreign subject":     func(item *InvitationNotificationProjection) { item.FollowRequestID = "request-1" },
		"future notification": func(item *InvitationNotificationProjection) { item.CreatedAt = now.Add(time.Second) },
	} {
		t.Run(name, func(t *testing.T) {
			item := valid
			mutate(&item)
			invalid := dependencies
			invalid.Invitations = controlledInvitationRepository{notificationPage: NotificationPage{Items: []InvitationNotificationProjection{item}}}
			if _, _, _, err := NewInvitationService(invalid).ListNotifications(context.Background(), "Bearer valid", "", 25); !errors.Is(err, errInvalidInvitationDependencies) {
				t.Fatalf("invalid comment notification error=%v", err)
			}
		})
	}
}
