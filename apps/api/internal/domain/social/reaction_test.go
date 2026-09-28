package social

import "testing"

func TestReactionIsLimitedToCuratedPositiveSet(t *testing.T) {
	want := []Reaction{ReactionHeart, ReactionApplause, ReactionFire, ReactionStrong, ReactionCelebrate}
	got := Reactions()
	if len(got) != len(want) {
		t.Fatalf("reactions=%v", got)
	}
	for index := range want {
		if got[index] != want[index] || !got[index].Valid() {
			t.Fatalf("reactions=%v", got)
		}
	}
	for _, value := range []Reaction{"", "like", "thumbs_down", "Heart", " heart "} {
		if value.Valid() {
			t.Fatalf("unexpected valid reaction %q", value)
		}
	}
}

func TestReactionSummaryValidatesCountsAndViewerSelection(t *testing.T) {
	summary := ReactionSummary{Counts: ReactionCounts{Heart: 2, Fire: 1}, ViewerReaction: ReactionFire}
	if !summary.Valid() {
		t.Fatalf("valid summary rejected: %+v", summary)
	}
	summary.Counts.Applause = -1
	if summary.Valid() {
		t.Fatalf("negative count accepted: %+v", summary)
	}
	summary = ReactionSummary{ViewerReaction: "custom"}
	if summary.Valid() {
		t.Fatalf("custom viewer reaction accepted: %+v", summary)
	}
}

func TestEmojiReactionCanonicalizationAndBounds(t *testing.T) {
	for _, emoji := range []string{"❤️", "👏", "🔥", "💪", "🎉", "🧑🏽‍🚀", "🇪🇸", "1️⃣", "🦊"} {
		reaction, ok := CanonicalEmojiReaction(emoji)
		if !ok || !reaction.ValidStored() {
			t.Fatalf("valid emoji rejected: %q", emoji)
		}
	}
	reaction, ok := CanonicalEmojiReaction("❤️")
	if !ok || reaction != ReactionHeart {
		t.Fatalf("curated alias not canonical: %q", reaction)
	}
	for _, emoji := range []string{"", "text", "🦊🦊", "🏽", "🇪", " heart ", "<script>"} {
		if _, ok := CanonicalEmojiReaction(emoji); ok {
			t.Fatalf("invalid emoji accepted: %q", emoji)
		}
	}
}
