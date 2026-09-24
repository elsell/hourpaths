package config

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type Config struct {
	HTTPAddr, PublicBaseURL, DatabaseDSN, CursorSigningKey, PushTokenKey, PushProviderEndpoint, OIDCIssuer, OIDCBackchannelURL, OIDCDocsClientID, OIDCDocsRedirectURI, SpiceDBEndpoint, SpiceDBToken, AccountProvisioningMode string
	MetricsBearerToken                                                                                                                                                                                                        string
	Policy                                                                                                                                                                                                                    PolicyConfiguration
	OTLPHTTPEndpoint, OTelServiceName                                                                                                                                                                                         string
	OIDCAudiences                                                                                                                                                                                                             []string
	AccountInvitedEmails                                                                                                                                                                                                      []string
	InvitationAdminIdentities                                                                                                                                                                                                 []string
	DatabaseInsecure, PushProviderInsecure, OIDCInsecure, SpiceDBInsecure                                                                                                                                                     bool
	ScalarDocsDisabled                                                                                                                                                                                                        bool
	AccountSelfDeactivationEnabled                                                                                                                                                                                            bool
	OTLPInsecure                                                                                                                                                                                                              bool
	CORSAllowedOrigins                                                                                                                                                                                                        []string
	TrustedProxyCIDRs                                                                                                                                                                                                         []string
	AuditEventsPerMinute, AuditLimiterPrincipals                                                                                                                                                                              int
	DatabaseMaxOpenConns, DatabaseMaxIdleConns, DatabaseConnMaxLifetime, DatabaseConnMaxIdleTime                                                                                                                              int
	SessionTTLMinutes                                                                                                                                                                                                         int
	SessionAbsoluteTTLMinutes                                                                                                                                                                                                 int
	AuthorizationMaxAttempts                                                                                                                                                                                                  int
	RequestsPerMinute, RequestLimiterSources                                                                                                                                                                                  int
	OwnershipTransferExpiration                                                                                                                                                                                               time.Duration
}

// PolicyConfiguration identifies the exact published policies presented by
// onboarding and the monitored support location. It has no production defaults.
type PolicyConfiguration struct {
	Revision                                                                            int64
	CurrentTermsVersion, CurrentPrivacyPolicyVersion, CurrentCommunityGuidelinesVersion string
	TermsURL, PrivacyPolicyURL, CommunityGuidelinesURL, SupportURL                      string
}

func Load() (Config, error) {
	if err := validateRawEnvironment(); err != nil {
		return Config{}, err
	}
	ownershipTransferExpiration, err := ownershipTransferExpiration()
	if err != nil {
		return Config{}, err
	}
	c := Config{
		HTTPAddr:                       value("HOURPATHS_HTTP_ADDR", ":8080"),
		PublicBaseURL:                  os.Getenv("HOURPATHS_PUBLIC_BASE_URL"),
		DatabaseDSN:                    os.Getenv("HOURPATHS_DATABASE_DSN"),
		DatabaseInsecure:               boolean("HOURPATHS_DATABASE_INSECURE"),
		CursorSigningKey:               os.Getenv("HOURPATHS_CURSOR_SIGNING_KEY"),
		PushTokenKey:                   os.Getenv("HOURPATHS_PUSH_TOKEN_KEY"),
		PushProviderEndpoint:           value("HOURPATHS_PUSH_PROVIDER_ENDPOINT", "https://exp.host"),
		PushProviderInsecure:           boolean("HOURPATHS_PUSH_PROVIDER_INSECURE"),
		MetricsBearerToken:             os.Getenv("HOURPATHS_METRICS_BEARER_TOKEN"),
		Policy:                         loadPolicyConfiguration(),
		OTLPHTTPEndpoint:               os.Getenv("HOURPATHS_OTEL_EXPORTER_OTLP_ENDPOINT"),
		OTLPInsecure:                   boolean("HOURPATHS_OTEL_EXPORTER_OTLP_INSECURE"),
		OTelServiceName:                value("HOURPATHS_OTEL_SERVICE_NAME", "hourpaths-api"),
		AccountProvisioningMode:        value("HOURPATHS_ACCOUNT_PROVISIONING_MODE", "open"),
		AccountInvitedEmails:           normalizedList("HOURPATHS_ACCOUNT_INVITED_EMAILS"),
		InvitationAdminIdentities:      list("HOURPATHS_INVITATION_ADMIN_IDENTITIES"),
		OIDCIssuer:                     os.Getenv("HOURPATHS_OIDC_ISSUER"),
		OIDCBackchannelURL:             os.Getenv("HOURPATHS_OIDC_BACKCHANNEL_URL"),
		OIDCAudiences:                  list("HOURPATHS_OIDC_AUDIENCES"),
		OIDCDocsClientID:               os.Getenv("HOURPATHS_OIDC_DOCS_CLIENT_ID"),
		OIDCDocsRedirectURI:            os.Getenv("HOURPATHS_OIDC_DOCS_REDIRECT_URI"),
		OIDCInsecure:                   boolean("HOURPATHS_OIDC_INSECURE"),
		SpiceDBEndpoint:                os.Getenv("HOURPATHS_SPICEDB_ENDPOINT"),
		SpiceDBToken:                   os.Getenv("HOURPATHS_SPICEDB_TOKEN"),
		SpiceDBInsecure:                boolean("HOURPATHS_SPICEDB_INSECURE"),
		CORSAllowedOrigins:             list("HOURPATHS_CORS_ALLOWED_ORIGINS"),
		TrustedProxyCIDRs:              list("HOURPATHS_TRUSTED_PROXY_CIDRS"),
		AuditEventsPerMinute:           integer("HOURPATHS_AUDIT_EVENTS_PER_MINUTE", 120),
		AuditLimiterPrincipals:         integer("HOURPATHS_AUDIT_LIMITER_PRINCIPALS", 10000),
		ScalarDocsDisabled:             boolean("HOURPATHS_SCALAR_DOCS_DISABLED"),
		AccountSelfDeactivationEnabled: boolean("HOURPATHS_ACCOUNT_SELF_DEACTIVATION_ENABLED"),
		DatabaseMaxOpenConns:           integer("HOURPATHS_DATABASE_MAX_OPEN_CONNS", 25),
		DatabaseMaxIdleConns:           integer("HOURPATHS_DATABASE_MAX_IDLE_CONNS", 10),
		DatabaseConnMaxLifetime:        integer("HOURPATHS_DATABASE_CONN_MAX_LIFETIME_SECONDS", 1800),
		DatabaseConnMaxIdleTime:        integer("HOURPATHS_DATABASE_CONN_MAX_IDLE_SECONDS", 300),
		SessionTTLMinutes:              integer("HOURPATHS_SESSION_TTL_MINUTES", 60),
		SessionAbsoluteTTLMinutes:      integer("HOURPATHS_SESSION_ABSOLUTE_TTL_MINUTES", 720),
		AuthorizationMaxAttempts:       integer("HOURPATHS_AUTHORIZATION_MAX_ATTEMPTS", 5),
		RequestsPerMinute:              integer("HOURPATHS_REQUESTS_PER_MINUTE", 300),
		RequestLimiterSources:          integer("HOURPATHS_REQUEST_LIMITER_SOURCES", 10000),
		OwnershipTransferExpiration:    ownershipTransferExpiration,
	}
	if err := c.validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func validateRawEnvironment() error {
	for _, key := range []string{"HOURPATHS_DATABASE_INSECURE", "HOURPATHS_PUSH_PROVIDER_INSECURE", "HOURPATHS_OIDC_INSECURE", "HOURPATHS_SPICEDB_INSECURE", "HOURPATHS_SCALAR_DOCS_DISABLED", "HOURPATHS_ACCOUNT_SELF_DEACTIVATION_ENABLED", "HOURPATHS_OTEL_EXPORTER_OTLP_INSECURE"} {
		// hourpaths-env-inventory: allow-computed reviewed fixed boolean key registry
		if raw := os.Getenv(key); raw != "" {
			if _, err := strconv.ParseBool(raw); err != nil {
				return errors.New("invalid boolean configuration for " + key)
			}
		}
	}
	ranges := []struct {
		key      string
		min, max int
	}{
		{"HOURPATHS_AUDIT_EVENTS_PER_MINUTE", 1, 10000},
		{"HOURPATHS_AUDIT_LIMITER_PRINCIPALS", 100, 1000000},
		{"HOURPATHS_DATABASE_MAX_OPEN_CONNS", 1, 100000},
		{"HOURPATHS_DATABASE_MAX_IDLE_CONNS", 0, 100000},
		{"HOURPATHS_DATABASE_CONN_MAX_LIFETIME_SECONDS", 1, 86400},
		{"HOURPATHS_DATABASE_CONN_MAX_IDLE_SECONDS", 1, 86400},
		{"HOURPATHS_SESSION_TTL_MINUTES", 5, 1440},
		{"HOURPATHS_SESSION_ABSOLUTE_TTL_MINUTES", 5, 10080},
		{"HOURPATHS_AUTHORIZATION_MAX_ATTEMPTS", 1, 100},
		{"HOURPATHS_REQUESTS_PER_MINUTE", 1, 100000},
		{"HOURPATHS_REQUEST_LIMITER_SOURCES", 100, 1000000},
		{"HOURPATHS_CURRENT_POLICY_REVISION", 1, math.MaxInt32},
	}
	for _, item := range ranges {
		// hourpaths-env-inventory: allow-computed reviewed fixed integer key registry
		raw := os.Getenv(item.key)
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < item.min || value > item.max {
			return errors.New("invalid integer configuration for " + item.key)
		}
	}
	return nil
}

func ownershipTransferExpiration() (time.Duration, error) {
	const defaultMinutes int64 = 10080
	raw := os.Getenv("HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES")
	if raw == "" {
		raw = strconv.FormatInt(defaultMinutes, 10)
	}
	minutes, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || minutes < 1 || minutes > math.MaxInt64/int64(time.Minute) {
		return 0, errors.New("HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES must be a positive whole number of safely representable minutes")
	}
	return time.Duration(minutes) * time.Minute, nil
}

func validateHTTPAddress(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil || port == "" {
		return errors.New("HOURPATHS_HTTP_ADDR must be a host and numeric TCP port")
	}
	for _, character := range port {
		if character < '0' || character > '9' {
			return errors.New("HOURPATHS_HTTP_ADDR must be a host and numeric TCP port")
		}
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort > 65535 {
		return errors.New("HOURPATHS_HTTP_ADDR must use a TCP port between 0 and 65535")
	}
	return nil
}

func (c Config) validate() error {
	if c.HTTPAddr != "" {
		if err := validateHTTPAddress(c.HTTPAddr); err != nil {
			return err
		}
	}
	if c.PublicBaseURL == "" || c.DatabaseDSN == "" || len(c.CursorSigningKey) < 32 ||
		len(c.PushTokenKey) < 32 || len(c.MetricsBearerToken) < 32 || c.OIDCIssuer == "" ||
		len(c.OIDCAudiences) == 0 || c.SpiceDBEndpoint == "" {
		return errors.New("database, OIDC audiences, and SpiceDB configuration is required")
	}
	pushEndpoint, err := url.Parse(c.PushProviderEndpoint)
	if err != nil || pushEndpoint.Host == "" || pushEndpoint.User != nil ||
		pushEndpoint.RawQuery != "" || pushEndpoint.Fragment != "" ||
		(pushEndpoint.Scheme != "https" && !(c.PushProviderInsecure && pushEndpoint.Scheme == "http")) {
		return errors.New("push provider endpoint must use HTTPS unless local insecure mode is explicit")
	}
	if !c.ScalarDocsDisabled && (c.OIDCDocsClientID == "" || c.OIDCDocsRedirectURI == "") {
		return errors.New("OIDC docs configuration is required while Scalar documentation is enabled")
	}
	if err := c.Policy.validate(c.OIDCInsecure); err != nil {
		return err
	}
	localSecrets := c.CursorSigningKey == "local-development-cursor-signing-key-change-me" || c.MetricsBearerToken == "local-development-metrics-token-change-me"
	if localSecrets && !(c.DatabaseInsecure && c.OIDCInsecure && c.SpiceDBInsecure) {
		return errors.New("local development signing and metrics credentials cannot be used at a secure boundary")
	}
	if c.AuditEventsPerMinute < 1 || c.AuditEventsPerMinute > 10000 || c.AuditLimiterPrincipals < 100 || c.AuditLimiterPrincipals > 1000000 {
		return errors.New("audit rate limit configuration is out of range")
	}
	if c.SessionTTLMinutes != 0 && (c.SessionTTLMinutes < 5 || c.SessionTTLMinutes > 1440) {
		return errors.New("session TTL must be between 5 and 1440 minutes")
	}
	if c.SessionAbsoluteTTLMinutes != 0 && (c.SessionAbsoluteTTLMinutes < c.SessionTTLMinutes || c.SessionAbsoluteTTLMinutes > 10080) {
		return errors.New("session absolute TTL must be at least the rotating TTL and at most seven days")
	}
	if c.AuthorizationMaxAttempts != 0 && (c.AuthorizationMaxAttempts < 1 || c.AuthorizationMaxAttempts > 100) {
		return errors.New("authorization max attempts is out of range")
	}
	if (c.RequestsPerMinute != 0 || c.RequestLimiterSources != 0) && (c.RequestsPerMinute < 1 || c.RequestsPerMinute > 100000 || c.RequestLimiterSources < 100 || c.RequestLimiterSources > 1000000) {
		return errors.New("request rate limit configuration is out of range")
	}
	if c.AccountProvisioningMode != "" && c.AccountProvisioningMode != "open" && c.AccountProvisioningMode != "existing" {
		return errors.New("account provisioning mode must be open or existing")
	}
	for _, email := range c.AccountInvitedEmails {
		if !validEmail(email) {
			return errors.New("account invited emails must contain valid normalized addresses")
		}
	}
	for _, configured := range c.InvitationAdminIdentities {
		separator := strings.LastIndex(configured, "#")
		if separator < 1 || separator == len(configured)-1 {
			return errors.New("invitation administrator identities must use issuer#subject")
		}
		issuer, err := url.Parse(configured[:separator])
		if err != nil || issuer.Host == "" || issuer.User != nil || issuer.RawQuery != "" || issuer.Fragment != "" || (issuer.Scheme != "https" && !(c.OIDCInsecure && issuer.Scheme == "http")) {
			return errors.New("invitation administrator identities must contain a valid issuer and non-empty subject")
		}
	}
	if c.OTLPHTTPEndpoint != "" {
		endpoint, err := url.Parse(c.OTLPHTTPEndpoint)
		if err != nil || endpoint.Host == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || (endpoint.Scheme != "https" && !(c.OTLPInsecure && endpoint.Scheme == "http")) || c.OTelServiceName == "" {
			return errors.New("OTLP endpoint must be an absolute HTTPS URL unless OTLP insecure mode is explicit")
		}
	}
	poolConfigured := c.DatabaseMaxOpenConns != 0 || c.DatabaseMaxIdleConns != 0 || c.DatabaseConnMaxLifetime != 0 || c.DatabaseConnMaxIdleTime != 0
	if poolConfigured && (c.DatabaseMaxOpenConns < 1 || c.DatabaseMaxIdleConns < 0 || c.DatabaseMaxIdleConns > c.DatabaseMaxOpenConns || c.DatabaseConnMaxLifetime < 1 || c.DatabaseConnMaxIdleTime < 1) {
		return errors.New("database pool configuration is invalid")
	}
	if err := ValidateDatabase(c.DatabaseDSN, c.DatabaseInsecure); err != nil {
		return err
	}
	issuer, err := url.Parse(c.OIDCIssuer)
	if err != nil || issuer.Host == "" || issuer.User != nil || issuer.RawQuery != "" || issuer.Fragment != "" {
		return errors.New("OIDC issuer must be an absolute URL without credentials, query, or fragment")
	}
	if !c.OIDCInsecure && issuer.Scheme != "https" {
		return errors.New("OIDC issuer must use HTTPS unless OIDC_INSECURE is true")
	}
	if c.OIDCInsecure && issuer.Scheme != "http" && issuer.Scheme != "https" {
		return errors.New("OIDC issuer must use HTTP or HTTPS")
	}
	if c.OIDCBackchannelURL != "" {
		backchannel, err := url.Parse(c.OIDCBackchannelURL)
		if err != nil || backchannel.Host == "" || backchannel.User != nil || backchannel.RawQuery != "" || backchannel.Fragment != "" || backchannel.Path != issuer.Path {
			return errors.New("OIDC backchannel URL must be an absolute URL with the same path as the public issuer and without credentials, query, or fragment")
		}
		if backchannel.Scheme != "https" && !(c.OIDCInsecure && backchannel.Scheme == "http") {
			return errors.New("OIDC backchannel URL must use HTTPS unless OIDC_INSECURE is true")
		}
	}
	publicBase, err := url.Parse(c.PublicBaseURL)
	if err != nil || publicBase.Host == "" || publicBase.User != nil || publicBase.Path != "" || publicBase.RawQuery != "" || publicBase.Fragment != "" || (publicBase.Scheme != "https" && !(c.OIDCInsecure && publicBase.Scheme == "http")) {
		return errors.New("public API base URL must be a secure origin unless OIDC insecure mode is explicit")
	}
	if !c.ScalarDocsDisabled {
		docsRedirect, err := url.Parse(c.OIDCDocsRedirectURI)
		if err != nil || docsRedirect.Host == "" || docsRedirect.User != nil || docsRedirect.RawQuery != "" || docsRedirect.Fragment != "" {
			return errors.New("OIDC docs redirect must be an absolute URL without credentials, query, or fragment")
		}
		if docsRedirect.Scheme != "http" && docsRedirect.Scheme != "https" {
			return errors.New("OIDC docs redirect must use HTTP or HTTPS")
		}
		if !c.OIDCInsecure && docsRedirect.Scheme != "https" {
			return errors.New("OIDC docs redirect must use HTTPS unless OIDC_INSECURE is true")
		}
		if docsRedirect.Path != "/docs" || !includes(c.OIDCAudiences, c.OIDCDocsClientID) {
			return errors.New("OIDC docs redirect must target /docs and its client ID must be an allowed audience")
		}
		if publicBase.Scheme != docsRedirect.Scheme || publicBase.Host != docsRedirect.Host {
			return errors.New("public API base URL must match the OIDC docs redirect origin")
		}
	}
	if !c.SpiceDBInsecure && c.SpiceDBToken == "" {
		return errors.New("secure SpiceDB requires a preshared token")
	}
	for _, origin := range c.CORSAllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && !(c.OIDCInsecure && parsed.Scheme == "http")) {
			return errors.New("CORS origins must be secure origins without paths")
		}
	}
	for _, network := range c.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(network); err != nil {
			return errors.New("trusted proxy CIDRs must be valid network prefixes")
		}
	}
	return nil
}

func loadPolicyConfiguration() PolicyConfiguration {
	return PolicyConfiguration{
		Revision:                          int64(integer("HOURPATHS_CURRENT_POLICY_REVISION", 0)),
		CurrentTermsVersion:               os.Getenv("HOURPATHS_CURRENT_TERMS_VERSION"),
		CurrentPrivacyPolicyVersion:       os.Getenv("HOURPATHS_CURRENT_PRIVACY_POLICY_VERSION"),
		CurrentCommunityGuidelinesVersion: os.Getenv("HOURPATHS_CURRENT_COMMUNITY_GUIDELINES_VERSION"),
		TermsURL:                          os.Getenv("HOURPATHS_TERMS_URL"),
		PrivacyPolicyURL:                  os.Getenv("HOURPATHS_PRIVACY_POLICY_URL"),
		CommunityGuidelinesURL:            os.Getenv("HOURPATHS_COMMUNITY_GUIDELINES_URL"),
		SupportURL:                        os.Getenv("HOURPATHS_SUPPORT_URL"),
	}
}

func (p PolicyConfiguration) validate(localInsecure bool) error {
	if p.Revision < 1 || p.Revision > math.MaxInt32 {
		return errors.New("current policy revision must be between 1 and 2147483647")
	}
	versions := []string{p.CurrentTermsVersion, p.CurrentPrivacyPolicyVersion, p.CurrentCommunityGuidelinesVersion}
	for _, version := range versions {
		if !utf8.ValidString(version) || strings.TrimSpace(version) == "" || utf8.RuneCountInString(version) > 128 {
			return errors.New("current policy versions are required, valid UTF-8, non-blank, and at most 128 characters")
		}
	}
	for _, location := range []string{p.TermsURL, p.PrivacyPolicyURL, p.CommunityGuidelinesURL, p.SupportURL} {
		parsed, err := url.Parse(location)
		if err != nil || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Scheme != "https" && !(localInsecure && parsed.Scheme == "http")) {
			return errors.New("policy and support locations must be absolute HTTPS URLs without credentials, query, or fragment unless local insecure mode is explicit")
		}
	}
	return nil
}

// LoadDatabase returns the validated database settings shared by auxiliary
// runtimes that do not need the API server's complete configuration.
func LoadDatabase() (string, bool, error) {
	dsn := os.Getenv("HOURPATHS_DATABASE_DSN")
	rawInsecure := os.Getenv("HOURPATHS_DATABASE_INSECURE")
	insecure := false
	if rawInsecure != "" {
		parsed, err := strconv.ParseBool(rawInsecure)
		if err != nil {
			return "", false, fmt.Errorf("invalid boolean configuration for HOURPATHS_DATABASE_INSECURE: %w", err)
		}
		insecure = parsed
	}
	if err := ValidateDatabase(dsn, insecure); err != nil {
		return "", false, err
	}
	return dsn, insecure, nil
}

func ValidateDatabase(dsn string, insecure bool) error {
	database, err := url.Parse(dsn)
	if err != nil || (database.Scheme != "postgres" && database.Scheme != "postgresql") || database.Host == "" {
		return errors.New("database DSN must be an absolute PostgreSQL URL")
	}
	if !insecure && database.Query().Get("sslmode") != "verify-full" {
		return errors.New("database sslmode=verify-full is required unless DATABASE_INSECURE is true")
	}
	return nil
}
func includes(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func value(key, fallback string) string {
	// hourpaths-env-inventory: allow-computed reviewed typed value helper
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
func boolean(key string) bool {
	// hourpaths-env-inventory: allow-computed reviewed typed boolean helper
	value, _ := strconv.ParseBool(os.Getenv(key))
	return value
}
func integer(key string, fallback int) int {
	// hourpaths-env-inventory: allow-computed reviewed typed integer helper
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return value
}
func list(key string) []string {
	var out []string
	// hourpaths-env-inventory: allow-computed reviewed typed list helper
	for _, value := range strings.Split(os.Getenv(key), ",") {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
func normalizedList(key string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, value := range list(key) {
		value = strings.ToLower(value)
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			out = append(out, value)
		}
	}
	return out
}
func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Name == "" && address.Address == value && strings.Contains(value, "@")
}
