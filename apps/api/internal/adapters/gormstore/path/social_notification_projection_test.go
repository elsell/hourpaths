package pathstore

import (
	"testing"
	"time"

	application "github.com/elsell/hour-paths/apps/api/internal/app/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
)

func TestSocialNotificationProjectionValidatesSubjectsWithoutInventingAPath(t *testing.T) {
	now := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	username := "alice"
	base := invitationNotificationRow{NotificationPersistence: NotificationPersistence{
		ID: "notification-1", RecipientUserID: "recipient", ActorUserID: "actor", FollowSubjectUserID: "actor",
		PresentationClass: "informational", Channel: "following", CreatedAt: now,
	}, ActorID: "actor", ActorUsername: &username, ActorName: "Alice"}
	base.Kind = "new_follower"
	item, err := invitationNotificationFromRow(base, "recipient")
	if err != nil || item.Kind != application.NotificationNewFollower || item.PathID != "" || item.PathName != "" || item.FollowRequestID != "" {
		t.Fatalf("new follower projection=%+v err=%v", item, err)
	}

	requestCreated := now.Add(-time.Minute)
	received := base
	received.Kind, received.PresentationClass, received.FollowRequestID = "follow_request_received", "actionable", "request-1"
	received.FollowRequesterID, received.FollowTargetID, received.FollowRequestCreatedAt = "actor", "recipient", &requestCreated
	item, err = invitationNotificationFromRow(received, "recipient")
	if err != nil || item.Kind != application.NotificationFollowRequestReceived || item.FollowRequestID != "request-1" {
		t.Fatalf("request projection=%+v err=%v", item, err)
	}

	malformed := received
	malformed.Channel = "path_access"
	if _, err := invitationNotificationFromRow(malformed, "recipient"); err == nil {
		t.Fatal("social notification accepted the Path channel")
	}

	reaction := invitationNotificationRow{NotificationPersistence: NotificationPersistence{
		ID: "reaction-notification", RecipientUserID: "recipient", ActorUserID: "actor", PathID: "path",
		SocialFeedEventID: "practice:activity", ReactionType: "fire", Kind: "practice_reaction",
		PresentationClass: "informational", Channel: "reactions", CreatedAt: now,
	}, PathName: "Piano", ActorID: "actor", ActorUsername: &username, ActorName: "Alice"}
	item, err = invitationNotificationFromRow(reaction, "recipient")
	if err != nil || item.Kind != application.NotificationPracticeReaction || item.SocialFeedEventID != "practice:activity" || item.Reaction != "fire" || item.PathName != "Piano" {
		t.Fatalf("reaction projection=%+v err=%v", item, err)
	}
	reason, ownerUsername := "reactions", "owner"
	disabledAt := now.Add(time.Minute)
	disabledReaction := reaction
	disabledReaction.DeletedAt, disabledReaction.InteractionDisabledReason = &disabledAt, &reason
	disabledReaction.EventOwnerID, disabledReaction.EventOwnerUsername, disabledReaction.EventOwnerName = "owner", &ownerUsername, "Owner"
	item, err = invitationNotificationFromRow(disabledReaction, "recipient")
	if err != nil || item.InteractionDisabled != application.InteractionDisabledReactions || item.Reaction != "" || item.CommentID != "" || item.Actor.UserID != "owner" {
		t.Fatalf("disabled reaction projection=%+v err=%v", item, err)
	}
	reaction.RecipientUserID = "actor"
	if _, err := invitationNotificationFromRow(reaction, "actor"); err == nil {
		t.Fatal("self-reaction notification accepted")
	}

	heart := invitationNotificationRow{NotificationPersistence: NotificationPersistence{
		ID: "heart-notification", RecipientUserID: "comment-author", ActorUserID: "actor", PathID: "path",
		SocialFeedEventID: "practice:activity", CommentID: "comment-1", Kind: "comment_heart",
		PresentationClass: "informational", Channel: "comment_hearts", CreatedAt: now,
	}, PathName: "Piano", ActorID: "actor", ActorUsername: &username, ActorName: "Alice", CommentEventID: "practice:activity", CommentAuthorID: "comment-author"}
	item, err = invitationNotificationFromRow(heart, "comment-author")
	if err != nil || item.Kind != application.NotificationCommentHeart || item.CommentID != "comment-1" {
		t.Fatalf("comment heart projection=%+v err=%v", item, err)
	}
	heart.CommentAuthorID = "other"
	if _, err := invitationNotificationFromRow(heart, "comment-author"); err == nil {
		t.Fatal("comment heart projected to someone other than the comment author")
	}
}

func TestNudgeNotificationProjectionRequiresThePersistedPresetSubject(t *testing.T) {
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)
	username := "alice"
	base := invitationNotificationRow{NotificationPersistence: NotificationPersistence{
		ID: "nudge-notification", RecipientUserID: "recipient", ActorUserID: "actor", PathID: "path", NudgeID: "nudge-1",
		Kind: "nudge_received", PresentationClass: "informational", Channel: "nudges", CreatedAt: now,
	}, PathName: "Piano", ActorID: "actor", ActorUsername: &username, ActorName: "Alice",
		NudgeRecordID: "nudge-1", NudgeSenderID: "actor", NudgeRecipientID: "recipient", NudgePathID: "path",
		NudgeContentKind: "preset", NudgePreset: "keep_it_going", NudgeSentAt: &now,
	}
	item, err := invitationNotificationFromRow(base, "recipient")
	if err != nil || item.Kind != application.NotificationNudgeReceived || item.PathID != "path" || item.PathName != "Piano" ||
		item.NudgeContent != (socialdomain.NudgeContent{Kind: socialdomain.NudgeContentPreset, Preset: socialdomain.NudgeKeepItGoing}) {
		t.Fatalf("nudge projection=%+v err=%v", item, err)
	}
	for name, mutate := range map[string]func(*invitationNotificationRow){
		"wrong channel":      func(row *invitationNotificationRow) { row.Channel = "following" },
		"missing record":     func(row *invitationNotificationRow) { row.NudgeRecordID = "" },
		"mismatched record":  func(row *invitationNotificationRow) { row.NudgeRecordID = "other" },
		"wrong recipient":    func(row *invitationNotificationRow) { row.NudgeRecipientID = "other" },
		"wrong actor":        func(row *invitationNotificationRow) { row.NudgeSenderID = "other" },
		"wrong path":         func(row *invitationNotificationRow) { row.NudgePathID = "other" },
		"custom content":     func(row *invitationNotificationRow) { row.NudgeContentKind = "custom" },
		"unknown preset":     func(row *invitationNotificationRow) { row.NudgePreset = "shame" },
		"wrong sent instant": func(row *invitationNotificationRow) { changed := now.Add(-time.Second); row.NudgeSentAt = &changed },
		"foreign subject":    func(row *invitationNotificationRow) { row.CommentID = "comment" },
	} {
		t.Run(name, func(t *testing.T) {
			row := base
			mutate(&row)
			if projected, err := invitationNotificationFromRow(row, "recipient"); err == nil || projected != (application.InvitationNotificationProjection{}) {
				t.Fatalf("invalid nudge projected: %+v err=%v", projected, err)
			}
		})
	}
}
