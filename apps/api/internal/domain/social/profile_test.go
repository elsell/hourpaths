package social

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeProfileQueryTrimsNormalizesAndRequiresTwoCharacters(t *testing.T) {
	got, err := NormalizeProfileQuery("  E\u0301L  ")
	if err != nil || got != "él" {
		t.Fatalf("NormalizeProfileQuery() = %q, %v; want NFC lowercase query", got, err)
	}
	for _, query := range []string{"", "  ", "a", " \u0301 "} {
		if _, err := NormalizeProfileQuery(query); err == nil {
			t.Fatalf("NormalizeProfileQuery(%q) accepted fewer than two non-whitespace characters", query)
		}
	}
}

func TestNormalizeUsernameUsesTheCanonicalProfileRules(t *testing.T) {
	got, err := NormalizeUsername("  Alice.ONE_2  ")
	if err != nil || got != "alice.one_2" {
		t.Fatalf("NormalizeUsername() = %q, %v", got, err)
	}
	for _, username := range []string{"", "ab", strings.Repeat("a", 65), "alice smith", "álîce", "alice-1"} {
		if _, err := NormalizeUsername(username); err == nil {
			t.Fatalf("invalid username accepted: %q", username)
		}
	}
}

func TestPublicProfileRejectsPrivateOrIncompleteIdentity(t *testing.T) {
	valid := PublicProfile{ID: "user-1", Username: "alice", DisplayName: "Alice", FollowerCount: 2, FollowingCount: 3, Relationship: RelationshipNone}
	if !valid.Valid() {
		t.Fatal("valid public profile rejected")
	}
	for _, mutate := range []func(*PublicProfile){
		func(profile *PublicProfile) { profile.ID = "" },
		func(profile *PublicProfile) { profile.Username = "" },
		func(profile *PublicProfile) { profile.Username = "alice smith" },
		func(profile *PublicProfile) { profile.DisplayName = "" },
		func(profile *PublicProfile) { profile.DisplayName = strings.Repeat("x", 101) },
		func(profile *PublicProfile) { profile.Description = strings.Repeat("x", 501) },
		func(profile *PublicProfile) { profile.Description = "  " },
		func(profile *PublicProfile) { profile.FollowerCount = -1 },
		func(profile *PublicProfile) { profile.FollowingCount = -1 },
		func(profile *PublicProfile) { profile.Relationship = "secret" },
	} {
		profile := valid
		mutate(&profile)
		if profile.Valid() {
			t.Fatalf("invalid public profile accepted: %+v", profile)
		}
	}
}

func TestViewerRelationshipStatesAreExplicitAndClosed(t *testing.T) {
	for _, state := range []RelationshipState{RelationshipSelf, RelationshipNone, RelationshipRequested, RelationshipFollowing} {
		if !state.Valid() {
			t.Fatalf("valid relationship state rejected: %q", state)
		}
	}
	for _, state := range []RelationshipState{"", "follower", "blocked"} {
		if state.Valid() {
			t.Fatalf("unknown relationship state accepted: %q", state)
		}
	}
}

func TestIncomingFollowRequestRequiresSafeDistinctPendingIdentity(t *testing.T) {
	now := time.Date(2026, 7, 27, 18, 0, 0, 0, time.UTC)
	valid := FollowRequest{
		ID: "request-1", RecipientUserID: "recipient", CreatedAt: now,
		Requester: PublicProfile{ID: "sender", Username: "alice", DisplayName: "Alice", Relationship: RelationshipNone},
	}
	if !valid.Valid() {
		t.Fatal("valid incoming follow request rejected")
	}
	for _, mutate := range []func(*FollowRequest){
		func(request *FollowRequest) { request.ID = "" },
		func(request *FollowRequest) { request.RecipientUserID = "" },
		func(request *FollowRequest) { request.Requester.ID = request.RecipientUserID },
		func(request *FollowRequest) { request.Requester.Relationship = RelationshipSelf },
		func(request *FollowRequest) { request.CreatedAt = time.Time{} },
		func(request *FollowRequest) {
			request.CreatedAt = request.CreatedAt.In(time.FixedZone("not-utc", 3600))
		},
	} {
		request := valid
		mutate(&request)
		if request.Valid() {
			t.Fatalf("invalid incoming follow request accepted: %+v", request)
		}
	}
}
