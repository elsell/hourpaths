# Gap-closure completion checklist

This checklist preserves the full approved October 1 audit scope. Detailed domain
specifications remain authoritative; this checklist does not create new product
choices or replace their acceptance scenarios. It tracks acceptance areas, not a promised
number of PRs. Implemented code without client and release evidence remains open.

Historical delivery gate: v0.25.0 publishes merged PRs #92–#94. Web revision
`ad40d8b` is deployed through GitOps `a08886b`, with Ready/Healthy Flux, the exact
running image and public availability verified. TestFlight 0.25.0 (245) is VALID,
IN_BETA_TESTING, and assigned to the internal group. Evidence: [release](https://github.com/elsell/hourpaths/actions/runs/36903557348)
and [Apple distribution status](https://github.com/elsell/hourpaths/actions/runs/36905986943).
Physical-device acceptance remains open in area 13.

**11 acceptance areas remain; 2 of these 13 are closed**.
This is a remaining-work count, not a claim that delivery starts from zero.
Existing Studio tracking, social/feed/profile viewing, global Stats, settings,
lifecycle/history, sharing/membership/ownership, blocking and People are credited
and retained. Release evidence below is historical unless identified as current.

## Current slice — October 9

Native synchronization feedback (area 2) is the sole active implementation slice:
confirmed deletion must replace an open stale detail screen, and archival notices
must show localized saved/discarded durations. Android acceptance demonstrated
both presentation defects while proving the underlying replay results correct.

Offline-banner persistence merged in PR #144 as `3c8732d` after all five gates;
publication is in progress. Android screen-listener lifetime PR #145 is the
qualifying native-startup blocker: all five gates and 20 offline cold launches
passed; checked merge and release remain pending.

Native route recovery merged in [PR #143](https://github.com/elsell/hourpaths/pull/143)
as `048c331` after all five candidate gates passed. Android acceptance covers
activity-to-History replacement and Back navigation. Exact API/web images and
public readiness are verified through GitOps `bae8ab0`; TestFlight is pending.

Retained-history synchronization merged in [PR #142](https://github.com/elsell/hourpaths/pull/142)
as `d9b2f20` after all five candidate gates passed. Android acceptance verifies
pending-to-saved convergence, offline edit persistence across restart, and a
Studio/native conflict preserving the later edit and both revisions. Publication
has deployed exact API/web images through GitOps `906faa3`; TestFlight upload
is in progress. Separate debug offline-startup crashes remain open;
this evidence does not close signed physical-device acceptance.

Native Live Activity row restoration merged in [PR #141](https://github.com/elsell/hourpaths/pull/141)
as `cfa403d` after all five candidate gates passed. The actual widget serializer
now retains one, two, and overflow timer rows. Production exact images and readiness are verified through GitOps `bc9a263`.
TestFlight 0.49.1 (421) is VALID, nonexpired, IN_BETA_TESTING and assigned to
the internal group (Apple status run `37969745496`). Signed-device visual
acceptance and the richer proposed design remain open in area 6.

Studio default-entry cutover merged in [PR #140](https://github.com/elsell/hourpaths/pull/140)
as `c886c79` after all five immutable candidate gates passed. Legacy Home is
removed. Real browser acceptance covers account entry, empty states, invitation
confirmation/focus restoration, manual activity CRUD/history and offline-shell
reload, goal review/removal, participant denial, visibility changes, and cross-tab
notification convergence. Exact-source production release and availability are verified; area 12 is closed.

Stats readability release 0.48.1, source `6ee7299`, is deployed through GitOps
`16b17fd`; exact running images and public readiness are verified. TestFlight
build 415 is VALID, nonexpired, IN_BETA_TESTING and assigned to the internal
testing group (Apple status run `37956001746`). Physical-device acceptance of
that build remains unverified.

User-provided iPhone screenshots now confirm Dynamic Island rendering. They also
show an expanded presentation missing Path detail, and the user rejected the bare
visual treatment. Area 6 therefore remains open. The proposed richer design awaits
approval; the screenshots do not establish which exact build was installed.

Work in the order below, with
one product slice active. Split a large area only into user-visible vertical
slices; keep those slices under the same numbered acceptance area. Do not add
adjacent polish to closure criteria. A demonstrated P0/P1 blocker may interrupt.

| Order | Area | Acceptance required to close |
| --- | --- | --- |
| 1 | Remaining Studio legacy journeys — closed | Creator-only Path visibility has authoritative privacy limits, expansion confirmation and stale-state recovery; account entry/onboarding/recovery uses the approved Studio layout and preserves existing authentication behavior. These are the two remaining legacy-journey groups, not two guaranteed PRs. Calendar grouping controls already exist in Studio; their remaining correctness/acceptance belongs to area 11. New cross-client features below stay in their owning areas. |
| 2 | Durable offline use | Supported offline tracking and activity edits survive process/device restart; retained local history is readable; causal queue replay is idempotent; timer/edit/delete, membership and archival conflicts follow the offline spec; transient failures retain credentials/work; logout, expiry and account replacement cannot leak or silently lose pending work. Demonstrate reconnect and two-client conflicts. |
| 3 | Permanent account deletion | Mobile and authenticated public web deletion show complete warnings, require explicit confirmation, revoke access, discard timers, remove personal data/relationships and owned Paths without deleting unrelated activity. Verify the specified retention deadlines and restore rehearsal that reapplies deletion records before serving traffic. |
| 4 | Provider identities and recovery | Google and Apple can each be explicitly linked and used to reach the same stable account; only a nonfinal identity can be unlinked; conflicting ownership fails safely. Dual-provider recovery proves both identities, uses email only as a hint, supports cancel/new-account choices, and never merges completed accounts. |
| 5 | Notification delivery and subscriptions | All specified channels and per-person/Path subscriptions persist across clients; event matrix includes timer starts and achievements; authorized in-app and OS push delivery works, including deduplication, access loss/deletion, foreground behavior and disabled channels. |
| 6 | Native running timers | One consolidated iOS Live Activity/Dynamic Island and one grouped Android ongoing notification represent all concurrent timers. Start/stop and lifecycle recovery work; keep-running sign-out removes surfaces without stopping timers; denied/unsupported presentation never prevents tracking. Verify signed builds on supported devices. Widgets remain excluded. |
| 7 | Profiles and social privacy | Edit profile and safely validate/re-encode/upload images; privacy changes apply atomically with correct Path effects; follower/following pagination and follower removal respect privacy and blocks. All applicable clients expose the behavior. |
| 8 | Preferences and policy lifecycle | Editable week start persists and affects the specified calendars/goals; a configurable cross-midnight unavailable period uses the saved IANA zone and suppresses OS push, not in-app records. Required policy-version reacceptance and real published links/contact work. Resolve only genuinely undecided product inputs with the owner. |
| 9 | Goal-aware reminders and timer health | Reminder planning respects actionability, goal recurrence, active timers, configured unavailable periods, frequency and bundling; unachievable-goal and long-running-timer notices follow their own rules. Controlled-clock boundary evidence and real delivery prove the behavior. |
| 10 | Safety and moderation | Authorized reporting, internal case handling, enforcement notices, appeals and public-text checks work through the specified lifecycle; privacy, blocked relationships, retention and denied actions receive applicable boundary verification. |
| 11 | Analytics completion | Path-specific contribution grid and specified calendar summaries are available in mobile and Studio; week start, historical time zones, DST, range/Path filters, complete history, accessible values and latest-date behavior follow the analytics spec. Reuse existing global Stats rather than rebuild it. |
| 12 | Studio cutover — closed | All legacy supported workflows and required account lifecycle controls are reachable in Studio; shared UI and domain/application/port/adapter boundaries meet the architecture spec; original deep links, reload/back/forward and account isolation work; Studio becomes the default web UI and legacy presentation/temporary compatibility code is removed. Compare approved references at desktop/narrow widths and deploy the exact web revision through GitOps. This is the web-replacement finish line. |
| 13 | Final device and delivery acceptance | Close the physical-iPhone encouragement-selection report and each UI-28/accessibility journey with actual device evidence; verify applicable signed Android/iOS availability, production revision/health and complete spec-to-evidence reconciliation. Every preceding area must be closed; merged code/upload receipts alone do not qualify. |

## Closed-area evidence

Area 12 closed on October 9, 2026. [PR #140](https://github.com/elsell/hourpaths/pull/140)
merged as `c886c79251b25f71146f74ba1046d210fa20cec5` after all five candidate
checks. Main CI and [release publication](https://github.com/elsell/hourpaths/actions/runs/37962881084)
passed. GitOps `0f4ceb821f3f8ceb2c0f616c8e85f50dc79f6270` deployed the exact
published API/web images, with Ready/Healthy Flux and completed rollouts. Production
root returns 307 to Studio; slash, non-slash and mutation-return URLs render
without normalization redirects. Browser checks verified hydrated sign-in,
reload, desktop/narrow widths and no exceptions; both screenshots were reviewed.
Warm and candidate acceptance cover the migrated client journeys listed above.
Cross-client feature completion and physical-device acceptance stay in their own
areas; this closes replacement of the legacy web UI.

Area 1 closed on October 2, 2026. [PR #96](https://github.com/elsell/hourpaths/pull/96)
implements creator-only visibility with privacy limits, stale-state recovery and
expansion confirmation; its web and TestFlight 0.26.0 (249) availability were
verified. [PR #97](https://github.com/elsell/hourpaths/pull/97) implements Studio
account entry, onboarding and existing recovery. Focused account/session and
adversarial boundary checks passed, together with real-provider browser sign-in,
username-conflict edit retention, policy-review refresh, activation, sign-out,
callback recovery, and desktop/narrow keyboard/layout checks. Provider linking
and final physical-device accessibility acceptance remain in their own areas.

Release blockers #98 and #99 passed their required gates. The successful
[publish job](https://github.com/elsell/hourpaths/actions/runs/36953930324) supplied
revision `f006028f48f4101df1c02cadb7d5091d0ffd1e77`, deployed by GitOps commit
`8e4457ad63d86ad56d10573bca5350c9d8112dd9`. Flux reported Ready and Healthy;
exact-image pod readiness, completed API/web rollouts and public Studio HTTP 200
were verified. Web digest: `sha256:f6fc6c8497b60f1c2e3d7786e6b30cea139d5e48d657b56a21d3322d9e5fa5d9`.
API digest: `sha256:522322c1856b7df1f2c0aa08c90f82d0ec017429b21b95d83e0a30a8290298e1`.
TestFlight 0.27.0 (259) is VALID, IN_BETA_TESTING and assigned to the internal
group, confirmed by [Apple distribution status](https://github.com/elsell/hourpaths/actions/runs/36956130006).
This is availability evidence, not physical-device acceptance. The node-forge
exception expires October 9.

Studio completion is bounded by area 1's two legacy journey groups, applicable
cross-client controls in areas 3–5, 7–8 and 11, and the single cutover gate in 12.
Other areas are deliberately scheduled between those gates; Studio polish must
not indefinitely postpone offline, account, notification or native delivery.

For each closed area record the implementing PRs, focused behavioral evidence,
merged revision, deployed image/GitOps revision or signed build, and availability
proof. Keep partial evidence with its area and decrement the count only on full
acceptance. Existing delivered work is credited, not reimplemented.

Verification policy: retain required security and merge gates. Run focused checks
for changed behavior. Repeat an exact-candidate or end-to-end flow only when a
source/dependency/runtime change creates a relevant risk; record that reason.
Do not rerun unaffected broad journeys just to obtain newer timestamps. Use CI
completion watchers; report meaningful completion/failure/actionable changes,
not unchanged polling results.

Authorities: [Studio](../experience/web-studio.spec.md), [web boundaries](web-studio-architecture.spec.md), [offline](../sync/offline.spec.md), [deletion](../accounts/deletion.spec.md), [authentication](../accounts/authentication.spec.md), [notifications](../notifications/notifications.spec.md), [reminders](../notifications/reminders.spec.md), [native clients](clients.spec.md), [profile](../accounts/profile.spec.md), [visibility](../social/visibility.spec.md), [following](../social/following.spec.md), [preferences](../accounts/preferences.spec.md), [policies](../safety/user-policies.spec.md), [moderation](../safety/moderation.spec.md), [analytics](../analytics/stats-history.spec.md), [Path details](../experience/path-details.spec.md), and [accessibility](../experience/accessibility.spec.md).


## Area 2 delivery progress

PR #100 merged as `69fa9d4` on October 2 with durable timers, retained Home/history,
clock correction, replay and account fences. Its five PR gates passed, including
native Android/iOS compilation; browser offline/reload/reconnect and subsecond
notice acceptance passed. The successful [release](https://github.com/elsell/hourpaths/actions/runs/37003991427)
published revision `69fa9d4`; GitOps `c5494a8f766190da2df331fe49194c3825168578`
was Ready/Healthy with completed API/web rollouts and public Studio HTTP 200.
TestFlight 0.28.0 (267) is VALID, IN_BETA_TESTING and assigned to the internal
tester group ([Apple status evidence](https://github.com/elsell/hourpaths/actions/runs/37011326744)).
Physical-device acceptance remains unverified. PR #101 fixes the demonstrated merge-evidence payload limit without
weakening approval or check requirements.

Home hydration merged in PR #109 as `3c7a173` after all five gates. Real Chromium verified 11 rendered Paths, nine retained history entries, and one timer read per Path without API or page errors. Two independent browser ledgers converged on the newer online edit and retained the losing offline revision, including recovery from rate limiting. Remembered sessions merged in PR #108 as `347cd27`. Native device acceptance remains an area 2 closure requirement.

Reporting from native and Studio into restricted moderation cases (area 10) has shipped, following `SAFE-REPORT-01`; enforcement and physical acceptance remain open. Quiet hours still awaits the explicit equal-endpoint decision; ordinary actionable/bundled reminders remain open in area 9. No-longer-achievable interval-goal notices and per-Path participant reminder controls (area 9) merged in [PR #134](https://github.com/elsell/hourpaths/pull/134) as `74f3f33` after all five candidate gates passed. v0.46.0 API/web is deployed through GitOps `75d4c2d`; exact ready source/images, Flux Ready/Healthy, retention image and public Studio 200/API readiness 204 are verified. iOS release `37923420558` failed its required-input check; Android internal publication `37925662502` is in progress. Implementation and focused checks pass: exact interval boundaries, recorded progress, preference CAS/replay and account isolation, concurrent once-only receipts, old-client capability filtering, and final push access/preference/timer checks. Studio-to-Android preference persistence and the real-worker notice in both clients are verified; Both clients open the correct Path. Native availability and physical-device verification remain pending. Long-running timer notices merged in [PR #133](https://github.com/elsell/hourpaths/pull/133) as `1647782` after all five gates passed. Real-worker Studio/Android notice, Path navigation and native stop with independent Studio readback are verified. v0.45.0 API/web is deployed through GitOps `bc64a54`; exact ready source/images, Flux Ready/Healthy, retention image and public Studio 200/API readiness 204 are verified. Android internal run `37916446645` succeeded and committed version code 364 with release notes. iOS release `37915802033` failed the missing Live Activity provisioning-profile input; physical-device availability is not claimed. Isolated real-worker acceptance verified the once-only notice in Studio and Android, with both opening its Path. Current-average boundaries, audit rollback, concurrent workers, capability compatibility, history access revocation and push authorization/localization pass focused checks. Physical push acceptance remains pending. Policy reacceptance merged in [PR #132](https://github.com/elsell/hourpaths/pull/132) as `2c2fc8d` after all five immutable gates passed. Isolated Studio verified publication, review, timer stop through real SpiceDB, own-account JSON export and unchanged-session resumption. Android verified document-provider save/cancel, review from Following, deletion navigation/cancel, acceptance and sign-out after a subsequent publication. Main CI and scanned publication passed; GitOps `5b034e4` deploys the exact API/web images with Ready/Healthy Flux, completed rollouts, Studio HTTP 200 and API readiness HTTP 204. Native publication, iOS sharing and physical-device availability are not yet verified. Native-tab recovery merged in [PR #131](https://github.com/elsell/hourpaths/pull/131) as `92ac24d` after all five gates passed; inactive Following/Stats tabs no longer trigger recovery while unfocused. Its v0.43.1 API/web images are deployed through GitOps `15cfbb5`; exact source and image readiness, Flux Ready/Healthy, Studio HTTP 200 and API readiness HTTP 204 passed. Android internal publication run `37895298683` succeeded, committing version code 352 with release notes; physical-device installation remains unverified. iOS archive/upload failed and is not claimed available. Android 160% text and Following → Home → Settings checks passed. The recurring quiet-hours branch is paused for the equal-endpoint product decision; distinct-time Studio/Android save/readback and concurrent-edit checks passed, but it is not released.

Editable week start merged in PR #130 as `5a636b1`. Its v0.43.0 API/web release is deployed through GitOps `22ad015`; exact source/image readiness, Flux Ready/Healthy, Studio HTTP 200 and API readiness 204 passed. Google Play internal run `37890796421` succeeded, committing version code 349 with release notes; physical-device availability is unverified. Follower/following lists and silent removal merged in [PR #129](https://github.com/elsell/hourpaths/pull/129) as `0a9ebdd` after all five required checks. Studio and Android emulator acceptance covered count navigation, cancellation, confirmed removal, reverse-follow preservation, revoked list access and cross-client readback. Its v0.42.0 API/web release was verified through GitOps `5936841`; Google Play internal run `37888313760` committed version code 346 with release notes. Tester installation remains unverified. Profile-picture editing merged in [PR #127](https://github.com/elsell/hourpaths/pull/127) as `3bcb787`; v0.41.0 API/web is deployed through GitOps `06e565a`, with exact source/images, Flux health and HTTP readiness verified. Google Play internal release 341 includes release notes; tester installation is unverified. iOS remains held by Live Activity signing prerequisites. Profile privacy merged in [PR #126](https://github.com/elsell/hourpaths/pull/126) as `405f531` after all five required candidate checks passed. API/web v0.40.0 is deployed through GitOps `2a0246c`; exact source/image readiness, Flux Ready/Healthy, Studio HTTP 200 and API readiness 204 are verified. Android internal release run `37878003570` is in progress. iOS release run `37877469491` remains blocked by the missing `APPLE_WIDGET_PROVISIONING_PROFILE_NAME` and associated Live Activity signing configuration. Development acceptance now covers confirmed native save/cancel read back in Studio, stale Studio review/reload, active and archived Path narrowing, unrelated-viewer access loss, and no automatic widening on return to public. The owner approved narrowing a public Path to followers-only on acceptance by a private recipient, with an explicit warning. Real PostgreSQL narrowing/replay and concurrent acceptance/privacy checks pass, alongside strict authorization-worker audience validation; final bounded review found no P0/P1 blocker. The final Android ownership check confirmed narrowing and creator capability through an independent Studio readback. The account-settings crash found during that check was fixed with async account-lifetime guards. Profile text editing merged in [PR #125](https://github.com/elsell/hourpaths/pull/125) as `1aeb01d` after all five candidate gates passed. Real two-session Studio checks covered save, stale conflict, retained draft, reload and cancel; Android editing updated the native identity and was read back through a separate Studio session. Real PostgreSQL checks covered replay, collision, audit rollback, cross-owner isolation and immutable runtime replay privileges. API/web v0.39.0 is deployed through GitOps `d5f60be`, with the exact source/images, Flux Ready/Healthy, Studio HTTP 200 and API readiness 204 verified. iOS publication remains blocked by the missing Live Activity extension provisioning profile; Android v0.39.0 internal publication committed version code 325 (run 37870775164). Picture uploads/removal and follower-list/removal controls remain explicit subsequent requirements in this area. Consolidated native running-timer surfaces (area 6) merged in PR #123 as `c096962` after all five candidate gates passed. Native adapters and account cleanup are implemented; Android debug assembly, both platform prebuilds, iOS bundle export and focused checks pass. Android API 36 emulator acceptance verified one consolidated notification for two timers, an advancing background clock, the two-to-one-to-zero transition, a fresh notification tap opening Home, keep-running logout removing the surface, sign-in restoring it, and cold restart with API/OIDC disconnected restoring the retained timer; this is not physical-device acceptance. Native signing/profile setup and physical-device acceptance remain open. The final critic found no P0/P1 blocker; P2 follow-up: repeated native cleanup failure currently has no user-visible feedback after logout retries. The separate Go security PR #121 merged as `ff7d789` after all five gates passed. Its v0.37.0 API/web release is deployed through GitOps `2576187`, with exact images, Flux Ready/Healthy, Studio HTTP 200 and API readiness 204 verified; TestFlight publication remains in progress. Participant interval-goal and overall-target notifications merged in PR #120 as `aeecf15` after all five checks. Real Studio acceptance covered both goal notices, read/navigation, notification deletion, and activity-deletion count convergence; API/web availability is verified in v0.37.0. Per-person and per-Path subscriptions merged in PR #119 as `bb6fa3f`. Its v0.36.0 API/web rollout is verified through GitOps `a3dee60`; TestFlight 344 is VALID, IN_BETA_TESTING and assigned to the internal group (status run 37854830329), with verified release notes. Android internal publication committed version code 305 (run 37854874920). Physical-device delivery remains unverified. All ten independent notification-channel controls and delivery enforcement merged in PR #118 as `776b081` after all five required gates; live browser verification covered persistence across two signed-in sessions. Its v0.35.0 API/web images are deployed through GitOps `2d01fe0`; exact source/image readiness, Flux Ready/Healthy, Studio HTTP 200 and API readiness 204 were verified. Native publication is verified: TestFlight v0.35.0 build 340 is VALID and IN_BETA_TESTING with the internal group assigned (status run 37845433186), its release-note verification passed, and Android internal publication committed version code 301 (run 37844546314). Physical-device acceptance is still outstanding. Recovery is deployed to API/Studio; TestFlight 0.34.0 (334) is VALID and IN_BETA_TESTING with the internal group assigned (status run 37836075308). Native provider-link cancellation merged in PR #117. Account deletion (area 3) remains open for physical-device and applicable external-retention acceptance. [PR #110](https://github.com/elsell/hourpaths/pull/110) merged as `b93b191` through the immutable checked gate on October 8. All five candidate gates passed. Migration 75 passed real PostgreSQL eligibility/privilege checks; encrypted backup replay, journal restart/concurrency, and live authorization cleanup passed. The final critic's confirmed blockers were resolved. Production deployment is verified below; native-device acceptance remains open.

Browser acceptance on candidate `069ae6b` completed approved deletion of an isolated disposable account: the review identified the intended account, confirmation removed its account/identity rows, local cleanup returned to sign-in, and reload stayed signed out. A subsequent provider sign-in showed fresh onboarding and created a different provisional account ID; the old account remained absent. This is web acceptance, not physical-device evidence.

Account deletion's API/web release is deployed in [v0.32.0](https://github.com/elsell/hourpaths/releases/tag/v0.32.0), source `47a64d5b342a16fb77b54041451b949a67b56ad3`. PR #114 fixed four newly reported dependency advisories; the owner explicitly approved the exact three remaining exceptions through October 15 and all five checks passed before checked merge. Main CI [37797549294](https://github.com/elsell/hourpaths/actions/runs/37797549294) passed; [release 37799506291](https://github.com/elsell/hourpaths/actions/runs/37799506291) published the images.

GitOps `599eb5c7ef138da4b3dc3a9c2d3fbf2cc95d8eb1` is Ready/Healthy. Both deployed source annotations and running image IDs match the release: API `sha256:f55b5bf9399189908a39f6ba59cd832a50aaf78bbd52bfe1b014c5a4f6375384`, web `sha256:22f3092b524cd41d28d9d1d10e1fc226dd83b8dbe17164cacd13f3f66bab034a`. Migration 75 is clean; public Studio returned 200 and API readiness 204. Browser acceptance verified the production public deletion warning and authenticated-entry link without deleting a production account.

The journal key and independent PVC are deployed. Startup initially failed on the NFS mount; preparing the private records directory and explicitly matching the API process user/group to journal owner 65532 resolved it. The API is healthy with that GitOps configuration. The hourly restricted-role retention CronJob is enabled; a job cloned from its deployed configuration completed successfully with zero expired records. Controlled-clock PostgreSQL expiry tests and restore replay supply deletion/expiry evidence; the production run proves the deployed credential and scheduling path, not that nonexistent expired production rows were removed.

Remaining area 3 release acceptance:
1. Complete physical-device deletion/recovery. TestFlight 0.32.0 (322) is VALID, IN_BETA_TESTING and assigned to the internal group ([Apple evidence](https://github.com/elsell/hourpaths/actions/runs/37802565748)). The release publisher read back and verified its changelog. Physical-iPhone checks were requested and remain unproven; no device acceptance is inferred from availability. Android signed bundle verification and internal publication succeeded in [workflow 37802275915](https://github.com/elsell/hourpaths/actions/runs/37802275915), committing v0.32.0 version code 284 with release notes. Device installation remains unverified.
2. Verify any applicable external operational identifier expiry and retained backups. CNPG currently configures no backup service. A new backup service is not an additional requirement. Grafana has not been established to collect identifying account fields; its retention is not a confirmed blocker without that evidence.

Keep area 3 open until those remaining requirements have evidence. The wider thirteen-area completion count is unchanged.

The manual-activity slice now has a durable account queue, immutable replay,
owner-only causal metadata, retained losing revisions, and Studio/mobile form
integration. Focused PostgreSQL acceptance covers lost acknowledgements, online
versus delayed offline edits, deletion retries, and invalid pre-membership
occurrences. Browser acceptance on warm revision `a0120a6` demonstrated offline
create/edit/reload/reconnect with unchanged operation IDs and a server-backed
saved note. Later precision and validation fixes have focused regression evidence.
PR #104 merged as `2c28dfa` after owner approval and all five candidate gates;
main CI also passed. Release v0.29.0 published that revision and GitOps
`fc6f1ea2bee06041ef440981ba6e7b9e517b36dc` was Ready/Healthy, with successful
API/web rollouts and migrations. TestFlight 0.29.0 (280) is VALID, IN_BETA_TESTING
and assigned to the internal group ([Apple status evidence](https://github.com/elsell/hourpaths/actions/runs/37058487080)).
PR #103 also merged; this release attached and read-back verified TestFlight
release notes. [Google Play internal publication](https://github.com/elsell/hourpaths/actions/runs/37058182689)
committed version code 241 with release notes; installation on a device remains
unverified. No additional acceptance area is closed.


Browser acceptance on warm web revision `e2b1f8e` and API `0308245` demonstrated
manual creation and two independent account ledgers converging on the newer
online edit after an older offline edit replayed; the losing revision remained
visible. The second browser opened the editor directly without retaining the
new Path through Home. This closes the direct-editor regression, not area 2.
Large shared fixture accounts still exhaust the read quota during concurrent
Home/history hydration; retain this as an area 2 delivery issue. Do not raise
server limits or claim large-account acceptance from the isolated conflict check.

Cold-shell browser acceptance on production-built warm web `ea6e725` restored
an offline running timer after reload, stopped it locally, opened a nested manual
activity editor directly, and replayed exactly one activity after reconnect.
Actual CacheStorage held only 19 public shell/build requests. A fresh signed-out
tab exposed no retained account, and an expired tab showed account entry instead
of Path data. Focused cache tests cover explicit server rejection, failed update
preservation and exclusion of API/provider/non-Studio requests. The shell slice
merged in PR #105 as `52e1ec1`. Main CI then found a new braces advisory with no published patched version. PR #106 merged as `a8a6380` after standing owner approval and all five checks, with the exact exception bounded through October 9; this is temporary risk acceptance, not a vulnerability fix. The shell does not close durable web session
restoration or the remaining area 2 acceptance requirements.

The remembered-session implementation is review-ready on
`codex/web-session-restoration`. Production-built warm web `2049627` with API
`0308245` passed a real browser process restart offline, preserved absolute
expiry, restored a second tab, retained local Stop through competing expiry,
and synchronized both offline stops exactly once after same-account sign-in.
A rate-limited sign-in preserved the queue; an explicit later retry completed
recovery. Logout cleared an online legacy tab and an offline Studio tab. A new
account replaced the shared session without showing the previous private Paths.
Malformed durable session storage failed closed while preserving account ledgers.
Focused tests cover queued rotation versus logout and replacement-account fences;
the two confirmed review blockers were fixed. Web tests, type checking, i18n and
the client capability boundary passed. PR #108 merged and its API/web release is
verified below; this does not close area 2. Prompt retained-Home fallback now avoids hiding cached Paths
behind 429 retries, but reducing large-account hydration traffic remains open.

Native feedback audit: Path creation/archive/leave and timer confirmations previously appeared as loose footer text; they now share an app-level overlay. All visual pending-sync indicators defer for one continuous second. Offline status uses polite, neutral presentation; errors and corrections requiring user action remain immediate and persistent. Empty history no longer implies loading. The review also found and fixed prior-account Path names surviving sign-out in confirmation state. PR #107 merged as `67ac627` after all five gates and exact-head owner attestation. Actual native screenshots, large Dynamic Type and device acceptance remain pending release.

The user approved all future PRs within this goal; exact-head protected-path attestations remain recorded and all mandatory merge/security gates remain enforced. PR #106 merged as `a8a6380`, unblocking the temporary bounded dependency exception; it does not fix the upstream advisory.

Release v0.30.0 published `a8a6380` and deployed via GitOps `d2e55dfe00faefaf895d8cad907d9801e01359b8`. Flux Ready/Healthy, completed API/web rollouts, exact running source/image verification, and public Studio HTTP 200 passed. [Release evidence](https://github.com/elsell/hourpaths/actions/runs/37097322708). Web digest `sha256:6a089d5f1c4c67a06396cd1199bd71d0099584a2e4ef9fb852106054574cb81b`; API digest `sha256:141c83ee22ba83ccc99f5643e478608b7e96bd36dbf886bcf1e7da5c3798654f`. TestFlight 0.30.0 (286) is VALID, IN_BETA_TESTING, and assigned to the internal group ([Apple status evidence](https://github.com/elsell/hourpaths/actions/runs/37098278968)); this does not claim device acceptance.

PR #107 publication was superseded by #108 at the exact-current-main guard before publishing. The combined [release](https://github.com/elsell/hourpaths/actions/runs/37099999117) published source `347cd27f871bc568a5ef264df0461d12068edc1d`; GitOps `669ab5fde8deb6fa77108c637e0ebdd4c2325fa6` deployed it. Flux Ready/Healthy, completed API/web rollouts, exact running source/image checks, and public Studio HTTP 200 passed. Web digest `sha256:348f2b3a0560b92b0b70523dea565ec7905e8ae9d1967fe0a4eb6fc6225748f9`; API digest `sha256:d98b16444c688c4ad6cb147daa6953960417659f6361801484ddfd992dce0c2f`. TestFlight 0.31.0 (292) is VALID, IN_BETA_TESTING, and assigned to the internal group ([Apple evidence](https://github.com/elsell/hourpaths/actions/runs/37101100815)). The release and release-note publication completed successfully. Physical-iPhone confirmation was requested and remains pending; availability does not prove visual acceptance. Android emulator acceptance on Paul remains unverified: the refreshed development build passed the initial JDK mismatch after using cached JDK 17, then failed downloading NDK dependencies because the root disk lacked space. Newly downloaded NDKs were removed; hosted native CI remains passing. This is not visual/device acceptance.

Home hydration release [37102060283](https://github.com/elsell/hourpaths/actions/runs/37102060283) published source `3c7a17307a5fe6dbacb1124c2cdb84e54ac9003b`. GitOps `99acf1aee33376d1124746ec53398fb4a3364e2c` is Ready/Healthy with completed API/web rollouts and public Studio HTTP 200. Running web digest `sha256:26c54ce2f3f0f348b0fb40015fa21b98a2c2448ab249703da635468ef78b3426`; API digest `sha256:f25c9da1708fbe3f13ed6d7186b250f622cbb639ab99531f14da2e67c913a48d`. Native availability for this release remains unverified.

Area 3 local verification (not released): an isolated PostgreSQL rehearsal took
an encrypted `pg_dump` before deletion, deleted through the repository operation,
exported deletion records independently, and restored the older backup with
`pg_restore`. Assertions proved the deleted account and its activity were present
before replay and absent afterward, while another participant and their activity
survived. A second replay passed. Local evidence:
`/tmp/hourpaths-deletion-rehearsal.log`; remote fixture directory
`/tmp/hourpaths-deletion-rehearsal.sZLliu`. This establishes the replay mechanism,
not production backup scheduling, external-journal completeness, authorization
convergence, or the 30-day retention bound; those remain release requirements.

Account-deletion recovery now exports accepted records directly from the encrypted
external journal without a primary database connection. Local focused checks also
cover automatic retry after database failure, progress past an independently
failing account, and fencing authentication/session issuance/rotation/activation
while deletion is pending. A missing journal remains a service error. These
changes are unmerged; production journal storage and key management, retention,
full boundary verification, and client/release acceptance remain outstanding.

Fresh-database verification on 2026-10-03 applied the complete migration chain
through version 72 with a clean ledger. Runtime/migration-role checks passed for
atomic deletion, preserved unrelated shared activity, concurrent provisioning,
fresh identity reuse after deletion, and idempotent restore. Restricted-role
retention checks passed, including denied direct audit deletion and denied
summary mutation. An encrypted pg_dump/pg_restore rehearsal exported the external
journal with no database DSN, reapplied its accepted deletion to the old backup,
preserved the survivor's data, and passed a second replay. This is isolated
rehearsal evidence; production scheduling, key provisioning, retention completion,
authorization convergence, and client/release acceptance are still required.

Account-deletion candidate verification: the complete PostgreSQL store suite passed
on a fresh migrated database after correcting fixture cleanup and assertions for
random account IDs. Warm deployment exposed a Docker journal-directory mode
mismatch; runtime rejection was retained and the Compose configuration now uses a private records subdirectory created by the
non-root application, matching the staged Kubernetes configuration.
Production secret/storage and client acceptance remain outstanding.

## Area 4 delivery progress

[PR #115](https://github.com/elsell/hourpaths/pull/115) merged on October 8 as
`cf500603c74c708fc0252bdad2d511d82cc5a7c2` after all five exact-candidate gates
passed. It adds account-bound Google/Apple linking, final-provider protection,
and shared mobile/Studio settings. Real PostgreSQL checks cover ownership,
replay, atomic audit failure, and concurrent final-provider removal. Rendered
Studio settings acceptance passed with a controlled service; this is not proof
of live Google/Apple conformance.

Release [v0.33.0](https://github.com/elsell/hourpaths/releases/tag/v0.33.0) published
that source after successful main CI. GitOps `54bd82fd0a775b1c635acbd3798a51cf994cf819`
is Ready/Healthy; running API/web image IDs and source annotations match the
published digests. Public Studio returned HTTP 200 and API readiness 204.
Migration 76 is clean. Four trusted provider labels and four audit events
committed; a subsequent rollback-only verification reported zero changes, and
the private export was removed. One active and seven provisional identities
without supported metadata were preserved; their support is unresolved.
TestFlight 0.33.0 (330) is VALID, IN_BETA_TESTING and assigned to the internal
group ([Apple status](https://github.com/elsell/hourpaths/actions/runs/37823225647));
the publisher read back and verified its release notes. Android internal
publication succeeded in [workflow 37822583347](https://github.com/elsell/hourpaths/actions/runs/37822583347), committing version code 291 with release notes. Device and real-provider
acceptance remain separate from store availability.

[PR #116](https://github.com/elsell/hourpaths/pull/116) merged as `fbae5f0`
after all five candidate gates passed. Explicit recovery now verifies both
identities, preserves consumed-proof replay protection and original session
expiry, and retires the provisional account atomically. PostgreSQL race and
audit-failure checks, signed OIDC boundary checks, shared client lifecycle
checks, and rendered Studio recovery acceptance passed. Release and actual
provider/device acceptance remain pending.

[PR #117](https://github.com/elsell/hourpaths/pull/117) merged as `fc296bf` after
all five gates passed. It invalidates pending native provider links across
authentication lifetimes, including same-owner reauthentication. A controlled
held-proof check reproduced the previous failure; eleven provider workflow
checks pass, including routine credential rotation and protecting newer attempts.
Native/device release acceptance remains open.

Recovery [v0.34.0](https://github.com/elsell/hourpaths/releases/tag/v0.34.0) is
deployed through GitOps `48cdb51e72b8e56d74b0f7f08f67b9aae1985afb`. Flux is
Ready/Healthy; API/web source annotations and running image IDs match release
`fbae5f0`. Migration 77 is clean; public Studio recovery returned 200 and API
readiness returned 204. API digest is
`sha256:e5617bab15a2e1069e1afcedfcc60c340d86600e820c1c818fcee5c175577043`,
web digest is `sha256:287bcf33c363667cc94b895a1eb022bcf15caefaac46d0f69caa128281369c87`.
The signed store build remains in progress in release workflow `37831306567`;
this deployment does not establish TestFlight, Play, or actual provider acceptance.

Remaining closure criteria:
1. Verify signed mobile availability and both providers on both clients against
   the actual broker; deployed API/web and metadata reconciliation are credited.
2. Complete explicit dual-identity recovery, including atomic session adoption,
   cancellation, concurrent onboarding/deletion, and completed-account guidance.
3. Verify account-lifecycle cancellation on mobile, including a held callback
   after same-owner sign-out/sign-in, and complete physical-device acceptance.

No acceptance area closes from this partial delivery; twelve remain.

Nonblocking notification follow-up: revalidate a delayed goal-deadline push against interval expiry and subsequently recorded goal completion before provider handoff. Current delivery already rechecks preferences, active timers and access.

### Reporting slice evidence

`SAFE-REPORT-01` now has shared Studio/native reporting on profiles, Paths,
feed events, comments, and received nudges. Real API submissions from Studio
and the Android emulator return a private acknowledgment. Studio block-review
cancellation and native cancellation followed by confirmed blocking are verified;
the reports remain retained and the blocked profile becomes inaccessible.
Fresh PostgreSQL migrations, all five target visibility checks, concurrent
idempotent retries, immutable evidence, audit rollback, runtime account deletion,
restricted operational review, and bounded closed-case retention pass focused
checks. PR #135 merged as `44cb21c` and release v0.47.0 was deployed through
GitOps `f5d2598`; exact source/image readiness and Studio/API health passed.
The subsequent v0.47.1 build 406 processed VALID with verified TestFlight notes;
tester-group availability and physical-device acceptance still need verification.

The bounded security critic found no P0/P1 blockers. Its null operational page
limit issue is fixed with a PostgreSQL regression check. Native expired block
reviews still require Cancel → Block to refresh; record an explicit refresh
control as a nonblocking follow-up. Enforcement, appeals, and public-text checks
remain separate open requirements in area 10.


### Warning notices and appeals (review-ready)

Native and Studio Settings now expose private Account notices, one immutable
appeal within 30 days, and its final decision. Isolated QA demonstrated an
operational warning, Studio submission and reload, audited operator review,
and the same reversed decision with original explanation in Android Settings.
Fresh migrations through 93 and PostgreSQL checks cover ownership, retries,
audit rollback, restricted review grants and immutable final decisions. Native
and Studio types, localization, transport boundary and Settings checks passed;
the scoped security critic found no P0/P1 blockers. Final CI, merge and release
remain pending. This does not close area 10: content removal, suspension, bans,
restricted-account appeals and automated public-text checks remain required.
