package shared

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/elsell/hour-paths/apps/api/internal/domain/identity"
	"github.com/elsell/hour-paths/apps/api/internal/ports"
)

const (
	policyReviewPurpose       = "onboarding-policy-review"
	maxPolicyReviewTokenBytes = 4096
)

// PolicyReviewPayload is server-issued evidence that an authenticated
// provisional owner was shown one exact set of policies.
type PolicyReviewPayload struct {
	Version        int
	Owner          string
	PolicyRevision int64
	Policies       identity.CurrentPolicyVersions
}

type policyReviewClaims struct {
	Version             int    `json:"v"`
	Purpose             string `json:"purpose"`
	Owner               string `json:"owner"`
	PolicyRevision      int64  `json:"policyRevision"`
	TermsOfService      string `json:"terms"`
	PrivacyPolicy       string `json:"privacy"`
	CommunityGuidelines string `json:"guidelines"`
}

func EncodePolicyReviewToken(key []byte, owner string, policyRevision int64, policies identity.CurrentPolicyVersions) (string, error) {
	if len(key) < 32 {
		return "", errors.New("policy review signing key is required")
	}
	if !validPolicyReviewOwner(owner) || policyRevision <= 0 || !validPolicyReviewSet(policies) {
		return "", ports.ErrInvalidArgument
	}
	claims := policyReviewClaims{
		Version: 1, Purpose: policyReviewPurpose, Owner: owner, PolicyRevision: policyRevision,
		TermsOfService: policies.TermsOfService, PrivacyPolicy: policies.PrivacyPolicy, CommunityGuidelines: policies.CommunityGuidelines,
	}
	body, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	return base64.RawURLEncoding.EncodeToString(append(body, mac.Sum(nil)...)), nil
}

func DecodePolicyReviewToken(key []byte, token string) (PolicyReviewPayload, error) {
	if len(key) < 32 || token == "" || len(token) > maxPolicyReviewTokenBytes {
		return PolicyReviewPayload{}, ports.ErrInvalidArgument
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) <= sha256.Size {
		return PolicyReviewPayload{}, ports.ErrInvalidArgument
	}
	body, signature := raw[:len(raw)-sha256.Size], raw[len(raw)-sha256.Size:]
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(body)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return PolicyReviewPayload{}, ports.ErrInvalidArgument
	}
	var claims policyReviewClaims
	if json.Unmarshal(body, &claims) != nil {
		return PolicyReviewPayload{}, ports.ErrInvalidArgument
	}
	canonical, err := json.Marshal(claims)
	if err != nil || !hmac.Equal(body, canonical) {
		return PolicyReviewPayload{}, ports.ErrInvalidArgument
	}
	policies := identity.CurrentPolicyVersions{
		TermsOfService: claims.TermsOfService, PrivacyPolicy: claims.PrivacyPolicy, CommunityGuidelines: claims.CommunityGuidelines,
	}
	if claims.Version != 1 || claims.Purpose != policyReviewPurpose || !validPolicyReviewOwner(claims.Owner) || claims.PolicyRevision <= 0 || !validPolicyReviewSet(policies) {
		return PolicyReviewPayload{}, ports.ErrInvalidArgument
	}
	return PolicyReviewPayload{Version: claims.Version, Owner: claims.Owner, PolicyRevision: claims.PolicyRevision, Policies: policies}, nil
}

func validPolicyReviewOwner(owner string) bool {
	return utf8.ValidString(owner) && strings.TrimSpace(owner) != "" && len(owner) <= 512
}

func validPolicyReviewSet(policies identity.CurrentPolicyVersions) bool {
	return identity.ValidatePolicyAcceptance(identity.PolicyAcceptance{
		TermsOfServiceAcceptedVersion:      policies.TermsOfService,
		PrivacyPolicyAcknowledgedVersion:   policies.PrivacyPolicy,
		CommunityGuidelinesAcceptedVersion: policies.CommunityGuidelines,
		AcceptedAt:                         time.Unix(1, 0).UTC(),
	}, policies) == nil
}
