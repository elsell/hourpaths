package social

import (
	"testing"
	"time"
)

func TestNudgePresetCatalogIsExactAndPositiveOnly(t *testing.T) {
	want := []NudgePreset{
		NudgeYouHaveGotThis,
		NudgeLetsGo,
		NudgeLittleProgressCounts,
		NudgeKeepItGoing,
		NudgeTimeToWork,
	}
	got := NudgePresets()
	if len(got) != len(want) {
		t.Fatalf("presets=%v", got)
	}
	for index := range want {
		if got[index] != want[index] || !got[index].Valid() {
			t.Fatalf("presets=%v", got)
		}
	}
	got[0] = "mutated"
	if NudgePresets()[0] != want[0] {
		t.Fatal("NudgePresets exposed mutable catalog storage")
	}
	for _, value := range []NudgePreset{"", "custom", "You’ve got this!", "lets-go", " lets_go "} {
		if value.Valid() {
			t.Fatalf("unexpected valid preset %q", value)
		}
	}
}

func TestNudgeAudienceCatalogDefaultsToPathMembers(t *testing.T) {
	want := []NudgeAudience{NudgeAudienceNobody, NudgeAudiencePathMembers, NudgeAudienceFollowers, NudgeAudienceEveryone}
	got := NudgeAudiences()
	if len(got) != len(want) || DefaultNudgeAudience != NudgeAudiencePathMembers {
		t.Fatalf("audiences=%v default=%q", got, DefaultNudgeAudience)
	}
	for index := range want {
		if got[index] != want[index] || !got[index].Valid() {
			t.Fatalf("audiences=%v", got)
		}
	}
	got[0] = "mutated"
	if NudgeAudiences()[0] != want[0] {
		t.Fatal("NudgeAudiences exposed mutable catalog storage")
	}
	for _, value := range []NudgeAudience{"", "members", "public", "Path_members", " everyone "} {
		if value.Valid() {
			t.Fatalf("unexpected valid audience %q", value)
		}
	}
}

func TestNudgeContentAcceptsOnlyTheInitialPresetKind(t *testing.T) {
	valid := NudgeContent{Kind: NudgeContentPreset, Preset: NudgeLetsGo}
	if !valid.Valid() {
		t.Fatalf("valid content rejected: %+v", valid)
	}
	for _, content := range []NudgeContent{
		{},
		{Kind: NudgeContentPreset},
		{Kind: NudgeContentPreset, Preset: "custom"},
		{Kind: "custom_message", Preset: NudgeLetsGo},
	} {
		if content.Valid() {
			t.Fatalf("unsupported content accepted: %+v", content)
		}
	}
}

func TestNudgeRequiresDistinctPeoplePathAndCanonicalUTCInstant(t *testing.T) {
	now := time.Date(2026, time.August, 5, 13, 0, 0, 0, time.UTC)
	valid := Nudge{
		ID: "nudge-1", SenderID: "sender", RecipientID: "recipient", PathID: "path-1",
		Content: NudgeContent{Kind: NudgeContentPreset, Preset: NudgeKeepItGoing},
		SentAt:  now,
	}
	if !valid.Valid() {
		t.Fatalf("valid nudge rejected: %+v", valid)
	}
	cases := []Nudge{
		{},
		func() Nudge { value := valid; value.SenderID = value.RecipientID; return value }(),
		func() Nudge { value := valid; value.PathID = " "; return value }(),
		func() Nudge { value := valid; value.PathID = " path-1 "; return value }(),
		func() Nudge { value := valid; value.Content.Preset = "custom"; return value }(),
		func() Nudge {
			value := valid
			value.SentAt = value.SentAt.In(time.FixedZone("offset", 3600))
			return value
		}(),
	}
	for _, value := range cases {
		if value.Valid() {
			t.Fatalf("invalid nudge accepted: %+v", value)
		}
	}
}
