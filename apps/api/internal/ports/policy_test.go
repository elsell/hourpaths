package ports

import (
	"testing"
	"time"
)

func TestValidatePolicySetRejectsIncompleteOrUnsafeAuthority(t *testing.T) {
	valid := PolicySet{
		Revision:     1,
		TermsVersion: "terms-v1", PrivacyPolicyVersion: "privacy-v1", CommunityGuidelinesVersion: "guidelines-v1",
		TermsURL: "https://app.example/legal/terms", PrivacyPolicyURL: "https://app.example/legal/privacy",
		CommunityGuidelinesURL: "https://app.example/community-guidelines", SupportURL: "https://app.example/support",
		UpdatedAt: time.Date(2026, 7, 21, 12, 0, 0, 0, time.UTC),
	}
	if err := ValidatePolicySet(valid); err != nil {
		t.Fatal(err)
	}
	tests := map[string]func(*PolicySet){
		"zero revision":         func(value *PolicySet) { value.Revision = 0 },
		"blank version":         func(value *PolicySet) { value.TermsVersion = " \t" },
		"oversized version":     func(value *PolicySet) { value.PrivacyPolicyVersion = string(make([]byte, 129)) },
		"invalid version UTF-8": func(value *PolicySet) { value.CommunityGuidelinesVersion = string([]byte{0xff}) },
		"relative URL":          func(value *PolicySet) { value.SupportURL = "/support" },
		"URL credentials":       func(value *PolicySet) { value.TermsURL = "https://user:secret@app.example/terms" },
		"URL query":             func(value *PolicySet) { value.PrivacyPolicyURL = "https://app.example/privacy?current=true" },
		"oversized URL": func(value *PolicySet) {
			value.CommunityGuidelinesURL = "https://app.example/" + string(make([]byte, 2049))
		},
		"missing update time":         func(value *PolicySet) { value.UpdatedAt = time.Time{} },
		"sub-microsecond update time": func(value *PolicySet) { value.UpdatedAt = value.UpdatedAt.Add(time.Nanosecond) },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := ValidatePolicySet(candidate); err == nil {
				t.Fatal("invalid policy authority accepted")
			}
		})
	}
}
