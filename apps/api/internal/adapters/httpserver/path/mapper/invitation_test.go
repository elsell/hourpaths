package mapper

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	pathapp "github.com/elsell/hour-paths/apps/api/internal/app/path"
	domain "github.com/elsell/hour-paths/apps/api/internal/domain/path"
	socialdomain "github.com/elsell/hour-paths/apps/api/internal/domain/social"
)

func TestInvitationMapsPendingAndAcceptedLifecycleWithoutInventingExpiry(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	pending := domain.Invitation{
		ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
		RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now,
	}
	pendingDTO := Invitation(pending)
	if pendingDTO.ID != "invitation-1" || pendingDTO.PathID != "path-1" ||
		pendingDTO.InviterUserID != "creator" || pendingDTO.RecipientUserID != "recipient" ||
		pendingDTO.OfferedRole != "participant" || pendingDTO.CreatedAt != now ||
		pendingDTO.AcceptedAt != nil {
		t.Fatalf("pending invitation DTO = %+v", pendingDTO)
	}

	accepted := pending
	accepted.AcceptedAt = now.Add(time.Minute)
	acceptedDTO := Invitation(accepted)
	if acceptedDTO.AcceptedAt == nil || !acceptedDTO.AcceptedAt.Equal(accepted.AcceptedAt) {
		t.Fatalf("accepted invitation DTO = %+v", acceptedDTO)
	}
}

func TestPendingInvitationMapsOnlyPathAndAlwaysPublicInviterContext(t *testing.T) {
	now := time.Date(2026, time.July, 23, 12, 0, 0, 0, time.UTC)
	value := pathapp.PendingInvitation{
		Invitation: domain.Invitation{
			ID: "invitation-1", PathID: "path-1", InviterUserID: "creator",
			RecipientUserID: "recipient", OfferedRole: domain.RoleParticipant, CreatedAt: now,
		},
		PathName: "Morning Reading",
		Inviter: pathapp.InvitationPublicIdentity{
			UserID: "creator", Username: "Book.Owner", DisplayName: "Book Owner",
		},
		Warning: &pathapp.InvitationWarningContext{
			PathVisibility: "followers", HasRetainedActivity: true,
		},
	}
	mapped := PendingInvitation(value)
	if mapped.Invitation.ID != "invitation-1" || mapped.PathName != "Morning Reading" ||
		mapped.Inviter.UserID != "creator" || mapped.Inviter.Username != "Book.Owner" ||
		mapped.Inviter.DisplayName != "Book Owner" ||
		mapped.Warning == nil || mapped.Warning.PathVisibility != "followers" ||
		!mapped.Warning.HasRetainedActivity {
		t.Fatalf("pending invitation DTO = %+v", mapped)
	}
}

func TestOwnershipTransferNotificationMapsExclusiveTransferContext(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{
		ID: "notice-1", Kind: pathapp.NotificationPathOwnershipTransferReceived,
		Presentation: pathapp.NotificationActionable, CreatedAt: time.Now().UTC(),
		Actor:  pathapp.InvitationPublicIdentity{UserID: "creator", Username: "owner", DisplayName: "Owner"},
		PathID: "path-1", PathName: "Practice", OwnershipTransferID: "transfer-1",
	})
	if mapped.Type != "path_ownership_transfer_received" || mapped.OwnershipTransferID != "transfer-1" ||
		mapped.InvitationID != "" || mapped.OfferedRole != "" {
		t.Fatalf("ownership transfer notification DTO = %+v", mapped)
	}
}

func TestPathDeletionNotificationMapsStandaloneSnapshotWithoutTargetIdentifiers(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{
		ID: "notice-1", Kind: pathapp.NotificationPathDeleted,
		Presentation: pathapp.NotificationInformational, CreatedAt: time.Now().UTC(),
		Actor:  pathapp.InvitationPublicIdentity{UserID: "creator", Username: "owner", DisplayName: "Owner"},
		PathID: "stale-path", PathName: "Former Practice",
		InvitationID: "stale-invitation", OwnershipTransferID: "stale-transfer",
	})
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Type != "path_deleted" || mapped.Presentation != "informational" ||
		mapped.PathName != "Former Practice" || mapped.Actor.Username != "owner" {
		t.Fatalf("path deletion notification DTO = %+v", mapped)
	}
	for _, forbidden := range []string{`"pathId"`, `"invitationId"`, `"ownershipTransferId"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("path deletion notification leaked target identifier %s: %s", forbidden, encoded)
		}
	}
}

func TestSocialNotificationMapsOnlyPublicActorAndFollowRequestContext(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{
		ID: "notice-social", Kind: pathapp.NotificationFollowRequestReceived,
		Presentation: pathapp.NotificationActionable, CreatedAt: time.Now().UTC(),
		Actor:           pathapp.InvitationPublicIdentity{UserID: "follower", Username: "reader", DisplayName: "Reader"},
		FollowRequestID: "request-1",
	})
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Type != "follow_request_received" || mapped.FollowRequestID != "request-1" || mapped.Actor.Username != "reader" {
		t.Fatalf("social notification DTO = %+v", mapped)
	}
	for _, forbidden := range []string{`"pathId"`, `"pathName"`, `"invitationId"`, `"ownershipTransferId"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("social notification leaked unrelated context %s: %s", forbidden, encoded)
		}
	}
}

func TestPracticeReactionNotificationMapsOnlyPathEventAndCuratedReactionContext(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{
		ID: "notice-reaction", Kind: pathapp.NotificationPracticeReaction,
		Presentation: pathapp.NotificationInformational, CreatedAt: time.Now().UTC(),
		Actor:               pathapp.InvitationPublicIdentity{UserID: "reactor", Username: "reader", DisplayName: "Reader"},
		PathID:              "path-1",
		PathName:            "Piano",
		SocialFeedEventID:   "practice:activity-1",
		Reaction:            socialdomain.ReactionFire,
		InvitationID:        "stale-invitation",
		OwnershipTransferID: "stale-transfer",
		FollowRequestID:     "stale-follow-request",
		OfferedRole:         domain.RoleSupporter,
	})
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Type != "practice_reaction" || mapped.PathID != "path-1" || mapped.PathName != "Piano" ||
		mapped.SocialFeedEventID != "practice:activity-1" || mapped.Reaction != "fire" || mapped.Actor.UserID != "reactor" {
		t.Fatalf("practice reaction notification DTO = %+v", mapped)
	}
	for _, forbidden := range []string{`"invitationId"`, `"ownershipTransferId"`, `"followRequestId"`, `"offeredRole"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("reaction notification leaked unrelated context %s: %s", forbidden, encoded)
		}
	}
}

func TestPracticeCommentNotificationMapsOnlyPathEventAndCommentContext(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{
		ID: "notice-comment", Kind: pathapp.NotificationPracticeComment,
		Presentation: pathapp.NotificationInformational, CreatedAt: time.Now().UTC(),
		Actor:               pathapp.InvitationPublicIdentity{UserID: "commenter", Username: "reader", DisplayName: "Reader"},
		PathID:              "path-1",
		PathName:            "Piano",
		SocialFeedEventID:   "practice:activity-1",
		CommentID:           "comment-1",
		InvitationID:        "stale-invitation",
		OwnershipTransferID: "stale-transfer",
		FollowRequestID:     "stale-follow-request",
		OfferedRole:         domain.RoleSupporter,
	})
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Type != "practice_comment" || mapped.PathID != "path-1" || mapped.PathName != "Piano" ||
		mapped.SocialFeedEventID != "practice:activity-1" || mapped.CommentID != "comment-1" || mapped.Actor.UserID != "commenter" {
		t.Fatalf("practice comment notification DTO = %+v", mapped)
	}
	for _, forbidden := range []string{`"invitationId"`, `"ownershipTransferId"`, `"followRequestId"`, `"offeredRole"`, `"reaction"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("comment notification leaked unrelated context %s: %s", forbidden, encoded)
		}
	}
}

func TestCommentHeartNotificationMapsToFocusedCommentContext(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{
		ID: "notice-heart", Kind: pathapp.NotificationCommentHeart, Presentation: pathapp.NotificationInformational, CreatedAt: time.Now().UTC(),
		Actor:  pathapp.InvitationPublicIdentity{UserID: "hearter", Username: "reader", DisplayName: "Reader"},
		PathID: "path-1", PathName: "Piano", SocialFeedEventID: "practice:activity-1", CommentID: "comment-1",
	})
	if mapped.Type != "comment_heart" || mapped.SocialFeedEventID != "practice:activity-1" || mapped.CommentID != "comment-1" {
		t.Fatalf("comment heart notification DTO=%+v", mapped)
	}
}

func TestNudgeNotificationMapsToPathAndPresetContentOnly(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{
		ID: "notice-nudge", Kind: pathapp.NotificationNudgeReceived,
		Presentation: pathapp.NotificationInformational, CreatedAt: time.Now().UTC(),
		Actor:        pathapp.InvitationPublicIdentity{UserID: "sender", Username: "reader", DisplayName: "Reader"},
		PathID:       "path-1",
		PathName:     "Piano",
		NudgeContent: socialdomain.NudgeContent{Kind: socialdomain.NudgeContentPreset, Preset: socialdomain.NudgeKeepItGoing},
		InvitationID: "stale-invitation", FollowRequestID: "stale-follow", CommentID: "stale-comment",
	})
	encoded, err := json.Marshal(mapped)
	if err != nil {
		t.Fatal(err)
	}
	if mapped.Type != "nudge_received" || mapped.PathID != "path-1" || mapped.PathName != "Piano" || mapped.Content == nil ||
		mapped.Content.Kind != "preset" || mapped.Content.Preset != "keep_it_going" {
		t.Fatalf("nudge notification DTO=%+v", mapped)
	}
	for _, forbidden := range []string{`"invitationId"`, `"followRequestId"`, `"commentId"`, `"nudgeId"`, `"message"`, `"text"`} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("nudge notification leaked %s: %s", forbidden, encoded)
		}
	}
}

func TestVisibilityNotificationMapsPathAndNewVisibility(t *testing.T) {
	mapped := InvitationNotification(pathapp.InvitationNotificationProjection{ID: "notice", Kind: pathapp.NotificationPathVisibilityChanged, Presentation: pathapp.NotificationInformational, PathID: "path-1", PathName: "Read", PathVisibility: "followers"})
	if mapped.Type != "path_visibility_changed" || mapped.PathID != "path-1" || mapped.PathName != "Read" || mapped.PathVisibility != "followers" || mapped.InvitationID != "" || mapped.OfferedRole != "" {
		t.Fatalf("mapped=%+v", mapped)
	}
}
