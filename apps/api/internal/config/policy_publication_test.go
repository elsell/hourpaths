package config

import "testing"

func TestLoadPolicyPublicationValidatesOnlyExplicitPolicyEnvironment(t *testing.T) {
	for key, value := range map[string]string{
		"HOURPATHS_CURRENT_POLICY_REVISION":              "12",
		"HOURPATHS_CURRENT_TERMS_VERSION":                "terms-v12",
		"HOURPATHS_CURRENT_PRIVACY_POLICY_VERSION":       "privacy-v12",
		"HOURPATHS_CURRENT_COMMUNITY_GUIDELINES_VERSION": "guidelines-v12",
		"HOURPATHS_TERMS_URL":                            "https://app.example/legal/terms",
		"HOURPATHS_PRIVACY_POLICY_URL":                   "https://app.example/legal/privacy",
		"HOURPATHS_COMMUNITY_GUIDELINES_URL":             "https://app.example/community-guidelines",
		"HOURPATHS_SUPPORT_URL":                          "https://app.example/support",
	} {
		t.Setenv(key, value)
	}
	got, err := LoadPolicyPublication(false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Revision != 12 || got.CurrentTermsVersion != "terms-v12" {
		t.Fatalf("policy publication = %#v", got)
	}

	t.Setenv("HOURPATHS_CURRENT_POLICY_REVISION", "not-a-number")
	if _, err := LoadPolicyPublication(false); err == nil {
		t.Fatal("malformed revision accepted")
	}
	t.Setenv("HOURPATHS_CURRENT_POLICY_REVISION", "12")
	t.Setenv("HOURPATHS_TERMS_URL", "http://localhost:5173/legal/terms")
	if _, err := LoadPolicyPublication(false); err == nil {
		t.Fatal("insecure URL accepted without explicit local mode")
	}
	if _, err := LoadPolicyPublication(true); err != nil {
		t.Fatalf("explicit local URL rejected: %v", err)
	}
}
