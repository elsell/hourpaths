package shared

import (
	"bytes"
	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"testing"
)

func TestPolicyRenewalReviewCannotBeSubstitutedForOnboarding(t *testing.T) {
	key := bytes.Repeat([]byte{19}, 32)
	versions := identity.CurrentPolicyVersions{TermsOfService: "terms-v2", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "guidelines-v1"}
	renewal, err := EncodePolicyRenewalReviewToken(key, "account-owner", 8, versions)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodePolicyRenewalReviewToken(key, renewal)
	if err != nil || decoded.Owner != "account-owner" || decoded.PolicyRevision != 8 || decoded.Policies != versions {
		t.Fatalf("review lost binding: %+v %v", decoded, err)
	}
	if _, err := DecodePolicyReviewToken(key, renewal); err == nil {
		t.Fatal("active-account review accepted by onboarding")
	}
	onboarding, err := EncodePolicyReviewToken(key, "account-owner", 8, versions)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePolicyRenewalReviewToken(key, onboarding); err == nil {
		t.Fatal("onboarding review accepted for active-account renewal")
	}
	if _, err := DecodePolicyRenewalReviewToken(bytes.Repeat([]byte{20}, 32), renewal); err == nil {
		t.Fatal("foreign signature accepted")
	}
}
