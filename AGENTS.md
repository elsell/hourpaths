# Project Engineering Guidance

This is a spec-driven, security-conscious application. Specifications under
`specs/` are the source of truth. Update the relevant `*.spec.md` before coding.

## Current phase

- The current phase is product implementation. Optimize for complete,
  spec-backed, user-visible vertical slices merged to `main`.
- Keep exactly one product slice in progress. At most one additional
  infrastructure issue may be in progress, and only when it blocks that slice or
  presents a demonstrated P0/P1 security, authorization, correctness, data-loss,
  or release risk.
- Freeze foundation expansion, make-app improvements, dependency work, and
  P2/P3 hardening when they do not meet that threshold. Record them without
  interrupting the active slice.
- A slice is complete only when its behavior is demonstrable through the
  relevant client, backed by the authoritative spec and focused tests, and
  merged to `main`.
- Keep implementation branches short lived. Integrate current `main` before
  starting a slice and aim to merge within one working day.

## Product delivery operating contract

### Scope and stop rules

- Implement the smallest coherent behavior that a user can observe from end to
  end. Do not turn adjacent cleanup, refactoring, or foundation opportunities
  into acceptance criteria for the slice.
- Timebox investigation of a blocking infrastructure problem to 90 minutes.
  After that, use the safest workable fallback and record follow-up hardening
  unless the problem still demonstrably blocks delivery or is P0/P1.
- Do not keep polishing infrastructure after a reliable path exists for the
  active slice.

### Proportional TDD and verification

- Preserve TDD: begin with the closest failing behavioral test, implement the
  smallest change, then refactor.
- During development, run the smallest relevant package, component, type, and
  structural checks. Run affected boundary integration tests when a contract,
  persistence, authorization, or client/server boundary changes.
- Run the applicable full PR merge gate once on the final candidate, not after
  every commit or revision.
- Run exact-SHA Docker acceptance and production-image verification on the final
  candidate only. Run clean native builds only when native dependencies,
  configuration, generated projects, or native source changed.
- Authentication, authorization, migrations, destructive lifecycle behavior,
  and data-integrity changes always receive their applicable adversarial and
  integration gates before merge.
- Put exhaustive unaffected-platform, broad vulnerability, full browser-matrix,
  and release-image checks on `main`, nightly, or release workflows unless the
  active diff presents the corresponding risk.

### Development environment and CI

- Keep the shared API, Dex, PostgreSQL, SpiceDB, and Metro development
  environment warm and available for user verification.
- Deployment scripts must derive the Git SHA mechanically and expose the
  deployed revision; never copy or type a candidate SHA manually.
- Rebuild and restart only affected services unless a schema, dependency, or
  runtime-boundary change requires more.
- CI must calculate changed areas once, cancel superseded runs for the same
  branch, reuse dependency and build caches, and route only required PR jobs.
  Required risk-triggered merge gates must never be skipped.
- Local hooks should provide fast feedback. Do not duplicate an authoritative
  remote full gate locally unless the diff's risk requires it.

### Planning and coordination

- Keep the GitHub Project centered on demonstrable product epics, the active
  slice, the next few slices, and genuine blockers. Incidental implementation
  tasks do not belong on the roadmap unless they materially affect delivery.
- Keep at most two items `In Progress`: the active product slice and one
  qualifying blocker. Update the Project when work starts, a PR becomes
  reviewable, a blocker changes, or work merges; do not perform ceremonial
  evidence updates between those transitions.
- Give each subagent one bounded deliverable and explicit file or subsystem
  ownership. Use one implementation pass and one final risk-focused critic;
  avoid overlapping audits. Interrupt or reassign an agent that is not producing
  useful progress while continuing independent critical-path work.
- Upstream make-app work is tracked separately and may interrupt HourPaths only
  when it blocks the active slice. Hand a confirmed make-app defect to a
  subagent; do not expand the HourPaths slice to improve the template.
- Batch noncritical dependency maintenance outside the product critical path.

### Throughput evidence

- Track issue-to-`main` lead time, branch age, CI wall-clock and reruns,
  infrastructure-versus-product effort, and user-visible slices shipped.
- Use these measures to remove repeated delay, not as additional delivery
  ceremony. A normal slice should reach user verification and `main` the same
  day when its inherent scope permits.

## Architecture

- Always use hexagonal architecture with explicit domain, application, port,
  adapter, and bootstrap boundaries.
- Domain code must not import HTTP, persistence, OIDC, SpiceDB, framework, or
  observability implementations.
- Use dependency injection. Keep command entrypoints thin.
- Organize HTTP adapters domain-first; separate routes, DTOs, and mapping when a
  surface grows beyond trivial behavior.
- GORM models are infrastructure mappers. Domain entities never persist themselves.
- User-owned repository reads require a user ID. Unscoped reads are a security smell.

## Specifications and documentation

- Product specs live in domain directories as `specs/<domain>/*.spec.md`.
- Cross-cutting engineering specs live under `specs/platform/`.
- Code and docs follow specs. Resolve disagreements by updating the spec first.
- Keep human documentation concise, verified, and focused on useful workflows.
- Write requirements with normative language: **must**, **must not**, **should**,
  and **may**. Use **must** only for behavior that is required and testable.
- Each product behavior spec should define its purpose, terminology, actors,
  preconditions, behavior, authorization and visibility rules, failure and edge
  cases, and acceptance scenarios where applicable.
- Prefer concrete examples for time calculations, privacy, offline conflicts, and
  boundary conditions. Examples clarify rules but do not replace normative rules.
- Keep product requirements separate from proposed implementation. Put
  cross-cutting technical decisions in `specs/platform/`.
- Link related specs instead of duplicating rules. One spec must be authoritative
  for each behavior.
- Maintain an explicit list of open questions when behavior is not yet decided.
  An open question is not an implicit license for an implementation choice.

## Identity, authorization, and tenancy

- OIDC identities use immutable `(issuer, subject)` keys. Email is profile data.
- Authentication and authorization use separate ports.
- SpiceDB is the permission decision point for protected resources.
- Do not assume organizations or tenants. Add them only through a product spec.
- Model ownership and sharing explicitly in domain language and authorization schema.

## Security

- Pin dependencies, actions, tools, and container images. Runtime images require
  immutable digests; floating tags such as `latest` are forbidden.
- Every authentication or authorization boundary needs adversarial end-to-end
  coverage: unauthenticated, malformed, expired, wrong issuer/audience,
  cross-user, insufficient permission, dependency failure, and legitimate access.
- Fail closed and avoid leaking whether inaccessible resource identifiers exist.

## Testing

- Use TDD: failing behavioral test, smallest implementation, then refactor.
- Never use mocks. Use realistic in-memory or controlled fakes behind ports.
- Test functionality through appropriate ports and real interaction boundaries.
- Inject clocks for behavior involving time, leases, audit, expiration, or tests.

## Observability and configuration

- Use typed, domain-oriented events through injected, fan-out-capable ports.
- Do not add `print`, `println`, or ad hoc logging in application code.
- Read environment variables only in configuration/bootstrap packages.
- Validate configuration before starting network listeners or workers.

## API contracts and clients

- Use Huma-generated OpenAPI as the REST contract.
- Keep envelopes and pagination consistent.
- Generate TypeScript contracts with pinned `openapi-typescript`; access them
  through pinned `openapi-fetch` behind frontend adapters.
- CI must reject stale generated output.

## Specification workflow

1. Identify the product domain and the authoritative spec for the behavior.
2. Define terms and actors before using them in requirements.
3. Capture the primary behavior and concrete acceptance scenarios.
4. Specify ownership, visibility, authorization, failure, and edge cases.
5. Check related specs for contradictions or duplicated rules.
6. Record unresolved decisions as open questions.
7. Do not begin implementation until the relevant behavior is agreed upon.

## Interactive specification interviews

- Work breadth-first across the application before pursuing edge cases within a
  single product area.
- Ask one short, focused question at a time unless the user requests otherwise.
- Every product decision requires the user's explicit approval; an industry
  standard, platform convention, or agent recommendation is evidence for a
  choice, not authorization to finalize it.
- Ask consequential product-direction questions individually.
- Present small, low-risk decisions in compact tables when that makes them
  faster to scan. Each row must state the decision, recommended choice, concise
  rationale or applicable standard, meaningful alternatives, and whether it is
  awaiting approval.
- Keep minor-decision tables to a small, coherent batch and obtain explicit
  approval or correction before recording those decisions as settled.
- After resolving a small batch, continue the current breadth-first interview
  pass rather than using the table format to pursue one area too deeply.
- Use progressive passes:
  1. establish the purpose and major behavior of every product area;
  2. define primary workflows, roles, and permissions;
  3. define failure behavior, lifecycle transitions, and cross-domain effects;
  4. resolve calculation rules, boundary cases, and implementation precision.
- Update the coverage index in `specs/README.md` as decisions are made.
- Do not treat an unanswered lower-level question as a reason to delay coverage
  of another major product area; record it as an open question and continue.
- Return to deeper questions systematically so open behavior is not forgotten.

## Implementation workflow

1. Select one user-demonstrable slice and move only it to `In Progress`.
2. Confirm or update its authoritative spec without expanding adjacent scope.
3. Write the closest failing behavioral test with controlled fakes.
4. Implement the smallest end-to-end change through the required ports and
   adapters.
5. Add adversarial tests only for security and authorization boundaries touched.
6. Run focused checks while developing and affected integration gates at changed
   boundaries.
7. Make the slice available in the warm verification environment.
8. Run the applicable full merge gate once on the final candidate.
9. Commit atomically with Conventional Commits and open the PR. Because this
   private repository cannot currently enforce server-side required checks,
   never use raw `gh pr merge` or auto-merge; merge only through
   `scripts/merge-checked-pr.sh PR_NUMBER EXPECTED_HEAD_SHA` after its immutable
   five-check gate passes.
10. Update the Project at the meaningful transition and begin the next slice.

<!-- BEGIN MAKE-APP BASELINE GUIDANCE -->
# HourPaths Engineering Guidance

This is a spec-driven, security-conscious application. Specifications under
`specs/` are the source of truth. Update the relevant `*.spec.md` before coding.

## Architecture

- Always use hexagonal architecture with explicit domain, application, port,
  adapter, and bootstrap boundaries.
- Domain code must not import HTTP, persistence, OIDC, SpiceDB, framework, or
  observability implementations.
- Use dependency injection. Keep command entrypoints thin.
- Organize HTTP adapters domain-first; separate routes, DTOs, and mapping when a
  surface grows beyond trivial behavior.
- GORM models are infrastructure mappers. Domain entities never persist themselves.
- User-owned repository reads require a user ID. Unscoped reads are a security smell.

## Specifications and documentation

- Product specs live in domain directories as `specs/<domain>/*.spec.md`.
- Cross-cutting engineering specs live under `specs/platform/`.
- Code and docs follow specs. Resolve disagreements by updating the spec first.
- Keep human documentation concise, verified, and focused on useful workflows.

## Internationalization

- Internationalization is mandatory, not an optional feature.
- All user-facing copy in web and mobile clients must come from the shared
  `packages/i18n` locale catalog, including labels, buttons, placeholders,
  validation feedback, empty states, errors, accessibility text, and notices.
- Do not place literal user-facing strings in Svelte or JSX/TSX. Product names,
  external data, and non-user-facing diagnostic identifiers are not copy.
- Add every key to every supported locale in the same change. Preserve matching
  interpolation parameters and plural forms across catalogs.
- Use the shared translator for locale negotiation, fallback, interpolation,
  plurals, numbers, dates, and times. Do not build presentation sentences in APIs.
- API errors must expose stable machine-readable codes and structured data;
  clients own their localized presentation.
- Run the i18n structural gate whenever client copy or locale catalogs change.

## Identity, authorization, and tenancy

- OIDC identities use immutable `(issuer, subject)` keys. Email is profile data.
- OIDC tokens are accepted only at session exchange. Ordinary routes accept only
  rotating opaque application sessions; never store or reuse provider tokens.
- Session rotation must preserve the original absolute session-family expiry;
  never implement indefinitely renewable sliding sessions.
- Authentication and authorization use separate ports.
- SpiceDB is the permission decision point for protected resources.
- The API runtime credential must go through the typed SpiceDB capability proxy.
  Never give the API process or its credential schema-write capability.
- Do not assume organizations or tenants. Add them only through a product spec.
- Model ownership and sharing explicitly in domain language and authorization schema.
- Invitation administration is authorized by persisted administrator status
  derived from exact configured issuer/subject bootstrap identities, never email.
  Invitation consumption and user provisioning remain one transaction.

## Security

- Added-domain services are generated with typed injected dependencies and a
  fail-closed authorization-policy placeholder. Replace that placeholder only
  after specifying policy and writing adversarial boundary tests. Never bypass
  authentication, SpiceDB, audit behavior, or the generated repository port to
  make a newly registered route return data.
- Keep `apps/api/internal/generated/domains.go` generator-owned. Domain policy
  belongs in `internal/app/<domain>`; the generated registry owns only adapter
  construction and Huma route composition.

- Pin dependencies, actions, tools, and container images. Runtime images require
  immutable digests; floating tags such as `latest` are forbidden.
- Use Red Hat Hardened Images wherever the catalog supplies a compatible build
  or runtime component. Pin an immutable release tag and multi-platform digest,
  review its SBOM and CVE report, and verify Red Hat's signature when updating
  it. Use a reviewed upstream image only when no compatible HI component exists.
  Selecting a FIPS-tagged image alone does not establish FIPS compliance.
- Every authentication or authorization boundary needs adversarial end-to-end
  coverage: unauthenticated, malformed, expired, wrong issuer/audience,
  cross-user, insufficient permission, dependency failure, and legitimate access.
- Fail closed and avoid leaking whether inaccessible resource identifiers exist.
- Run the fourteen-day dependency-age gate for every dependency change. Exact,
  reviewed exceptions require a spec update and `dependency-age-allowlist.json`
  entry with a reason and compensating verification.

## Audit

- Audit history is a first-class domain boundary, not a substitute for logs.
- Every authenticated domain read, list, state change, and denied authorization
  decision must emit a structured audit event.
- Successful state changes and their audit events must commit atomically. Never
  return success for an unaudited mutation.
- Audit records are append-only. Do not add ordinary update or delete behavior.
- Never place tokens, credentials, request bodies, or unrestricted metadata in
  audit events.
- Audit queries must preserve actor/owner visibility and must have adversarial
  cross-user tests.
- Audit-producing interaction points must use the configured principal limiter;
  do not add an endpoint that can generate unbounded audit writes.
- Runtime database credentials must never own migrations or receive audit
  update, delete, or truncate privileges.
- Audit retention uses only the separately credentialed bounded retention
  adapter. Never grant its delete capability to the API runtime or bypass the
  immutable retention summary.

## Testing

- Use TDD: failing behavioral test, smallest implementation, then refactor.
- Never use mocks. Use realistic in-memory or controlled fakes behind ports.
- Test functionality through appropriate ports and real interaction boundaries.
- Inject clocks for behavior involving time, leases, audit, expiration, or tests.

## Observability and configuration

- Use typed, domain-oriented events through injected, fan-out-capable ports.
- Preserve W3C trace context and emit domain probes to configured OTLP exporters;
  synthetic identifiers are only a fallback when export is disabled.
- Do not add `print`, `println`, or ad hoc logging in application code.
- Read environment variables only in configuration/bootstrap packages.
- Validate configuration before starting network listeners or workers.
- Request and audit rate limiting must remain coordinated across replicas through
  the PostgreSQL adapter; process-local limiting is test/single-process only.

## API contracts and clients

- Use Huma-generated OpenAPI as the REST contract.
- Keep envelopes and pagination consistent.
- Generate TypeScript contracts with pinned `openapi-typescript`; access them
  through pinned `openapi-fetch` behind frontend adapters.
- CI must reject stale generated output.

## Workflow

1. Update the relevant spec.
2. Write the failing test with fakes.
3. Implement through ports and adapters.
4. Add adversarial security tests for boundaries touched.
5. Run focused checks during development and the risk-appropriate full merge
   gate once on the final candidate.
6. Update the roadmap only when focus, sequencing, evidence, or blockers change.
7. Run one project-scoped `code-critic` pass on the completed slice, limited to
   confirmed P0/P1 merge blockers; record lower-severity follow-up without
   interrupting delivery.
8. Commit atomically using a Conventional Commit message when asked.

## Native mobile delivery

- Do not call an Expo export a native build. Preserve separate export, clean
  prebuild, Android Gradle, iOS Xcode, development-client, and signed-release gates.
- Keep callback scheme, iOS bundle identifier, Android package, application
  version, and platform build numbers explicit and validated.
- Never commit signing credentials or permit release commands to consume local
  development endpoints.
- Treat mobile session classification as security-sensitive. Transient network,
  rate-limit, and service failures retain a locally valid credential; only expiry,
  explicit rejection, revocation, or unreadable secure storage removes it.
- Restore mobile secure storage independently of OIDC discovery. Discovery may
  gate interactive sign-in and code exchange, never cold offline entry with an
  otherwise valid application session.
- Production web images default to fail-closed production mode. Never restore
  localhost fallbacks or permit absent, local, credentialed, or non-HTTPS API and
  OIDC settings outside an explicitly selected development environment.
- Framework-independent client orchestration belongs in `packages/client-core`;
  Svelte and React Native presentation models remain separate.

## Custom agents

- Project-scoped Codex agents live under `.codex/agents/`.
- The read-only code critic owns post-implementation review for security,
  architecture drift, weak tests, unsafe configuration, supply-chain ambiguity,
  and spec/code disagreement.
<!-- END MAKE-APP BASELINE GUIDANCE -->
