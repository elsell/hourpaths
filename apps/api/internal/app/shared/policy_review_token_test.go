package shared

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

func TestPolicyReviewTokenRoundTripsOnlyTheServerIssuedOwnerAndPolicySet(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	want := PolicyReviewPayload{
		Version: 1, Owner: "provisional-owner", PolicyRevision: 7,
		Policies: identity.CurrentPolicyVersions{TermsOfService: "terms-v1", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "guidelines-v1"},
	}
	token, err := EncodePolicyReviewToken(key, want.Owner, want.PolicyRevision, want.Policies)
	if err != nil {
		t.Fatalf("encode policy review token: %v", err)
	}
	got, err := DecodePolicyReviewToken(key, token)
	if err != nil {
		t.Fatalf("decode policy review token: %v", err)
	}
	if got != want {
		t.Fatalf("policy review payload = %+v, want %+v", got, want)
	}
}

func TestPolicyReviewTokenRejectsMissingWeakMalformedAndForgedEvidence(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	policies := identity.CurrentPolicyVersions{TermsOfService: "terms-v1", PrivacyPolicy: "privacy-v1", CommunityGuidelines: "guidelines-v1"}
	token, err := EncodePolicyReviewToken(key, "provisional-owner", 7, policies)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		key   []byte
		token string
	}{
		{name: "missing", key: key},
		{name: "malformed", key: key, token: "not-a-token"},
		{name: "forged", key: key, token: token[:len(token)-1] + alternateBase64Byte(token[len(token)-1])},
		{name: "wrong key", key: []byte("abcdef0123456789abcdef0123456789"), token: token},
		{name: "weak key", key: []byte("too-short"), token: token},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := DecodePolicyReviewToken(test.key, test.token); !errors.Is(err, ports.ErrInvalidArgument) {
				t.Fatalf("decode error = %v, want invalid argument", err)
			}
		})
	}
	if _, err := EncodePolicyReviewToken(key, "", 7, policies); err == nil {
		t.Fatal("encoded a token without an owner")
	}
	if _, err := EncodePolicyReviewToken(key, "provisional-owner", 7, identity.CurrentPolicyVersions{}); err == nil {
		t.Fatal("encoded a token without a complete policy set")
	}
	if _, err := EncodePolicyReviewToken([]byte("too-short"), "provisional-owner", 7, policies); err == nil {
		t.Fatal("encoded a token with a weak key")
	}
	if _, err := EncodePolicyReviewToken(key, "provisional-owner", 0, policies); err == nil {
		t.Fatal("encoded a token without a positive shared policy revision")
	}
}

func TestPolicyReviewTokenRejectsSignedButNonCanonicalOrUnexpectedShapes(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	for _, body := range []string{
		`{"v":1,"purpose":"onboarding-policy-review","owner":"provisional-owner","policyRevision":7,"terms":"terms-v1","privacy":"privacy-v1","guidelines":"guidelines-v1","extra":true}`,
		`{"v":1,"v":1,"purpose":"onboarding-policy-review","owner":"provisional-owner","terms":"terms-v1","privacy":"privacy-v1","guidelines":"guidelines-v1"}`,
		`{"v":1, "purpose":"onboarding-policy-review","owner":"provisional-owner","terms":"terms-v1","privacy":"privacy-v1","guidelines":"guidelines-v1"}`,
		`{"v":2,"purpose":"onboarding-policy-review","owner":"provisional-owner","terms":"terms-v1","privacy":"privacy-v1","guidelines":"guidelines-v1"}`,
		`{"v":1,"purpose":"another-purpose","owner":"provisional-owner","terms":"terms-v1","privacy":"privacy-v1","guidelines":"guidelines-v1"}`,
	} {
		if _, err := DecodePolicyReviewToken(key, signPolicyReviewBody(key, []byte(body))); !errors.Is(err, ports.ErrInvalidArgument) {
			t.Fatalf("decoded unexpected signed shape %s: %v", body, err)
		}
	}
}

func signPolicyReviewBody(key, body []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(append(body, mac.Sum(nil)...))
}

func alternateBase64Byte(value byte) string {
	if value == 'A' {
		return "B"
	}
	return "A"
}
