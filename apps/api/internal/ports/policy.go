package ports

import (
	"context"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// PolicySet is the single, revisioned policy authority presented to users and
// retained with their acceptance evidence.
type PolicySet struct {
	Revision                                                       int64
	TermsVersion, PrivacyPolicyVersion, CommunityGuidelinesVersion string
	TermsURL, PrivacyPolicyURL, CommunityGuidelinesURL, SupportURL string
	UpdatedAt                                                      time.Time
}

type PolicyAuthority interface {
	Current(context.Context) (PolicySet, error)
}

type PolicyPublisher interface {
	Publish(context.Context, PolicySet) (PolicySet, error)
}

// ValidatePolicySet enforces the shared storage boundary. Deployment-specific
// configuration may impose stricter transport rules (production requires
// HTTPS), while this boundary also supports explicitly insecure local URLs.
func ValidatePolicySet(value PolicySet) error {
	if value.Revision <= 0 || value.UpdatedAt.IsZero() || !value.UpdatedAt.Equal(value.UpdatedAt.Truncate(time.Microsecond)) {
		return ErrInvalidArgument
	}
	for _, version := range []string{value.TermsVersion, value.PrivacyPolicyVersion, value.CommunityGuidelinesVersion} {
		if !utf8.ValidString(version) || strings.TrimSpace(version) == "" || utf8.RuneCountInString(version) > 128 {
			return ErrInvalidArgument
		}
	}
	for _, location := range []string{value.TermsURL, value.PrivacyPolicyURL, value.CommunityGuidelinesURL, value.SupportURL} {
		if !utf8.ValidString(location) || strings.TrimSpace(location) == "" || utf8.RuneCountInString(location) > 2048 {
			return ErrInvalidArgument
		}
		parsed, err := url.Parse(location)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return ErrInvalidArgument
		}
	}
	return nil
}

func PolicySetsEqual(left, right PolicySet) bool {
	return left.Revision == right.Revision &&
		left.TermsVersion == right.TermsVersion &&
		left.PrivacyPolicyVersion == right.PrivacyPolicyVersion &&
		left.CommunityGuidelinesVersion == right.CommunityGuidelinesVersion &&
		left.TermsURL == right.TermsURL && left.PrivacyPolicyURL == right.PrivacyPolicyURL &&
		left.CommunityGuidelinesURL == right.CommunityGuidelinesURL && left.SupportURL == right.SupportURL &&
		left.UpdatedAt.Equal(right.UpdatedAt)
}
