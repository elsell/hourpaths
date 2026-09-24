# Production checklist

Deploy the API and web images by immutable digest. Run PostgreSQL migrations and
the SpiceDB datastore migration/schema jobs with separate credentials before
starting API replicas. API database credentials must not own schema or migration
state; API SpiceDB credentials go through the capability proxy and cannot change
schema.

Required preparation:

- Set `HOURPATHS_APP_ENV=production`, `HOURPATHS_API_URL`,
  `HOURPATHS_OIDC_ISSUER`, and `HOURPATHS_WEB_OIDC_CLIENT_ID` for the web container.
  The image defaults to production mode and refuses to serve with absent, local,
  credentialed, or non-HTTPS production endpoints. Local Compose explicitly
  overrides this to development.
- Set `HOURPATHS_CURRENT_POLICY_REVISION` to a positive, monotonically increasing
  number, incrementing it whenever any published policy version or location
  changes. Set `HOURPATHS_CURRENT_TERMS_VERSION`,
  `HOURPATHS_CURRENT_PRIVACY_POLICY_VERSION`, and
  `HOURPATHS_CURRENT_COMMUNITY_GUIDELINES_VERSION` to the exact immutable version
  identifiers users are accepting or acknowledging. Set `HOURPATHS_TERMS_URL`,
  `HOURPATHS_PRIVACY_POLICY_URL`, `HOURPATHS_COMMUNITY_GUIDELINES_URL`, and
  `HOURPATHS_SUPPORT_URL` to their published HTTPS locations; none of these API
  settings has a production default.
- After migrations, run `/policy-publish -updated-at <immutable-RFC3339-instant>`
  once with migration credentials and the desired policy settings, before rolling
  API replicas. Reusing a revision with changed metadata fails; a lower revision
  cannot overwrite shared state. API credentials retain read-only access to the
  authority, and every onboarding read resolves it from PostgreSQL.
- Keep the generated Red Hat Hardened Images for Go, the static API runtime,
  Node.js, and PostgreSQL unless a documented compatibility constraint requires
  another source. When updating a base, select an immutable catalog release,
  pin its manifest-list digest, inspect its SBOM and CVE report, and verify its
  Red Hat signature using the catalog's current instructions. Hardened Images
  are usable without a Red Hat subscription. Do not claim FIPS compliance from
  a FIPS image tag alone; validate the complete application, cryptographic
  configuration, host, and deployment boundary.
- Terminate TLS at a trusted proxy and forward only validated scheme/host data.
  Keep API, PostgreSQL, SpiceDB, OTLP, and OIDC transport encryption enabled.
  Set `HOURPATHS_TRUSTED_PROXY_CIDRS` to only that proxy network when
  client-source forwarding is required; leave it empty for direct deployments.
- Generate independent high-entropy cursor, push-token encryption, metrics,
  database, SpiceDB, and other credentials. Set
  `HOURPATHS_PUSH_TOKEN_KEY` to at least 32 bytes of dedicated secret material;
  rotating it requires a coordinated installation re-registration plan because
  existing token snapshots cannot be decrypted with a replacement key. Keep
  `HOURPATHS_PUSH_PROVIDER_ENDPOINT` on its default HTTPS Expo boundary.
  `HOURPATHS_PUSH_PROVIDER_INSECURE` exists only for controlled local acceptance
  and must remain false in production. Never deploy values from `.env.example`
  or Compose.
- Configure exact public API/web origins, CORS origins, OIDC clients and redirect
  URIs using `docs/oidc.md`.
- Size the PostgreSQL connection pool across all replicas, not per process in
  isolation. Configure request/audit limits, session lifetimes, and authorization
  retry/dead-letter policy for expected traffic.
- Set `HOURPATHS_OWNERSHIP_TRANSFER_EXPIRATION_MINUTES` to the positive whole
  number of minutes used for newly created ownership transfers. The documented
  default is `10080` (seven days); changing it does not alter existing transfers.
- Configure OTLP export, scrape the authenticated metrics endpoint, and alert on
  readiness, authorization dead letters, audit-write rate, database capacity,
  latency, and error probes.
- Choose and test an audit retention period. Retention is disabled by default
  and must run with its separate database role; the API runtime cannot erase
  audit events.
- Bootstrap invitation administrators before selecting invitation-only account
  provisioning, then test invitation expiry, consumption, revocation, and
  concurrent first login.
- Route liveness and readiness separately. During rolling deployment, complete
  migrations and authorization schema first, wait for readiness, and drain API
  replicas using their bounded shutdown window.
- Exercise restore procedures and a complete staging rollout even when backup
  infrastructure is managed outside this repository.

Scalar documentation can be disabled. Do so unless interactive production API
documentation is intentional and its public OIDC client is registered.
