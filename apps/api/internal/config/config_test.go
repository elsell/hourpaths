package config

import (
	"math"
	"strconv"
	"testing"
	"time"
)

func TestOwnershipTransferExpirationUsesTypedDocumentedDefault(t *testing.T) {
	t.Setenv("HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES", "")
	duration, err := ownershipTransferExpiration()
	if err != nil {
		t.Fatal(err)
	}
	if duration != 10080*time.Minute {
		t.Fatalf("default duration = %s, want 10080 minutes", duration)
	}
}

func TestOwnershipTransferExpirationAcceptsPositiveWholeSafeMinutes(t *testing.T) {
	for _, minutes := range []int64{90, math.MaxInt64 / int64(time.Minute)} {
		t.Run(strconv.FormatInt(minutes, 10), func(t *testing.T) {
			t.Setenv("HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES", strconv.FormatInt(minutes, 10))
			duration, err := ownershipTransferExpiration()
			if err != nil {
				t.Fatal(err)
			}
			if duration != time.Duration(minutes)*time.Minute {
				t.Fatalf("duration = %s, want %d minutes", duration, minutes)
			}
		})
	}
}

func TestOwnershipTransferExpirationRejectsUnsafeValues(t *testing.T) {
	unsafeMinutes := strconv.FormatInt(math.MaxInt64/int64(time.Minute)+1, 10)
	for _, raw := range []string{"malformed", "1.5", "0", "-1", unsafeMinutes} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES", raw)
			if _, err := ownershipTransferExpiration(); err == nil {
				t.Fatalf("unsafe lifetime %q accepted", raw)
			}
		})
	}
}

func TestHTTPAddressIsValidatedBeforeBootstrap(t *testing.T) {
	for _, address := range []string{"missing-port", "localhost:not-a-port", "localhost:70000"} {
		if err := validateHTTPAddress(address); err == nil {
			t.Fatalf("invalid HTTP address %q accepted", address)
		}
	}
	for _, address := range []string{":8080", "0.0.0.0:8080", "[::]:8080"} {
		if err := validateHTTPAddress(address); err != nil {
			t.Fatalf("valid HTTP address %q rejected: %v", address, err)
		}
	}
}

func TestMigrationDatabaseEnvironmentFailsClosedOnMalformedInsecureFlag(t *testing.T) {
	t.Setenv("HOURPATHS_DATABASE_DSN", "postgres://localhost/app?sslmode=disable")
	t.Setenv("HOURPATHS_DATABASE_INSECURE", "sometimes")
	if _, _, err := LoadDatabase(); err == nil {
		t.Fatal("migration database configuration accepted malformed insecure flag")
	}
	t.Setenv("HOURPATHS_DATABASE_INSECURE", "true")
	dsn, insecure, err := LoadDatabase()
	if err != nil {
		t.Fatal(err)
	}
	if dsn == "" || !insecure {
		t.Fatalf("unexpected database configuration: dsn=%q insecure=%v", dsn, insecure)
	}
}

func TestRawEnvironmentParsingFailsClosed(t *testing.T) {
	t.Setenv("HOURPATHS_SCALAR_DOCS_DISABLED", "maybe")
	if err := validateRawEnvironment(); err == nil {
		t.Fatal("invalid boolean silently enabled its false fallback")
	}
	t.Setenv("HOURPATHS_SCALAR_DOCS_DISABLED", "true")
	t.Setenv("HOURPATHS_SESSION_TTL_MINUTES", "0")
	if err := validateRawEnvironment(); err == nil {
		t.Fatal("out-of-range integer silently selected fallback behavior")
	}
}
func TestInvitedEmailsAreNormalizedDeduplicatedAndValidated(t *testing.T) {
	t.Setenv("HOURPATHS_ACCOUNT_INVITED_EMAILS", " Invited@Example.COM,invited@example.com ")
	values := normalizedList("HOURPATHS_ACCOUNT_INVITED_EMAILS")
	if len(values) != 1 || values[0] != "invited@example.com" {
		t.Fatalf("unexpected invitations: %v", values)
	}
	config := Config{AccountInvitedEmails: []string{"not-an-email"}}
	if err := config.validate(); err == nil {
		t.Fatal("invalid invitation email accepted")
	}
}

func TestSecureConfigurationRejectsPlaintextBoundaries(t *testing.T) {
	base := validSecureConfig()
	base.MetricsBearerToken = "01234567890123456789012345678901"
	if err := base.validate(); err != nil {
		t.Fatalf("secure config rejected: %v", err)
	}
	tests := []struct {
		name   string
		change func(*Config)
	}{
		{"database TLS disabled", func(c *Config) { c.DatabaseDSN = "postgres://db.example/app?sslmode=disable" }},
		{"OIDC plaintext", func(c *Config) { c.OIDCIssuer = "http://id.example/dex" }},
		{"docs client missing audience", func(c *Config) { c.OIDCAudiences = []string{"web", "mobile"} }},
		{"docs redirect wrong path", func(c *Config) { c.OIDCDocsRedirectURI = "https://api.example/callback" }},
		{"docs redirect wrong origin", func(c *Config) { c.OIDCDocsRedirectURI = "https://other.example/docs" }},
		{"docs redirect invalid scheme", func(c *Config) {
			c.OIDCInsecure = true
			c.OIDCDocsRedirectURI = "ftp://api.example/docs"
			c.PublicBaseURL = "ftp://api.example"
		}},
		{"SpiceDB missing token", func(c *Config) { c.SpiceDBToken = "" }},
		{"CORS wildcard", func(c *Config) { c.CORSAllowedOrigins = []string{"*"} }},
		{"CORS path", func(c *Config) { c.CORSAllowedOrigins = []string{"https://app.example/path"} }},
		{"invalid trusted proxy", func(c *Config) { c.TrustedProxyCIDRs = []string{"not-a-network"} }},
		{"audit rate disabled", func(c *Config) { c.AuditEventsPerMinute = 0 }},
		{"absolute session shorter than rotating session", func(c *Config) { c.SessionAbsoluteTTLMinutes = 59 }},
		{"plaintext OTLP without explicit insecure mode", func(c *Config) { c.OTLPHTTPEndpoint = "http://collector.example:4318" }},
		{"missing current policy revision", func(c *Config) { c.Policy.Revision = 0 }},
		{"unsafe current policy revision", func(c *Config) { c.Policy.Revision = math.MaxInt32 + 1 }},
		{"missing current terms version", func(c *Config) { c.Policy.CurrentTermsVersion = "" }},
		{"plaintext terms URL", func(c *Config) { c.Policy.TermsURL = "http://app.example/legal/terms" }},
		{"policy URL credentials", func(c *Config) { c.Policy.PrivacyPolicyURL = "https://user:password@app.example/legal/privacy" }},
		{"policy URL query", func(c *Config) {
			c.Policy.CommunityGuidelinesURL = "https://app.example/community-guidelines?revision=current"
		}},
		{"support URL fragment", func(c *Config) { c.Policy.SupportURL = "https://app.example/support#contact" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := base
			test.change(&candidate)
			if err := candidate.validate(); err == nil {
				t.Fatal("expected configuration rejection")
			}
		})
	}
}

func TestExplicitLocalInsecureConfiguration(t *testing.T) {
	config := Config{PublicBaseURL: "http://localhost:8080", DatabaseDSN: "postgres://app:app@localhost/app?sslmode=disable", DatabaseInsecure: true, CursorSigningKey: "01234567890123456789012345678901", PushTokenKey: "01234567890123456789012345678901", PushProviderEndpoint: "https://exp.host", OIDCIssuer: "http://localhost:5556/dex", OIDCAudiences: []string{"web", "mobile", "docs"}, OIDCDocsClientID: "docs", OIDCDocsRedirectURI: "http://localhost:8080/docs", OIDCInsecure: true, SpiceDBEndpoint: "localhost:50051", SpiceDBInsecure: true, CORSAllowedOrigins: []string{"http://localhost:5173"}, AuditEventsPerMinute: 120, AuditLimiterPrincipals: 10000, SessionTTLMinutes: 60, SessionAbsoluteTTLMinutes: 720, Policy: PolicyConfiguration{Revision: 1, CurrentTermsVersion: "local-dev-v1", CurrentPrivacyPolicyVersion: "local-dev-v1", CurrentCommunityGuidelinesVersion: "local-dev-v1", TermsURL: "http://localhost:5173/legal/terms", PrivacyPolicyURL: "http://localhost:5173/legal/privacy", CommunityGuidelinesURL: "http://localhost:5173/community-guidelines", SupportURL: "http://localhost:5173/support"}}
	config.MetricsBearerToken = "01234567890123456789012345678901"
	if err := config.validate(); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyConfigurationRejectsUnsafeVersionsAndLocations(t *testing.T) {
	for name, change := range map[string]func(*Config){
		"blank version":         func(c *Config) { c.Policy.CurrentPrivacyPolicyVersion = " \t" },
		"oversized version":     func(c *Config) { c.Policy.CurrentCommunityGuidelinesVersion = string(make([]byte, 129)) },
		"invalid UTF-8 version": func(c *Config) { c.Policy.CurrentTermsVersion = string([]byte{0xff}) },
		"relative URL":          func(c *Config) { c.Policy.SupportURL = "/support" },
		"non-HTTP URL":          func(c *Config) { c.Policy.TermsURL = "mailto:support@example.com" },
	} {
		t.Run(name, func(t *testing.T) {
			config := validSecureConfig()
			change(&config)
			if err := config.validate(); err == nil {
				t.Fatal("unsafe policy configuration accepted")
			}
		})
	}
}

func TestPolicyConfigurationLoadsOnlyExplicitEnvironmentValues(t *testing.T) {
	want := PolicyConfiguration{
		Revision:                          7,
		CurrentTermsVersion:               "terms-2026-07",
		CurrentPrivacyPolicyVersion:       "privacy-2026-07",
		CurrentCommunityGuidelinesVersion: "guidelines-2026-07",
		TermsURL:                          "https://app.example/legal/terms",
		PrivacyPolicyURL:                  "https://app.example/legal/privacy",
		CommunityGuidelinesURL:            "https://app.example/community-guidelines",
		SupportURL:                        "https://app.example/support",
	}
	for key, value := range map[string]string{
		"HOURPATHS_CURRENT_POLICY_REVISION":              "7",
		"HOURPATHS_CURRENT_TERMS_VERSION":                want.CurrentTermsVersion,
		"HOURPATHS_CURRENT_PRIVACY_POLICY_VERSION":       want.CurrentPrivacyPolicyVersion,
		"HOURPATHS_CURRENT_COMMUNITY_GUIDELINES_VERSION": want.CurrentCommunityGuidelinesVersion,
		"HOURPATHS_TERMS_URL":                            want.TermsURL,
		"HOURPATHS_PRIVACY_POLICY_URL":                   want.PrivacyPolicyURL,
		"HOURPATHS_COMMUNITY_GUIDELINES_URL":             want.CommunityGuidelinesURL,
		"HOURPATHS_SUPPORT_URL":                          want.SupportURL,
	} {
		t.Setenv(key, value)
	}
	if got := loadPolicyConfiguration(); got != want {
		t.Fatalf("policy configuration = %#v, want %#v", got, want)
	}
}

func TestPolicyConfigurationHasNoProductionDefaults(t *testing.T) {
	for _, key := range []string{
		"HOURPATHS_CURRENT_POLICY_REVISION",
		"HOURPATHS_CURRENT_TERMS_VERSION",
		"HOURPATHS_CURRENT_PRIVACY_POLICY_VERSION",
		"HOURPATHS_CURRENT_COMMUNITY_GUIDELINES_VERSION",
		"HOURPATHS_TERMS_URL",
		"HOURPATHS_PRIVACY_POLICY_URL",
		"HOURPATHS_COMMUNITY_GUIDELINES_URL",
		"HOURPATHS_SUPPORT_URL",
	} {
		t.Setenv(key, "")
	}
	if got := loadPolicyConfiguration(); got != (PolicyConfiguration{}) {
		t.Fatalf("missing environment selected policy defaults: %#v", got)
	}
}

func validSecureConfig() Config {
	return Config{PublicBaseURL: "https://api.example", DatabaseDSN: "postgres://app:secret@db.example/app?sslmode=verify-full", CursorSigningKey: "01234567890123456789012345678901", PushTokenKey: "01234567890123456789012345678901", PushProviderEndpoint: "https://exp.host", MetricsBearerToken: "01234567890123456789012345678901", OIDCIssuer: "https://id.example/dex", OIDCAudiences: []string{"web", "mobile", "docs"}, OIDCDocsClientID: "docs", OIDCDocsRedirectURI: "https://api.example/docs", SpiceDBEndpoint: "spicedb.example:50051", SpiceDBToken: "secret", CORSAllowedOrigins: []string{"https://app.example"}, AuditEventsPerMinute: 120, AuditLimiterPrincipals: 10000, SessionTTLMinutes: 60, SessionAbsoluteTTLMinutes: 720, Policy: PolicyConfiguration{Revision: 1, CurrentTermsVersion: "terms-v1", CurrentPrivacyPolicyVersion: "privacy-v1", CurrentCommunityGuidelinesVersion: "guidelines-v1", TermsURL: "https://app.example/legal/terms", PrivacyPolicyURL: "https://app.example/legal/privacy", CommunityGuidelinesURL: "https://app.example/community-guidelines", SupportURL: "https://app.example/support"}}
}
func TestDisabledScalarDoesNotRequirePhantomDocsClient(t *testing.T) {
	config := validSecureConfig()
	config.OIDCAudiences = []string{"web", "mobile"}
	config.OIDCDocsClientID = ""
	config.OIDCDocsRedirectURI = ""
	config.ScalarDocsDisabled = true
	if err := config.validate(); err != nil {
		t.Fatalf("docs-disabled production config required phantom Scalar credentials: %v", err)
	}
}
func TestKnownLocalSecretsAreRejectedAtSecureBoundaries(t *testing.T) {
	config := validSecureConfig()
	config.CursorSigningKey = "local-development-cursor-signing-key-change-me"
	config.MetricsBearerToken = "local-development-metrics-token-change-me"
	config.OIDCAudiences = []string{"web"}
	config.OIDCDocsClientID = ""
	config.OIDCDocsRedirectURI = ""
	config.ScalarDocsDisabled = true
	if err := config.validate(); err == nil {
		t.Fatal("secure configuration accepted publicly known local credentials")
	}
}
func TestMigrationDatabaseValidationUsesSameTLSBoundary(t *testing.T) {
	if err := ValidateDatabase("postgres://db.example/app?sslmode=disable", false); err == nil {
		t.Fatal("migration accepted plaintext database without explicit insecure mode")
	}
	if err := ValidateDatabase("postgres://db.example/app?sslmode=verify-full", false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDatabase("postgres://localhost/app?sslmode=disable", true); err != nil {
		t.Fatal(err)
	}
}
