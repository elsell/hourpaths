package identity

import (
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

type ProfileVisibility string

const (
	ProfileVisibilityPublic  ProfileVisibility = "public"
	ProfileVisibilityPrivate ProfileVisibility = "private"
)

var errInvalidProfileVisibility = errors.New("profile visibility must be explicitly public or private")

func ValidateProfileVisibility(value ProfileVisibility) error {
	switch value {
	case ProfileVisibilityPublic, ProfileVisibilityPrivate:
		return nil
	default:
		return errInvalidProfileVisibility
	}
}

// FirstDayOfWeek uses ISO-8601 weekday numbers: Monday is 1 and Sunday is 7.
// Zero is intentionally invalid so account activation cannot acquire a default
// merely from Go's zero value.
type FirstDayOfWeek uint8

const (
	FirstDayMonday FirstDayOfWeek = iota + 1
	FirstDayTuesday
	FirstDayWednesday
	FirstDayThursday
	FirstDayFriday
	FirstDaySaturday
	FirstDaySunday
)

var errInvalidFirstDayOfWeek = errors.New("first day of week must be an ISO weekday from 1 through 7")

func ValidateFirstDayOfWeek(value FirstDayOfWeek) error {
	if value < FirstDayMonday || value > FirstDaySunday {
		return errInvalidFirstDayOfWeek
	}
	return nil
}

type IANATimeZone string

var errInvalidIANATimeZone = errors.New("time zone must be a named IANA time-zone identifier")

func ValidateIANATimeZone(value IANATimeZone) error {
	name := string(value)
	if name == "" || name == "Local" {
		return errInvalidIANATimeZone
	}
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("%w: %q", errInvalidIANATimeZone, name)
	}
	return nil
}

// MinimumAgeAttestation records only the required affirmative statement. It
// deliberately cannot represent or retain a birthdate.
type MinimumAgeAttestation string

const MinimumAgeAttestedAtLeast16 MinimumAgeAttestation = "at_least_16"

var errMinimumAgeNotAttested = errors.New("user must attest that they are at least 16")

func ValidateMinimumAgeAttestation(value MinimumAgeAttestation) error {
	if value != MinimumAgeAttestedAtLeast16 {
		return errMinimumAgeNotAttested
	}
	return nil
}

// CurrentPolicyVersions is supplied by validated application configuration.
// The domain deliberately does not choose or embed product policy versions.
type CurrentPolicyVersions struct {
	TermsOfService      string
	PrivacyPolicy       string
	CommunityGuidelines string
}

// PolicyAcceptance preserves the different acts required by the product
// contract: accepting the Terms and Guidelines, and acknowledging Privacy.
type PolicyAcceptance struct {
	TermsOfServiceAcceptedVersion      string
	PrivacyPolicyAcknowledgedVersion   string
	CommunityGuidelinesAcceptedVersion string
	AcceptedAt                         time.Time
}

var (
	errCurrentPolicyVersionMissing = errors.New("every current policy version must be configured")
	errStalePolicyAcceptance       = errors.New("each accepted or acknowledged policy version must exactly match the current configured version")
	errPolicyAcceptanceTimeMissing = errors.New("policy acceptance instant is required")
)

const maxPolicyVersionLength = 128

func validPolicyVersion(value string) bool {
	return utf8.ValidString(value) && strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= maxPolicyVersionLength
}

func ValidatePolicyAcceptance(acceptance PolicyAcceptance, current CurrentPolicyVersions) error {
	if !validPolicyVersion(current.TermsOfService) || !validPolicyVersion(current.PrivacyPolicy) || !validPolicyVersion(current.CommunityGuidelines) {
		return errCurrentPolicyVersionMissing
	}
	if acceptance.TermsOfServiceAcceptedVersion != current.TermsOfService ||
		acceptance.PrivacyPolicyAcknowledgedVersion != current.PrivacyPolicy ||
		acceptance.CommunityGuidelinesAcceptedVersion != current.CommunityGuidelines {
		return errStalePolicyAcceptance
	}
	if acceptance.AcceptedAt.IsZero() {
		return errPolicyAcceptanceTimeMissing
	}
	return nil
}

// OnboardingActivation is the decision-independent state that an atomic
// activation repository must persist together with provisional-to-active user
// transition and credential rotation. DisplayName is carried but intentionally
// not validated here until the product chooses Unicode counting semantics.
type OnboardingActivation struct {
	PolicySetRevision int64
	UserID            string
	Username          string
	DisplayName       string
	ProfileVisibility ProfileVisibility
	TimeZone          IANATimeZone
	FirstDayOfWeek    FirstDayOfWeek
	AgeAttestation    MinimumAgeAttestation
	PolicyAcceptance  PolicyAcceptance
}

var errActivationUserIDMissing = errors.New("activation user ID is required")
var errActivationDisplayNameMissing = errors.New("activation display name is required")
var errActivationPolicyRevisionMissing = errors.New("activation policy revision is required")

func ValidateOnboardingActivation(activation OnboardingActivation, current CurrentPolicyVersions) error {
	if activation.PolicySetRevision <= 0 {
		return errActivationPolicyRevisionMissing
	}
	if strings.TrimSpace(activation.UserID) == "" {
		return errActivationUserIDMissing
	}
	if err := ValidateUsername(activation.Username); err != nil {
		return err
	}
	if strings.TrimSpace(activation.DisplayName) == "" {
		return errActivationDisplayNameMissing
	}
	if err := ValidateProfileVisibility(activation.ProfileVisibility); err != nil {
		return err
	}
	if err := ValidateIANATimeZone(activation.TimeZone); err != nil {
		return err
	}
	if err := ValidateFirstDayOfWeek(activation.FirstDayOfWeek); err != nil {
		return err
	}
	if err := ValidateMinimumAgeAttestation(activation.AgeAttestation); err != nil {
		return err
	}
	return ValidatePolicyAcceptance(activation.PolicyAcceptance, current)
}
