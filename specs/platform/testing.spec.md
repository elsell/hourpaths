# Testing and Delivery Specification

Tests use fakes rather than mocks and validate behavior through ports. Every auth
boundary covers missing, invalid, expired, wrong-issuer, wrong-audience,
cross-user, dependency-failure, and legitimate cases where applicable.
Production-broker conformance must use Google and Apple claim fixtures to prove
provider routing, cross-client subject stability, provider-distinct subjects,
and rejection of wrong issuer, audience, authorized party, and unverified-email
authority. Fixtures must include a missing Apple `name` and an Apple private-relay
email without treating either as an identity or linkage signal. Staging acceptance
must cover both providers on web and mobile, broker key rotation and unavailability,
and must prove that no broker-side email auto-link occurs. A deterministic
repository check preserves the provider-neutral production contract while keeping
Dex local and CI only.
Audit tests prove atomic mutation/event commits, append-only database enforcement,
successful read and command coverage, denied-decision coverage, actor/owner
visibility, cross-user isolation, stable pagination, and persistence across a
complete stack restart. Persistence acceptance follows cursor pagination until it
finds the pre-restart audit event; it must not depend on a default page being large
enough for the accumulated acceptance history.
Account lifecycle tests prove self-deactivation is configuration-gated, atomically
disables the account, revokes all of its sessions, writes audit history, and
rejects every old session and later OIDC exchange.

Pre-commit and CI run Go formatting and tests, structural checks, OpenAPI/client
drift checks, TypeScript checks, and production builds. Dependencies, CI actions,
toolchains, and images are pinned. Generated projects must pass checks immediately
after bootstrap without manual source edits.
Git hooks must clear hook-owned Git directory, worktree, index, and prefix
overrides before tests create nested fixture repositories; fixture commits must
never pollute or replace the caller's staged index.
Mobile verification names its evidence precisely. Fast checks run Expo Doctor,
Expo dependency compatibility, JavaScript bundle export, configuration/identifier
validation, and clean iOS/Android prebuild. Linux CI compiles an unsigned Android
debug application with Gradle. Every iOS prebuild, dependency-resolution,
simulator-validation, and iOS-specific check runs on the GitHub-hosted `macos-26`
image; Linux jobs must not perform iOS work. The macOS job may report a successful
no-op for a change that the fail-safe classifier proves is not native-affecting,
but every native-affecting change must run `make mobile-build-ios` and compile the
simulator application with Xcode. Export success alone is never reported as a
native build. Session-state tests prove that transient network, 429, and 5xx
failures preserve a locally valid credential, while expiry, explicit 401, and
malformed secure storage remove it.
The mobile provider adapter test proves discovery network failure becomes
controlled unavailable state without an unhandled promise rejection.
Shared web/mobile refresh adapter tests cover network loss, 401, 429, 503,
successful rotation followed by profile-read failure, credential retention or
disposal, bounded retry, and expiry enforcement.
Production mobile configuration tests reject missing, non-HTTPS, loopback, and
localhost endpoint values in the actual Expo bundle environment.
First-party GitHub actions are pinned by full commit SHA to maintained releases
whose action runtime is Node 24 or newer; generated CI must not emit deprecated
action-runtime warnings. Release attestations use the maintained unified GitHub
attestation action, not deprecated compatibility wrappers, and disable optional
organization-only artifact storage records for personal-repository compatibility.
Release tooling, including EAS CLI, is installed from the same frozen pnpm
lockfile and is included in dependency-age and vulnerability gates.
The locked EAS executable retains the mobile app as its working directory.
CocoaPods is Bundler-locked, Android CI selects an exact JDK patch, and signed
EAS profiles select reviewed named Expo SDK 55 build images.
Hosted iOS compilation runs on the explicit GitHub `macos-26` image because
Expo SDK 55 requires Xcode 26 or newer.
All Linux CI jobs use the GitHub-hosted `ubuntu-24.04` image, including changes,
verification, live browser acceptance, and Android native compilation. The iOS
job uses the GitHub-hosted `macos-26` image. The workflow must not select
self-hosted runners or private runner labels. This keeps pull requests from
forks and the public repository on the same reproducible runner classes and
removes any dependency on privately managed runner state.
The verify job must set `CGO_ENABLED=1` explicitly so inherited service
configuration cannot disable race instrumentation. Its 45-minute job timeout
must preserve cleanup margin beyond a complete observed verification run.
Android jobs must select Java `17.0.15+6` and fail closed unless the hosted
runner exposes Android platform `android-36`, build tools `36.0.0`, React Native
NDK `27.1.12297006`, AGP default NDK `27.0.12077973`, CMake `3.22.1`, and Ninja.
The hosted check must verify the executable compiler/toolchain paths before
Gradle runs. Ubuntu packages track the hosted image's security-maintained state;
downloaded non-Ubuntu tool archives remain checksum-pinned.
Every locked Ruby gem participates in dependency-age and OSV vulnerability
checks. The normal generated check starts the real locked EAS executable.
Reviewed overrides raise vulnerable EAS transitive parsers and matchers to fixed
versions; the locked CLI must pass version startup, Expo Doctor, and audit after
any override change.
The initial locked EAS 21.0.2 toolchain has exact, reviewed age exceptions for
`eas-cli`, its matching `@expo/eas-build-job`, `@expo/eas-json`, `@expo/steps`,
`@expo/logger`, `@expo/turtle-spawn`, and resolved `multipasta`. Compensating
verification includes the frozen graph, fixed security overrides, clean audit,
CLI startup, Expo Doctor, prebuild, and hosted native compilation.
Migration acceptance must freeze the prior released migration set with reviewed
checksums, apply it first, then run the complete current migration set and verify
preserved baseline data and new schema objects. The upgrade fixture advances
explicitly through version 14, the version-15 authorization outbox attribution
backfill, version 16 resource persistence, and the checksum-frozen version-17
Path release before applying version 18 as current. It must prove the resource
persistence schema and runtime DML privileges at version 16, the Path and
membership schemas, bounded Path DML, and SELECT-only membership privileges at
version 17, the authorization outbox ordering and recovery schema at version 18,
a clean version-18 ledger, and an idempotent current-migrator rerun. Real
PostgreSQL adapter
acceptance must exercise resource create, update, and delete rejection so an
unaudited mutation cannot commit any resource, outbox, idempotency, or audit
state.
Because version-17 producers did not establish a trustworthy database order,
upgrading across version 18 must require every authorization outbox row to be
completed first; it must never guess an order or rewrite pending TOUCH/DELETE
history. The migrator must reject this condition before dirtying the migration
ledger, and migration 18 must repeat the guard to close the check/apply race.
The migration must hold a transaction-scoped table lock that conflicts with
version-17 inserts from before its guard through trigger installation.
Staged PostgreSQL acceptance must prove refusal with skewed pending TOUCH then
DELETE rows, a clean version-17 ledger after refusal, and successful version-18
application only after those rows are drained.
PostgreSQL adapter acceptance also verifies that session rotation revokes the old
credential atomically and caps the replacement at the original family deadline.
It must also prove that skewed producer clocks cannot reverse same-resource
TOUCH/DELETE order, lease fencing uses PostgreSQL time, retries stop at the
configured dead-letter bound, and a cross-user relationship subject cannot list
or requeue the resource owner's recovery item.

The release gate runs Go vulnerability analysis and the package-manager audit
against the resolved dependency graph. CI has least-privilege permissions,
bounded runtime, and cancels superseded work. High or critical dependency
findings at any severity fail delivery rather than being silently accepted.
CI runs the live Compose acceptance harness. A pinned Playwright/Chromium browser
must operate Scalar itself: authorize through Dex with PKCE, then send authenticated
Try It requests to `/v1/me` and a protected resource endpoint. Protocol-only
reconstruction is supporting evidence, not a substitute for this browser boundary.
The API must serve Scalar's exact reviewed browser runtime from its own embedded
assets; live documentation and acceptance must not depend on a third-party CDN.
An offline release gate must verify the embedded runtime's version, byte length,
cryptographic digest, and license attribution before it can ship.
The harness permits a bounded UI retry while Scalar applies a successfully exchanged
credential, but must fail if Scalar never attaches the bearer credential.
Every harness invocation has a unique Compose project and performs project and
volume cleanup both before setup and on exit. Interrupted, stale, or concurrent
runs must not share migration state or persistence fixtures.
Exact-SHA Docker-host verification requires an available, bootstrapped BuildKit
builder before acceptance starts. Compose and the final standalone API and web
image builds reuse that builder's cache; both standalone images are loaded into
the local Docker engine for inspection. Missing or unusable BuildKit capability
fails verification with an actionable error and must never fall back to Docker's
legacy builder.
Docker-host cleanup must refuse before removing temporary directories, caches, or
Docker data whenever an `hourpaths-verify.*` owner marker identifies a live
isolated verifier. Dead owners, malformed markers, legacy live markers, and fresh
unmarked directories retain their existing scoped cleanup behavior. Verification
holds a shared host-maintenance lock before creating its temporary directory;
cleanup holds the corresponding nonblocking exclusive lock across inspection and
every mutation, closing the startup race between marker discovery and pruning.
Each added domain carries an adversarial HTTP composition test proving its
generated routes are registered, missing or invalid sessions return 401, and a
valid session receives 503 until an explicit authorization policy replaces the
fail-closed scaffold. The test exercises the real Huma route boundary rather
than substituting a direct service-only assertion.
It also proves an authentication dependency failure is not rewritten as an
invalid credential. Multi-domain generator tests include domain names whose
separator-stripping title forms would otherwise collide in Huma.
The browser also opens the web client with `es-ES`, verifies negotiation to the
supported `es` locale, checks the document language, and observes Spanish catalog
copy. Static catalog checks alone are not sufficient rendering evidence.
Shared runtime tests exercise supported and unsupported locale selection,
interpolation, singular and plural forms, and locale-aware number/date formatting.
Locale-selection coverage also runs with `Intl.Locale` unavailable to represent
the supported Hermes runtime used by the native mobile client.
The Expo adapter is tested with controlled device-locale readers for supported
regional and unsupported locales. These behavioral tests run in `make check`,
the pre-commit release gate, and CI.
Playwright is exact and age-gated. Its exact package version fixes the Chromium
revision used by acceptance; the browser is not independently represented as an
npm dependency and must not be described as independently age-gated.
Go release tools and their transitive dependencies are pinned in the dedicated
`tools` module, which is covered by the same dependency-age gate.
Production images must copy every checked-in dependency patch before their
frozen package installation so the image resolves the exact reviewed graph
rather than depending on host files or failing after a patch is introduced.
The generated web development service uses the digest-pinned hardened Node 24
builder and npm so its exact pnpm package-manager pin can start without an
unpinned global install. Its Compose environment enables pnpm's
non-interactive CI behavior so a stale or host-created modules directory cannot
block first startup waiting for a terminal prompt. The service runs with the
unprivileged `MAKE_APP_UID` and `MAKE_APP_GID` (both defaulting to 1000) so it
does not leave root-owned artifacts in the bind-mounted generated repository.
The supported Make and acceptance entrypoints populate those values from
`id -u` and `id -g`, including on CI hosts whose user is not UID 1000. Writable
temporary HOME, XDG cache, and npm prefix/cache locations make arbitrary numeric users
independent of `/etc/passwd`; release acceptance runs the image as 1001:1001.
The repository-local ignored pnpm store is shared by host bootstrap and the web
container. Live acceptance gives a cold container installation up to ten minutes
to become ready on a slow registry, then fails if the web boundary is unavailable.
Once Compose is live, browser acceptance runs through Node directly; it must not
ask pnpm to reconcile the bind-mounted dependency tree during the live test.
Browser acceptance drives the generated SvelteKit client through Dex sign-in,
the callback code exchange, application-session exchange, and authenticated
profile rendering. An unauthenticated localization-only render is insufficient.
Frontend session adapters validate exchanges before persistence, retry only
retryable failures, and preserve authenticated-offline presentation state. A
mobile orchestration test cold-starts with a valid stored credential while OIDC
discovery and the API are unavailable, and proves restoration completes in the
authenticated-offline state without deleting the credential.
The web runtime configuration tests reject absent and unsafe production API,
issuer, and client settings. Container acceptance proves the production image
fails before serving without them while local Compose explicitly selects the
development contract.
PostgreSQL adapter integration tests run while the API service is stopped so its
authorization outbox worker cannot consume test fixtures concurrently. The
harness restarts the API and re-establishes readiness before exercising live HTTP
boundaries.
Both example and blank live harnesses stop SpiceDB and assert that `/readyz`
fails closed with 503 while `/livez` remains a dependency-independent 204.

npm packages and Go modules must be at least fourteen days old. The age gate
fails closed when registry metadata cannot be retrieved or parsed. A reviewed
exception must pin one exact ecosystem, name, and version in
`dependency-age-allowlist.json`, state why it is required, name compensating
verification, and be recorded in this specification. Lowering the global age
threshold is not an acceptable shortcut.
Metadata requests have bounded connection and total timeouts; unavailable
registries fail the gate closed instead of hanging local hooks or CI.
Broad transitive ranges are constrained when necessary to keep resolution
reviewable and age-compliant. The React Native/Jest 29 schema edge pins compatible
`@sinclair/typebox 0.27.8` instead of accepting newly published artifacts from
its broad range.

Reviewed baseline exceptions align native OIDC with the pinned Expo SDK 55
runtime: `expo-auth-session 55.0.17`, `expo-web-browser 55.0.18`,
`expo-secure-store 55.0.16`, `expo-application 55.0.17`, `expo-crypto 55.0.17`,
and `expo-linking 55.0.16`. Their compensating checks are exact pins, resolved
lockfile review, a clean vulnerability audit, Expo compatibility validation,
mobile type checking, and live OIDC/API acceptance as applicable.
The patched `brace-expansion` 1.1.16 and 2.1.2 releases have exact, reviewed
age exceptions because they remove GHSA-3jxr-9vmj-r5cp before the standard
fourteen-day observation window closes. Compensating verification includes
exact major-scoped overrides, frozen-lock regeneration, a clean package audit,
workspace tests and builds, Expo Doctor, clean prebuild, and native compilation.
The patched `nanoid` 3.3.18 release has an exact, reviewed age exception because
it removes the current nanoid denial-of-service advisories before the standard
fourteen-day observation window closes. Compensating verification includes an
exact workspace override, frozen-lock regeneration, a clean package audit,
dependency-age verification, Expo Doctor, workspace checks, and native
compilation. The build-time-only Metro `image-size` dependency remains at Metro's
compatible 1.2.1 while upstream has no patched release: GitHub reports
`first_patched_version: null` for GHSA-w3rx-r6r6-pgpr and
GHSA-5p2g-fcmc-qvqq. Only those two advisories may be ignored, only for exact
1.2.1, and only through 2026-08-21 after the 2026-08-14 review confirmed that
upstream still has no patched stable release; the gate must fail earlier when any stable
release newer than the currently unpatched 2.0.2 is published so the exception
is removed immediately.
The patched `fast-uri` 3.1.5 release has an exact, reviewed age exception because
it removes GHSA-7p8r-x3mc-p8w7 from AJV dependency paths before the standard
fourteen-day observation window closes. Compensating verification includes an
exact workspace override, frozen-lock installation, a clean package audit,
dependency-age tests, and the full CI gate.
The patched `brace-expansion` 5.0.9, `postcss` 8.5.23, and `tar` 7.5.21
releases have exact, reviewed age exceptions because they clear current
security advisories before the standard fourteen-day observation window closes.
The `brace-expansion` exception additionally preserves legacy minimatch
compatibility through a deterministic patch. Compensating verification includes
exact overrides, frozen-lock installation, a clean package audit, old and new
minimatch probes, web and mobile checks, React Native code generation, CocoaPods
installation, exact EAS CLI startup, and native compilation.
The patched `js-yaml` 3.15.1 and 4.3.1 releases remove
GHSA-5p4m-2wfm-xmqj from Expo and Jest tooling paths. They have exact, reviewed
age exceptions because the security fix is required before the standard
fourteen-day observation window closes. Verification includes exact major-scoped
overrides, a structural lockfile regression check, the dependency-age gate,
frozen-lock installation, a clean package audit, workspace tests and builds,
Expo Doctor, and native compilation.
The patched `@sveltejs/kit` 2.70.2 release has an exact, reviewed age exception
because it removes GHSA-29g2-3rmr-qm68 from the production web runtime before
the standard fourteen-day observation window closes. Compensating verification
includes an exact application pin, frozen-lock installation, a clean package
audit, focused web tests and type checking, and a production web build.

The structural gate rejects oversized handwritten Go files, mocks, ad hoc print
calls, direct SQL helpers, environment reads outside configuration/bootstrap,
floating CI action references, and container images without immutable digests.
It also rejects raw `/v1` requests in web and mobile presentation code. Generated
client and shared transport adapters are the only client-side locations permitted
to construct application API requests; public provider discovery and authorization
requests remain outside this rule.
The handwritten generated-client adapter and provider adapters must be pinned by
checked hash manifests. All other production sources beneath the API-client,
web, mobile, client-core, and i18n roots must be scanned fail-closed, including
directories whose names resemble generated output. Changes to adapters, their
manifests, the boundary checker, its structural-gate integration, or generated
contract drift enforcement require out-of-band review before merge.
The checked merge entrypoint must byte-match the copy retrieved from the current
`main` branch before evaluating a pull request, so an ordinary feature worktree
may use the policy while a pull request cannot use its own modified policy to
approve itself. A pull request that touches a protected delivery-policy path
must carry either an independently submitted GitHub `APPROVED` review or an
explicit repository-owner attestation. An independent review's commit ID must
be the exact expected head. Its body must exactly equal the merge entrypoint's
canonical versioned JSON evidence binding the repository, pull request number,
head SHA, and sorted complete protected-path set. The reviewer must not be the
pull-request author, compared case-insensitively, and must have GitHub's
`OWNER`, `MEMBER`, or `COLLABORATOR` association. Only each reviewer's latest
decisive review is authoritative; dismissal or a later change request revokes
earlier approval.

The repository-owner alternative must be an issue comment on the pull request
whose author login case-insensitively equals the owner component of the bound
repository and whose GitHub `author_association` is exactly `OWNER`. Its body
must exactly equal `HOURPATHS_PROTECTED_OWNER_ATTESTATION_V1 ` followed by the
same canonical JSON evidence. This alternative may be submitted by the pull-
request author so a sole repository owner can authorize an exceptional policy
change without manufacturing an independent identity. The complete paginated
comment history is mandatory, and the latest `OWNER` comment whose body begins
with the owner-policy prefix is authoritative; a later malformed or noncanonical
owner-policy comment revokes an earlier attestation. The owner login must equal
both the repository owner and pull-request author, the GitHub actor must be a
direct human `User` rather than an app, and any supplied issue URL must bind the
same repository and pull request. Missing, malformed, stale-head, wrong-
repository, wrong-pull-request, incomplete-path, noncanonical, wrong-author, or
untrusted-association evidence does not authorize the merge.

Protected-path classification includes both the current filename and
`previous_filename` so a rename out of a protected location cannot bypass
authorization. Missing, stale-head, self-authored independent, wrong-author
owner, untrusted, malformed, incomplete, noncanonical, or superseded evidence
must fail closed without weakening the clean-state, exact-head, or five-check
merge gates.
Every approved workspace client package must retain its exact reviewed root
export to `./src/index.ts`; package manifests are protected delivery-policy
artifacts. Production source must not import or re-export ignored test or fixture
files, even when those files remain available to the package's own test suite.
The client boundary gate must parse TypeScript and Svelte imports and runtime
bindings, follow local import graphs, reject alternate browser transports and
provider-object escape paths, and allow provider SDK imports only in the exact
hash-protected adapters. Its adversarial suite must cover aliases, assignments,
destructuring, exports, callbacks, returned closures, ambient declarations,
WindowProxy sources, computed primitives, CommonJS and dynamic imports, malformed
sources, and protected-adapter or classifier tampering.
Relative imports with runtime suffixes must follow TypeScript source substitution:
`.js` tries `.ts`, `.tsx`, `.js`, and `.jsx`; `.jsx` tries `.tsx`, `.ts`, `.jsx`,
and `.js`; `.mjs` and `.cjs` try their typed `.mts` and `.cts` sources before
runtime files. Every admitted substituted source must remain inside an approved
root and be scanned, while unrelated unsupported extensions remain fail-closed.
Generated delivery tests also enforce the reviewed Red Hat Hardened Images
boundary: Go and Node build stages, API and web runtimes, and PostgreSQL use
immutable HI references, while unsupported SpiceDB and Dex components remain
version-and-digest-pinned upstream images. Live acceptance proves that the
hardened PostgreSQL entrypoint, non-root Node runtime, and static Go runtime
preserve the generated application contract.
Generated Go files carrying the standard `Code generated ... DO NOT EDIT` marker
are exempt only from the line-length rule.
The structural gate also rejects literal user-facing text and translatable
attributes in Svelte and JSX/TSX. It validates every locale as non-empty and
key-complete against English, verifies matching interpolation parameters and
plural pairs, and runs before dependency installation as part of the normal gate.
Object property keys used only as protocol syntax, including HTTP header names,
are not copy; the gate must distinguish them from nested literal JSX expressions.

Generated-client tests must prove that authenticated Path creation preserves the
contract-generated method, JSON body, content type, and boundary-length
`Idempotency-Key`; observes credential rotation on every request; overwrites stale
caller authorization; removes caller authorization when no credential exists; and
performs no network request when credential lookup fails. Server adapter tests must
distinguish stable semantic problem codes that share a status while preserving
forbidden/not-found concealment. Shared client tests must cover known and unknown
codes, malformed and hostile bodies, status/code disagreement, catalog parity,
and equivalent web/mobile localization. Contract verification must demonstrate
that changing the Huma request or problem schema fails the checked-in OpenAPI and
TypeScript generation drift gate until both artifacts are regenerated.
The deliberate `FND10-DRIFT` acceptance fixture must prove that either checked-in
OpenAPI or TypeScript contract drift fails the same deterministic contract gate
used by local checks and CI, without mutating the caller's repository.

Routine pre-commit checks are change-aware. They always run structural,
formatting, contract-drift, and focused tests, and must run dependency-age and
resolved-graph security gates whenever a dependency manifest, lockfile, action,
tool module, or container definition changes. Full race, production-build, and
live acceptance gates remain mandatory before push and release. Release
publication may reuse successful CI evidence only when it is tied to the exact
commit SHA and fails closed if that evidence is absent.

Static Compose tests reject host networking and prove `.env` loading. Live
acceptance exercises bridge-network service discovery and the OIDC backchannel
without weakening public issuer validation. Acceptance cleanup removes its
project-scoped containers, volumes, and locally built images so repeated clean
runs do not exhaust a development or CI host.
