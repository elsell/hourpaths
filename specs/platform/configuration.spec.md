# Application Configuration

Status: Approved for implementation

## Purpose

Define deployment-level configuration behavior that affects product rules.

## Environment-variable naming

- Every application-owned environment variable must begin with a common,
  application-specific prefix followed by an underscore.
- Environment-variable names must use uppercase ASCII letters, digits, and
  underscores.
- The canonical application prefix must be `HOURPATHS`.
- Changing the product's display name must not silently change the established
  application prefix after deployment.
- Web and mobile application configuration must enter through canonical
  `HOURPATHS_`-prefixed deployment inputs; framework-specific public-environment
  prefixes must not become a second application configuration contract.
- The web server must validate application configuration before injecting its
  public values into rendered route data. Browser components must not read the
  process environment directly.
- Expo configuration must translate canonical deployment inputs into typed
  public `extra` values. Mobile application code must consume and validate that
  injected configuration rather than reading bundle environment variables.
- Production client configuration must reject missing or malformed deployment
  environments, blank OIDC client identifiers, and API or issuer endpoints that
  are not HTTPS or target local or loopback hosts.
- Production OIDC client identifiers must be the broker-assigned protocol
  identifiers for the deployed applications. Human-readable application names
  must not be substituted for those identifiers. Local Dex client identifiers
  may remain stable development defaults and must not override explicit
  production configuration.

## Ownership-transfer lifetime

- The lifetime of a pending ownership-transfer request must be configurable at
  the deployment level.
- The value must be supplied by
  `HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES` through
  environment-backed configuration.
- When the environment value is absent, the application must use a documented
  default lifetime.
- The environment-backed ownership-transfer lifetime must be expressed in
  minutes.
- The default ownership-transfer lifetime must be 10,080 minutes.
- The configured lifetime must be a whole number of minutes.
- The application must reject a malformed, fractional, zero, or negative value.
- The product must not impose a maximum ownership-transfer lifetime.
- The application must reject a value that cannot be represented safely or
  cannot produce a valid expiration timestamp.
- The configured lifetime must be used only when a new ownership-transfer
  request is created.
- Changing the configured lifetime must affect new requests only; each existing
  request must continue to use its stored expiration time.
- Users must not be able to choose or override a transfer lifetime in the current
  product behavior.
- Configuration must be validated before the application begins serving
  requests or processing background work.
- Application and domain code must receive the validated duration through typed
  configuration and must not read the environment directly.
