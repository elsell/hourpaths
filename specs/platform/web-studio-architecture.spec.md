# Studio web architecture

Status: Approved implementation direction, September 29, 2026

## Boundaries

The replacement web client must use React, TanStack Router for typed browser
navigation, and TanStack Query for server-state synchronization. Dependency
versions must be pinned and pass the existing age and security checks.

Each bounded context (identity/session, Paths/tracking, social, analytics,
preferences) must separate domain, application, ports, adapters, presentation,
and bootstrap responsibilities.

- Domain models and calculations must be framework-independent and must not
  import React, TanStack, HTTP, generated schemas, or API-client models.
- Application use cases must depend on explicit client-owned ports and domain
  models. Dependencies, clocks, and event sinks are injected.
- API adapters alone translate generated request/response DTOs into client
  models, validate required data, normalize errors, and enforce ownership.
- Generated OpenAPI contracts and openapi-fetch remain the transport contract;
  raw fetch calls in components and duplicated handwritten API DTOs are forbidden.
- Presentation consumes view models and commands. It must not cast API responses
  into client entities or import generated response types.
- TanStack hooks and route loaders are presentation/infrastructure orchestration,
  not domain services. Bootstrap composes transport, session, ports, use cases,
  query client, and router.
- Shared client-core code may be reused only where its dependency graph respects
  these boundaries; existing API-coupled types must not leak through re-exports.
- Cache keys must include account identity and query scope. Sign-out/account
  replacement must cancel and clear owned queries and mutations. Token rotation
  for the same account must not reset navigation or mix old/new account data.
- Existing OIDC PKCE, application-session exchange/rotation/revocation, runtime
  configuration validation, CSP, and security headers must survive migration.

## Design system and delivery

Shared web components and tokens own navigation, fields, actions, Path identity,
progress, reactions, and connected timelines. Domain-specific views compose these
primitives without embedding persistence or HTTP calls.

An isolated replacement entry point may coexist with the current application
during delivery. Cutover must preserve supported deep links and complete existing
user workflows; remaining legacy presentation and compatibility code must then
be removed. Existing mobile behavior must not depend on web framework choices.

Verification must focus on adapter mapping, real API interactions, account
isolation, and browser workflows. Visual review must compare rendered pages to
the approved Studio references at desktop and narrow widths. Structural checks
must enforce the dependency direction without freezing component source text.
