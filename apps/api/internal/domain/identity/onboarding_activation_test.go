package identity

import (
	"strings"
	"testing"
	"time"
)

func TestValidateProfileVisibilityRequiresAnExplicitSupportedValue(t *testing.T) {
	for _, visibility := range []ProfileVisibility{ProfileVisibilityPublic, ProfileVisibilityPrivate} {
		if err := ValidateProfileVisibility(visibility); err != nil {
			t.Fatalf("ValidateProfileVisibility(%q) error = %v", visibility, err)
		}
	}

	for _, visibility := range []ProfileVisibility{"", "followers", "PUBLIC"} {
		if err := ValidateProfileVisibility(visibility); err == nil {
			t.Fatalf("ValidateProfileVisibility(%q) accepted an unsupported value", visibility)
		}
	}
}

func TestValidateFirstDayOfWeekUsesISOWeekdayNumbers(t *testing.T) {
	for day := FirstDayOfWeek(1); day <= 7; day++ {
		if err := ValidateFirstDayOfWeek(day); err != nil {
			t.Fatalf("ValidateFirstDayOfWeek(%d) error = %v", day, err)
		}
	}

	for _, day := range []FirstDayOfWeek{0, 8, 255} {
		if err := ValidateFirstDayOfWeek(day); err == nil {
			t.Fatalf("ValidateFirstDayOfWeek(%d) accepted a non-ISO weekday", day)
		}
	}
}

func TestValidateIANATimeZoneUsesNamedZoneDatabaseIdentifiers(t *testing.T) {
	for _, zone := range []IANATimeZone{"America/New_York", "Europe/London", "Etc/UTC"} {
		if err := ValidateIANATimeZone(zone); err != nil {
			t.Fatalf("ValidateIANATimeZone(%q) error = %v", zone, err)
		}
	}

	for _, zone := range []IANATimeZone{"", "Local", "UTC-04:00", "+02:00", "Mars/Olympus_Mons"} {
		if err := ValidateIANATimeZone(zone); err == nil {
			t.Fatalf("ValidateIANATimeZone(%q) accepted a non-IANA zone", zone)
		}
	}
}

func TestValidateMinimumAgeAttestationRequiresAffirmativeAtLeast16Attestation(t *testing.T) {
	if err := ValidateMinimumAgeAttestation(MinimumAgeAttestedAtLeast16); err != nil {
		t.Fatalf("affirmative minimum-age attestation error = %v", err)
	}

	for _, attestation := range []MinimumAgeAttestation{"", "under_16", "true"} {
		if err := ValidateMinimumAgeAttestation(attestation); err == nil {
			t.Fatalf("ValidateMinimumAgeAttestation(%q) accepted a non-affirmative value", attestation)
		}
	}
}

func TestValidatePolicyAcceptanceRequiresEachConfiguredCurrentVersion(t *testing.T) {
	current := CurrentPolicyVersions{
		TermsOfService:      "terms-current",
		PrivacyPolicy:       "privacy-current",
		CommunityGuidelines: "guidelines-current",
	}
	acceptedAt := time.Date(2026, time.July, 21, 17, 30, 0, 0, time.UTC)
	valid := PolicyAcceptance{
		TermsOfServiceAcceptedVersion:      current.TermsOfService,
		PrivacyPolicyAcknowledgedVersion:   current.PrivacyPolicy,
		CommunityGuidelinesAcceptedVersion: current.CommunityGuidelines,
		AcceptedAt:                         acceptedAt,
	}
	if err := ValidatePolicyAcceptance(valid, current); err != nil {
		t.Fatalf("current policy acceptance error = %v", err)
	}

	tests := map[string]struct {
		acceptance PolicyAcceptance
		current    CurrentPolicyVersions
	}{
		"terms acceptance differs": {
			acceptance: withPolicyAcceptance(valid, func(value *PolicyAcceptance) { value.TermsOfServiceAcceptedVersion = "terms-old" }),
			current:    current,
		},
		"privacy acknowledgement differs": {
			acceptance: withPolicyAcceptance(valid, func(value *PolicyAcceptance) { value.PrivacyPolicyAcknowledgedVersion = "privacy-old" }),
			current:    current,
		},
		"guidelines acceptance differs": {
			acceptance: withPolicyAcceptance(valid, func(value *PolicyAcceptance) { value.CommunityGuidelinesAcceptedVersion = "guidelines-old" }),
			current:    current,
		},
		"acceptance instant absent": {
			acceptance: withPolicyAcceptance(valid, func(value *PolicyAcceptance) { value.AcceptedAt = time.Time{} }),
			current:    current,
		},
		"configured terms version absent": {
			acceptance: valid,
			current:    withCurrentPolicyVersions(current, func(value *CurrentPolicyVersions) { value.TermsOfService = "" }),
		},
		"configured terms version blank": {
			acceptance: withPolicyAcceptance(valid, func(value *PolicyAcceptance) { value.TermsOfServiceAcceptedVersion = " " }),
			current:    withCurrentPolicyVersions(current, func(value *CurrentPolicyVersions) { value.TermsOfService = " " }),
		},
		"configured privacy version absent": {
			acceptance: valid,
			current:    withCurrentPolicyVersions(current, func(value *CurrentPolicyVersions) { value.PrivacyPolicy = "" }),
		},
		"configured guidelines version absent": {
			acceptance: valid,
			current:    withCurrentPolicyVersions(current, func(value *CurrentPolicyVersions) { value.CommunityGuidelines = "" }),
		},
		"configured terms version too long": {
			acceptance: withPolicyAcceptance(valid, func(value *PolicyAcceptance) { value.TermsOfServiceAcceptedVersion = strings.Repeat("v", 129) }),
			current:    withCurrentPolicyVersions(current, func(value *CurrentPolicyVersions) { value.TermsOfService = strings.Repeat("v", 129) }),
		},
		"configured terms version is invalid UTF-8": {
			acceptance: withPolicyAcceptance(valid, func(value *PolicyAcceptance) { value.TermsOfServiceAcceptedVersion = string([]byte{0xff}) }),
			current:    withCurrentPolicyVersions(current, func(value *CurrentPolicyVersions) { value.TermsOfService = string([]byte{0xff}) }),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidatePolicyAcceptance(test.acceptance, test.current); err == nil {
				t.Fatal("ValidatePolicyAcceptance accepted incomplete or stale policy evidence")
			}
		})
	}
}

func TestValidateOnboardingActivationRequiresEveryDecisionIndependentInvariant(t *testing.T) {
	current := CurrentPolicyVersions{
		TermsOfService:      "terms-v1",
		PrivacyPolicy:       "privacy-v1",
		CommunityGuidelines: "guidelines-v1",
	}
	valid := OnboardingActivation{
		PolicySetRevision: 7,
		UserID:            "user-123",
		Username:          "Reviewed.User",
		DisplayName:       "A reviewed display name",
		ProfileVisibility: ProfileVisibilityPrivate,
		TimeZone:          "America/New_York",
		FirstDayOfWeek:    1,
		AgeAttestation:    MinimumAgeAttestedAtLeast16,
		PolicyAcceptance: PolicyAcceptance{
			TermsOfServiceAcceptedVersion:      current.TermsOfService,
			PrivacyPolicyAcknowledgedVersion:   current.PrivacyPolicy,
			CommunityGuidelinesAcceptedVersion: current.CommunityGuidelines,
			AcceptedAt:                         time.Date(2026, time.July, 21, 17, 30, 0, 0, time.UTC),
		},
	}
	if err := ValidateOnboardingActivation(valid, current); err != nil {
		t.Fatalf("valid onboarding activation error = %v", err)
	}

	tests := map[string]func(*OnboardingActivation){
		"policy revision absent": func(value *OnboardingActivation) { value.PolicySetRevision = 0 },
		"user identity absent":   func(value *OnboardingActivation) { value.UserID = "" },
		"username invalid":       func(value *OnboardingActivation) { value.Username = "no spaces" },
		"display name absent":    func(value *OnboardingActivation) { value.DisplayName = " \t" },
		"visibility absent":      func(value *OnboardingActivation) { value.ProfileVisibility = "" },
		"time zone invalid":      func(value *OnboardingActivation) { value.TimeZone = "UTC-04:00" },
		"first day absent":       func(value *OnboardingActivation) { value.FirstDayOfWeek = 0 },
		"age not attested":       func(value *OnboardingActivation) { value.AgeAttestation = "" },
		"policy evidence stale":  func(value *OnboardingActivation) { value.PolicyAcceptance.TermsOfServiceAcceptedVersion = "terms-old" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			candidate := valid
			mutate(&candidate)
			if err := ValidateOnboardingActivation(candidate, current); err == nil {
				t.Fatal("ValidateOnboardingActivation accepted an invalid aggregate")
			}
		})
	}
}

func withPolicyAcceptance(value PolicyAcceptance, mutate func(*PolicyAcceptance)) PolicyAcceptance {
	mutate(&value)
	return value
}

func withCurrentPolicyVersions(value CurrentPolicyVersions, mutate func(*CurrentPolicyVersions)) CurrentPolicyVersions {
	mutate(&value)
	return value
}
