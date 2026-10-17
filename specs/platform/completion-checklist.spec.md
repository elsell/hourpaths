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

## Current delivery — Live Activity Stop and provider photo

[PR #201](https://github.com/elsell/hourpaths/pull/201) fixes a reproduced native
Stop rejection: a history or synchronization update could advance the ledger
revision while the displayed timer remained the same. Stop now reads the current
ledger under the native SQLite write lock and accepts an older display revision
only for the exact same owner, Path and running timer. Future revisions,
replacement timers, unavailable Paths and replay remain rejected; Start retains
its strict revision requirement. Shared action tests and the real macOS
SQLite/JavaScriptCore harness cover an intervening history update and its
preservation. These checks do not establish physical-device button dispatch.

All five [candidate checks](https://github.com/elsell/hourpaths/actions/runs/39030990771)
and [main checks](https://github.com/elsell/hourpaths/actions/runs/39031931089) passed.
[Release v0.64.1](https://github.com/elsell/hourpaths/releases/tag/v0.64.1) completed
signed iOS upload and release-note publication in
[run 39032803137](https://github.com/elsell/hourpaths/actions/runs/39032803137).
TestFlight **0.64.1 (581)** is VALID, nonexpired, IN_BETA_TESTING and assigned to
the internal group ([Apple verification](https://github.com/elsell/hourpaths/actions/runs/39034012774)).
Google Play internal **554** committed in successful
[run 39034005940](https://github.com/elsell/hourpaths/actions/runs/39034005940).
This receipt satisfies the owner-approved Play Console verification waiver.
The owner has been asked to verify Stop and saved elapsed time on this exact
build. No successful device result has been received; area 6 remains open.

[PR #202](https://github.com/elsell/hourpaths/pull/202) implements the approved
automatic provider-photo import. Verified OIDC picture claims are private
provisional-account data. A bounded, IP-pinned HTTPS fetch refuses redirects and
internal destinations; the existing image processor and audited revision-checked
picture write normalize and save the image. Activation clears the source and
succeeds even when optional import fails. Later sign-ins preserve removal or
replacement. The existing optional photo screen exposes both controls.

All five [candidate checks](https://github.com/elsell/hourpaths/actions/runs/39032606788)
and [main checks](https://github.com/elsell/hourpaths/actions/runs/39033446375) passed.
Signed-claim, real TLS, application and real PostgreSQL tests cover malformed
claims, unsafe retrieval, activation, source cleanup, image writes, conflicts and
audit rollback. Browser interaction at 390px and the checksum-identical native
photo components on Android at 160% text size verified imported-image display,
removal, replacement preview/save and onboarding completion. These controlled
checks do not establish real-provider or physical-iPhone acceptance.

[Release v0.65.0](https://github.com/elsell/hourpaths/releases/tag/v0.65.0) published
API/web source `5aed4b7a9b1311f71cc4f20a69c29bc94eb718ea`. GitOps
`141f6024f6c64cb2d5b3dc49b7f846357814ae4e` is applied with exact running image
digests, ready API/web pods, Flux Ready/Healthy, matching retention image and
public HTTP availability. A read-only production check confirmed clean migration
104, the validated provisional-only source constraint and enabled lifecycle
clearing trigger.

TestFlight **0.65.0 (583)** is VALID, nonexpired, IN_BETA_TESTING and assigned to
the internal group ([Apple verification](https://github.com/elsell/hourpaths/actions/runs/39035546546)).
The [release workflow](https://github.com/elsell/hourpaths/actions/runs/39034217110)
completed signed upload and release-note publication. This build also includes
the Stop correction from PR #201. Android v0.65.0 publication remains pending.

External backup-retention verification remains temporarily deferred by the owner,
not completed or guaranteed. This delivery does not close any of the eleven
remaining acceptance areas; the original requirements below remain intact.

## Guided onboarding delivery — October 17

[PR #199](https://github.com/elsell/hourpaths/pull/199) replaces the stacked setup
form with focused identity, preferences and policy-review steps on Studio and
mobile. Profiles initially select the approved social visibility. Successful
activation opens an optional local-photo task with a visible Skip action; returning
accounts proceed directly Home. Automatic provider-photo import with remove/replace shipped to the API in PR #202,
with current evidence above.

All five [candidate checks](https://github.com/elsell/hourpaths/actions/runs/39007032335)
and [main checks](https://github.com/elsell/hourpaths/actions/runs/39007971250) passed.
[Release v0.64.0](https://github.com/elsell/hourpaths/releases/tag/v0.64.0) published
source `e3766d3702a42f02a1591129ee3b8b846b4b36fe`. GitOps
`145bd641e62078bd487f09c54a0cfd4fc302290d` is verified with exact API/web
source and image digests, ready pods, Flux Ready/Healthy, matching retention image,
and HTTP 200 for onboarding and photo setup routes. A transient cluster failure
resolved without node changes. HTTP availability does not establish activation.

Controlled-port browser interaction covered retained edits, explicit consent,
photo handoff, retry and Skip, and English/Spanish layout and heading focus at
320px. Current native components passed the complete flow on Android at 160%
system text size. Focused ownership checks cover sign-out and late activation.
These results do not establish real-provider activation, physical-iPhone,
VoiceOver or production image-upload acceptance.

TestFlight **0.64.0 (576)** is VALID, nonexpired, IN_BETA_TESTING and assigned to
the internal group ([Apple verification](https://github.com/elsell/hourpaths/actions/runs/39009895501)).
The [release workflow](https://github.com/elsell/hourpaths/actions/runs/39008687449)
read back its changelog. Google Play internal build **549** committed in successful
[run 39009867474](https://github.com/elsell/hourpaths/actions/runs/39009867474).
The publication receipt satisfies the owner-approved Play Console verification
waiver; physical-device acceptance and all eleven broader areas remain open.

## Current policy delivery — October 17

[PR #197](https://github.com/elsell/hourpaths/pull/197) publishes complete Terms,
Privacy Policy and Community Guidelines through a shared, localized document
layout. All five [candidate checks](https://github.com/elsell/hourpaths/actions/runs/39002833917)
and [main checks](https://github.com/elsell/hourpaths/actions/runs/39003556999) passed.
[Release v0.63.0](https://github.com/elsell/hourpaths/releases/tag/v0.63.0) published
scanned, attested API/web images from `e9289cb80760bdbec05fcc211d66f8938a6cfb4b`.
GitOps `927fab6fb8214dcfc7fc73a028fa6ddb96a80a0a` deployed those exact images;
ready pods, matching source annotations and healthy Flux were verified.

All three public URLs returned complete documents in English and Spanish before
activation. Production browser inspection verified the readable Privacy Policy
layout. Local HTTP checks also covered rendering without JavaScript and unknown
paths returning 404; narrow-screen browser verification passed. The documents use
the private support form and do not disclose private backup arrangements.

GitOps `0cf28f47edf3fd834aeda4398ae1083821565de3` then activated policy revision
**2**, version **public-2026-10-17**. At 05:05 UTC, the publisher init completed,
the durable policy authority matched the published version and timestamp, all
six localized public responses passed, and exact API/web images and Flux
Ready/Healthy were verified. Existing policy reacceptance and timer-stop,
export and deletion exceptions remain in place. No real user's acceptance was
performed on their behalf. Signed-device reacceptance remains unverified.

Public policy implementation, publication and activation are complete. Native
v0.63.0 distribution is tracked separately from this verified web/configuration
delivery. The eleven broader acceptance areas remain open.

## Support delivery — October 17

[PR #195](https://github.com/elsell/hourpaths/pull/195) adds private support with
ULID references and separate recovery credentials, an operator inbox and replies,
and private administrator notifications for support requests, moderation reports
and appeals. Platform administration uses an immutable identity, an audited
persisted grant and SpiceDB authorization; Path or invitation administration does
not confer it.

All five [candidate checks](https://github.com/elsell/hourpaths/actions/runs/38997594822)
and [main checks](https://github.com/elsell/hourpaths/actions/runs/38998598023) passed.
[Release v0.62.0](https://github.com/elsell/hourpaths/releases/tag/v0.62.0) published
scanned, attested images from `b3ca03d9844a04f2d7bb0ab7dd7f0339f44f5ade`.
GitOps `f6a8ca335e66074bd3cbef46f0ced3eb5fd19698` is applied with exact API/web
source and image digests, ready pods and Flux Ready/Healthy. Migration 103 is clean.
The owner-requested platform-admin grant is audited and its SpiceDB outbox delivery
completed without dead-lettering. No public account identifier is needed in this record.

The public support form renders at the configured support URL. Production
submission, private recovery, wrong-key rejection and idempotent retry passed;
the clearly labeled verification request created one owner notification and one
creation audit event. Unsigned admin support/moderation requests return 401.
Real PostgreSQL/HTTP tests cover operator access, replies, reports/appeals,
revocation, expiry and audit rollback. These tests do not substitute for a
signed-in production UI or physical-device acceptance check.

TestFlight **0.62.0 (564)** is VALID, nonexpired, IN_BETA_TESTING and assigned to
the internal group ([Apple verification](https://github.com/elsell/hourpaths/actions/runs/39000769413)).
The [release workflow](https://github.com/elsell/hourpaths/actions/runs/38999548247)
read back its release notes. Google Play internal build **537** committed with
release notes in successful [run 39000790116](https://github.com/elsell/hourpaths/actions/runs/39000790116).
The publication receipt satisfies delivery under the owner's October 14 Play
Console verification waiver. The owner's physical-iPhone admin inbox check
is pending. The prioritized support/moderation/admin implementation and release
are delivered; physical-device acceptance remains explicitly uncredited. Public
policy pages were the next implementation slice and are delivered above. All eleven broader
acceptance areas remain open.

## Prior delivery — October 15

[PR #193](https://github.com/elsell/hourpaths/pull/193) fixes two additional native
Live Activity defects: local publications now change ActivityKit content identity,
and first account restoration preserves a verified same-owner lease rather than
clearing existing controls. Concurrent same-owner restoration shares one transition.
Account replacement, sign-out, expiry and deletion protections remain intact.
The earlier bounded widget runtime and initial session-restoration fixes from
PRs #185 and #191 remain included.

All five [candidate checks](https://github.com/elsell/hourpaths/actions/runs/38776443537)
and [main checks](https://github.com/elsell/hourpaths/actions/runs/38778615635) passed.
Eleven coordinator and seven real SQLite tests cover restoration, replacement,
concurrent adoption, expiry and deletion fences. The macOS harness executes native
Start/Stop and verifies changed ActivityKit content after committed mutations.
These checks establish corrected code paths, not physical-device button dispatch.

[Release v0.61.7](https://github.com/elsell/hourpaths/releases/tag/v0.61.7)
completed signed iOS upload and release-note readback in
[run 38780126899](https://github.com/elsell/hourpaths/actions/runs/38780126899).
TestFlight **0.61.7 (557)** is VALID, nonexpired, IN_BETA_TESTING and assigned to
internal testers ([Apple verification](https://github.com/elsell/hourpaths/actions/runs/38783478814)).
Google Play v0.61.7 internal **530** was committed in
[run 38783175456](https://github.com/elsell/hourpaths/actions/runs/38783175456).
The owner’s waiver of independent Play Console verification applies; publication
does not establish physical-device acceptance.

Production GitOps `1ac437f4d164bab1502ce6602fef66234802f354` deploys source
`a93a786029ae9fe1594d662f4f4ecba6205024ea`. Exact API/web images and source
annotations, ready pods, Flux Ready/Healthy, retention-image consistency and
public API readiness 204 / Studio 200 were verified. APNs configuration is unchanged.

The owner reported that Stop still failed after updating following the 0.61.6
release. No successful device result for 0.61.7 has been received. Retest 0.61.7
(557): start a timer, background the app, confirm persistent Lock Screen/Dynamic
Island content, then Stop and Start there and reopen the app to verify timer state
and saved elapsed time. If Stop still appears ineffective, distinguish a timer
still running inside the app from a stale Live Activity display. Rendering and
native command acceptance remain separate; neither is closed by successful upload.
Background goal-period refresh, multi-timer, offline and account-lifecycle behavior
also require device evidence. Onboarding redesign remains on hold. All eleven
remaining acceptance areas stay open.

At this historical checkpoint the policy and support pages returned HTTP 404.
The October 17 deliveries above replace the missing support and policy pages.
This historical 404 result no longer describes production.

## Prior delivery and remaining work — October 13

- Account suspension/ban enforcement merged in [PR #165](https://github.com/elsell/hourpaths/pull/165).
  Running timers stop and save elapsed time, as approved by the owner. Its
  TestFlight 0.55.0 (484) and Google Play internal 455 publication are verified.
- Reported nudge removal merged in [PR #166](https://github.com/elsell/hourpaths/pull/166).
  v0.56.0 API/web deployment is verified through GitOps `999b0b2`. TestFlight
  0.56.0 (487) is VALID, nonexpired, IN_BETA_TESTING and assigned to the internal
  group ([availability verification](https://github.com/elsell/hourpaths/actions/runs/38355786548)).
  The publisher also read back its release notes.
  Google Play internal build 459 is published in [run 38355776506](https://github.com/elsell/hourpaths/actions/runs/38355776506).
  Physical-device acceptance remains unverified.
- Reported feed-event removal merged in [PR #167](https://github.com/elsell/hourpaths/pull/167)
  as `e08e6e7`. All five candidate and main checks passed. v0.57.0 API/web is
  verified through GitOps `739d55a`: exact source/images, Ready/Healthy Flux,
  current retention image and public 200/204 health. TestFlight 489 and
  Google Play internal 461 are verified available, with release notes, from
  [release 38356348820](https://github.com/elsell/hourpaths/actions/runs/38356348820).
  Studio appeal submission/reversal and Android enlarged-text rendering passed;
  those results do not establish signed physical-device acceptance.

The delivered removal flows preserve source recorded time and goal achievements,
hide removed social content and notifications, suppress queued push, and use the
shared private notice/appeal flow. The trusted-content no-op moderation port
shipped in PR #164; automated classification is explicitly deferred by the owner.

Path-name removal shipped in [PR #170](https://github.com/elsell/hourpaths/pull/170)
as `1d47699`. Removal preserves shared access, members, timers and recorded history;
private notices and appeals restore the original name only when later edits,
ownership changes or deletion have not superseded it. Focused PostgreSQL boundary
checks, Studio replacement/restoration and appeal interaction, Android enlarged-text
rendering, and all five candidate/main gates passed.

Current release [v0.58.0](https://github.com/elsell/hourpaths/releases/tag/v0.58.0)
is verified in production through GitOps `32cd8b2`: exact source/images,
Ready/Healthy Flux, current retention image and public 200/204 health.
TestFlight 0.58.0 (494) is VALID, nonexpired, IN_BETA_TESTING and assigned to the
internal group ([Apple verification](https://github.com/elsell/hourpaths/actions/runs/38389306190));
release notes were read back. Google Play internal 465 published in
[run 38389237934](https://github.com/elsell/hourpaths/actions/runs/38389237934),
and Play Console shows it active, fully rolled out and available to internal
testers with the Path-name removal release note. These records do not establish
signed physical-device acceptance.

Profile field removal merged in [PR #173](https://github.com/elsell/hourpaths/pull/173)
as `2b0861a`. Selected display name, description or picture removal preserves the
profile and history, with private notices and safe appeals. Focused PostgreSQL
boundary and push-race checks and real Studio notice/appeal acceptance passed;
the final security review found no P0/P1 blocker. All five candidate/main gates passed.

[v0.59.0](https://github.com/elsell/hourpaths/releases/tag/v0.59.0) is verified in
production through GitOps `174bbf5`: exact source/images, Flux Ready/Healthy,
current retention image, Studio HTTP 200 and API readiness 204. TestFlight
0.59.0 (498) is VALID, nonexpired, IN_BETA_TESTING and assigned to the internal
group ([Apple verification](https://github.com/elsell/hourpaths/actions/runs/38429843875));
release notes were read back. Google Play internal build 470 committed in
[run 38426925098](https://github.com/elsell/hourpaths/actions/runs/38426925098).
The owner waived independent Play Console verification on October 14; the
successful publication receipt satisfies that delivery requirement.
Physical-device acceptance remains unverified. The active product slice is
repairing the owner-reported iOS 0.61.0 Home loading, offline-state flicker,
timer Start rejection, and missing Live Activity regressions. Acceptance requires
reliable cached Home entry and local Start/Stop, quiet transient status, and
restored Lock Screen/Dynamic Island presentation in a corrected signed build.
Onboarding redesign is on hold while these regressions are resolved.
Automatic Live Activity goal-period refresh merged in PR #177; v0.61.0 is
available as TestFlight build 512 and Google Play internal 486, with production
GitOps `fc8df71`. Those distribution receipts do not establish device correctness. Richer controls merged in
[PR #175](https://github.com/elsell/hourpaths/pull/175) as `83c7c57` after all five
gates; its release availability is now verified above. The acceptance areas
below remain open.

The remaining-work table below is the finish line. Existing implementation does
not close its physical-device, provider, policy or external-retention evidence.

Nonblocking operational follow-up: nudge/feed removal suppression makes delivery rows
terminal but does not refresh the push-outbox aggregate status. Reconcile that
status in a later maintenance batch; terminal delivery rows prevent resend.
Feed removal also has a reviewed nonblocking lock-order inversion with concurrent
interactions: PostgreSQL may abort one transaction safely. Align the order in a
maintenance batch without weakening account-lifecycle admission or event hiding.

## Prior delivery evidence — October 12

- Production [v0.53.1](https://github.com/elsell/hourpaths/releases/tag/v0.53.1),
  source `e8e7516`, is verified through GitOps `9da65d4`: exact API/web source and
  image IDs, Ready/Healthy Flux, current retention image and public 200/204 health.
  It includes PRs #158–#161 and #163: shared Path analytics, large-text
  corrections, zero-activity dates through local today, bounded account-history
  loading and the independent past-year Stats calendar.
- TestFlight 0.53.1 (471) is VALID, nonexpired, IN_BETA_TESTING and internally
  assigned ([Apple availability](https://github.com/elsell/hourpaths/actions/runs/38082488999)).
  The publisher read back its release notes. Google Play v0.53.1 internal build
  442 was committed after signed-bundle verification in [run 38082431881](https://github.com/elsell/hourpaths/actions/runs/38082431881).
- For account-history loading, real PostgreSQL verifies owner scope, archived
  Paths, retention boundaries and snapshot edits. Studio renders 22 activities
  using one online and one retained-history request; Android cold reopen retains
  and refreshes all 22 entries. Offline Path detail retains readable activity and
  retries its charts successfully after reconnection. These are browser/emulator
  results, not signed physical-device acceptance.
- Independent Stats calendar range merged in [PR #163](https://github.com/elsell/hourpaths/pull/163)
  as `e8e7516` after all five gates. Live browser checks verify 365 dates, 13 month
  groups, chart-range independence and shared Path filtering; Android at 160%
  text size retains the past-year calendar when the chart switches to Day.
  Main CI and exact-source API/web deployment passed. TestFlight 471 and Google
  Play 442 contain the correction; physical-device acceptance remains pending.

### Remaining work by area

The acceptance criteria in the numbered table below remain the finish line.
This summary distinguishes implementation work from missing acceptance evidence;
it does not close an area merely because a build is available.

| Area | Work or evidence still needed |
| --- | --- |
| 2 — Offline | Signed-device restart, reconnect, account replacement and conflict acceptance; PR #161 publication is verified on both stores. |
| 3 — Deletion | Physical-device deletion/recovery. External backup inventory/expiry verification is temporarily deferred by the owner. Encrypted restore and authorization replay already passed; no identifying Grafana ingestion has been established. |
| 4 — Identities | Real Google/Apple linking, unlinking and dual-provider recovery on signed clients. |
| 5 — Notifications | Physical push delivery, foreground/disabled-channel behavior, deduplication and access-loss acceptance. |
| 6 — Native timers | PR #201 is included in current TestFlight 0.65.0 (583), with prior runtime fixes, controls and APNs retained. Verify Stop and saved elapsed time on this build; then Start, background period refresh and signed-device multi-timer/lifecycle behavior. A successful retest has not yet been received. |
| 7 — Profiles | Automatic provider-picture import with remove/replace is implemented and deployed in PR #202. Real-provider import and signed-device profile/image/privacy acceptance remain. |
| 8 — Preferences | Public policy documents and version activation are verified in v0.63.0; support is deployed in v0.62.0. Guided onboarding is implemented and published across clients above; signed-device week-start, policy, quiet-period and onboarding acceptance remain. |
| 9 — Reminders | Physical reminder/timer-health delivery and final quiet-period suppression acceptance. |
| 10 — Safety | Signed-device moderation/appeals acceptance remains. Approved Profile field removal shipped in PR #173/v0.59.0, preserving the profile and history; production and TestFlight availability are verified, and Play publication is accepted under the October 14 verification waiver. Path-name removal, account restrictions, comment/nudge/feed removal and the approved no-op port are implemented. |
| 11 — Analytics | Device/accessibility acceptance for PR #163 (signed mobile distribution verified); production deployment is credited above. |
| 13 — Final acceptance | Physical-iPhone encouragement/Send confirmation, UI-28 matrix and final spec-to-release evidence reconciliation. |

## Historical delivery evidence

The following records retain each slice's original verification limits. Later
verified releases may supersede an earlier “release pending” statement; they do
not silently satisfy a still-unverified physical-device requirement.


Comment removal and appeal enforcement ([issue #154](https://github.com/elsell/hourpaths/issues/154), area 10) merged in PR #155 as `d2bd82c` after all five checks.
Production v0.50.0 is verified through GitOps `12e991b`, with exact API/web
images, Ready/Healthy Flux and public readiness. TestFlight 0.50.0 (450) is VALID,
IN_BETA_TESTING and internally assigned; release notes were read back. Google Play
internal version code 420 was committed in run `38002647767`, after signed-bundle
verification. An operational removal must hide the comment across
social reads and deliveries and expose the existing private notice/appeal flow.
[PR #155](https://github.com/elsell/hourpaths/pull/155) implements removal,
private notices and appeal reversal. Real PostgreSQL checks cover hidden reads,
counts/retries, competing edit and push transactions, and ordinary deletion.
Studio acceptance verifies the affected-comment reference, appeal submission
and persisted reload at desktop and narrow widths. Android at 160% font size
shows the same notice and appeal, then the final reversal after Refresh.
Signed physical-device acceptance remains outstanding.

Bundled goal reminders and quiet periods merged in [PR #153](https://github.com/elsell/hourpaths/pull/153)
as `90b41af` after all five candidate checks. The approved rule rejects equal
start/end times when quiet hours are enabled, with localized correction and no
saved mutation. Scheduling, bundles, online/offline Stop recalculation,
account-scoped settings and final push suppression are implemented. Migration 94
is preserved; reminder and quiet-period migrations are 95–96. Focused PostgreSQL
checks and the full 149-test store/integration run passed, along with Studio and
Android at 160% text acceptance for the equal-time correction and disabled Save.

Release v0.51.0 is deployed through GitOps `3603732`: exact API/web images,
Ready/Healthy Flux, retention image and public readiness are verified. TestFlight
0.51.0 (453) is VALID, nonexpired, IN_BETA_TESTING and assigned to the internal
group ([Apple status](https://github.com/elsell/hourpaths/actions/runs/38047918135));
its release notes were published and read back. Google Play internal version code
423 was committed after signed-bundle verification in
[run 38047890528](https://github.com/elsell/hourpaths/actions/runs/38047890528).
Physical-device reminder and quiet-period delivery remain open in areas 8, 9
and 13; publication does not close those acceptance areas.

Release v0.51.1 contains PR #157's complete-range Stats
calendar fix. Production source `efc6de9` is deployed through GitOps `0fcf421`;
exact API/web images, Ready/Healthy Flux, retention configuration and public
readiness were verified. TestFlight 457 is VALID, nonexpired, IN_BETA_TESTING and
assigned to the internal group ([Apple status](https://github.com/elsell/hourpaths/actions/runs/38069861821));
release notes were published and read back. Google Play internal version 427 was
committed after signed-bundle verification in [run 38069812415](https://github.com/elsell/hourpaths/actions/runs/38069812415).

Path-specific analytics merged in [PR #158](https://github.com/elsell/hourpaths/pull/158)
as `e9c3f3c` after all five candidate gates. Path and Stats views reuse shared
chart/calendar components and cross-client date calculations. Personal-first
participant selection scopes lifetime metrics and recent activity; the calendar
covers every date in its trailing-year range. Studio and Android emulator
acceptance, scoped PostgreSQL/HTTP checks, and full browser lifecycle acceptance
passed. CI found stale Home goal fields after detail edits; the corrected
candidate patches cached goal fields and passed goal-removal acceptance.
Main CI [38071454749](https://github.com/elsell/hourpaths/actions/runs/38071454749)
passed. Release [v0.52.0](https://github.com/elsell/hourpaths/releases/tag/v0.52.0)
is deployed through GitOps `06908ac`: exact-source API/web images, Ready/Healthy
Flux, retention configuration, Studio HTTP200 and API readiness204 were verified.
TestFlight 0.52.0 (460) is VALID, nonexpired, IN_BETA_TESTING and assigned
to the internal group ([Apple status](https://github.com/elsell/hourpaths/actions/runs/38073747812));
release notes were published and read back. Google Play internal build 430 was
committed after signed-bundle verification in
[38073279818](https://github.com/elsell/hourpaths/actions/runs/38073279818).
Area 11 remains open: current-year Stats initially scrolls to future blank dates
instead of today, and native physical-device acceptance remains unverified. Shared mobile analytics large-text corrections merged as `28a8520` through all five gates in
[PR #159](https://github.com/elsell/hourpaths/pull/159): labeled range/grouping
choices and scaled calendar geometry passed Android 160% interaction checks.
High-Path-count Home history can reach its retry state under HTTP 429;
that separate recovery case remains in area 13's acceptance work.

Offline-banner persistence, Android screen-listener lifetime, and native
synchronization feedback merged in PRs #144–#146 through their five-check gates.
Combined source `f27cd1f` is deployed through GitOps `b26e1fc`, with exact
API/web images and Ready/Healthy Flux verified. TestFlight 0.49.5 (434) is VALID,
IN_BETA_TESTING and assigned to the internal group (status run `37984186271`). Android acceptance covers 20 offline
cold launches, edit/deletion and archival conflicts, membership removal, and a
newer Studio timer superseding an older native timer with one saved result and
notification cleanup. These checks do not close signed-device acceptance.

Native route recovery merged in [PR #143](https://github.com/elsell/hourpaths/pull/143)
as `048c331` after all five candidate gates passed. Android acceptance covers
activity-to-History replacement and Back navigation. Exact API/web images and
public readiness are verified through GitOps `bae8ab0`. TestFlight 0.49.3 (426)
is VALID, IN_BETA_TESTING and internally assigned (status run `37977765960`).

Retained-history synchronization merged in [PR #142](https://github.com/elsell/hourpaths/pull/142)
as `d9b2f20` after all five candidate gates passed. Android acceptance verifies
pending-to-saved convergence, offline edit persistence across restart, and a
Studio/native conflict preserving the later edit and both revisions. Publication
has deployed exact API/web images through GitOps `906faa3`; TestFlight upload
completed: TestFlight 0.49.2 (424) is VALID, IN_BETA_TESTING and internally
assigned (status run `37974613215`). Subsequent PRs #144–#146 address the
reported offline-startup and listener failures. Signed-device acceptance remains open.

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

Reporting from native and Studio into restricted moderation cases (area 10) has shipped, following `SAFE-REPORT-01`; enforcement and physical acceptance remain open. The owner subsequently approved rejecting equal enabled quiet-hour endpoints; PR #153 implements that rule and ordinary actionable/bundled reminders, pending release. No-longer-achievable interval-goal notices and per-Path participant reminder controls (area 9) merged in [PR #134](https://github.com/elsell/hourpaths/pull/134) as `74f3f33` after all five candidate gates passed. v0.46.0 API/web is deployed through GitOps `75d4c2d`; exact ready source/images, Flux Ready/Healthy, retention image and public Studio 200/API readiness 204 are verified. iOS release `37923420558` failed its required-input check; Android internal publication `37925662502` is in progress. Implementation and focused checks pass: exact interval boundaries, recorded progress, preference CAS/replay and account isolation, concurrent once-only receipts, old-client capability filtering, and final push access/preference/timer checks. Studio-to-Android preference persistence and the real-worker notice in both clients are verified; Both clients open the correct Path. Native availability and physical-device verification remain pending. Long-running timer notices merged in [PR #133](https://github.com/elsell/hourpaths/pull/133) as `1647782` after all five gates passed. Real-worker Studio/Android notice, Path navigation and native stop with independent Studio readback are verified. v0.45.0 API/web is deployed through GitOps `bc64a54`; exact ready source/images, Flux Ready/Healthy, retention image and public Studio 200/API readiness 204 are verified. Android internal run `37916446645` succeeded and committed version code 364 with release notes. iOS release `37915802033` failed the missing Live Activity provisioning-profile input; physical-device availability is not claimed. Isolated real-worker acceptance verified the once-only notice in Studio and Android, with both opening its Path. Current-average boundaries, audit rollback, concurrent workers, capability compatibility, history access revocation and push authorization/localization pass focused checks. Physical push acceptance remains pending. Policy reacceptance merged in [PR #132](https://github.com/elsell/hourpaths/pull/132) as `2c2fc8d` after all five immutable gates passed. Isolated Studio verified publication, review, timer stop through real SpiceDB, own-account JSON export and unchanged-session resumption. Android verified document-provider save/cancel, review from Following, deletion navigation/cancel, acceptance and sign-out after a subsequent publication. Main CI and scanned publication passed; GitOps `5b034e4` deploys the exact API/web images with Ready/Healthy Flux, completed rollouts, Studio HTTP 200 and API readiness HTTP 204. Native publication, iOS sharing and physical-device availability are not yet verified. Native-tab recovery merged in [PR #131](https://github.com/elsell/hourpaths/pull/131) as `92ac24d` after all five gates passed; inactive Following/Stats tabs no longer trigger recovery while unfocused. Its v0.43.1 API/web images are deployed through GitOps `15cfbb5`; exact source and image readiness, Flux Ready/Healthy, Studio HTTP 200 and API readiness HTTP 204 passed. Android internal publication run `37895298683` succeeded, committing version code 352 with release notes; physical-device installation remains unverified. iOS archive/upload failed and is not claimed available. Android 160% text and Following → Home → Settings checks passed. The recurring quiet-hours decision is resolved; Studio/Android save/readback, concurrent edits and equal-time validation pass, but the feature is not yet released.

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

This historical partial delivery did not close another acceptance area. The
current remaining count is maintained at the top of this checklist.

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


### Warning notices and appeals (released; device acceptance open)

Native and Studio Settings now expose private Account notices, one immutable
appeal within 30 days, and its final decision. Isolated QA demonstrated an
operational warning, Studio submission and reload, audited operator review,
and the same reversed decision with original explanation in Android Settings.
Fresh migrations through 93 and PostgreSQL checks cover ownership, retries,
audit rollback, restricted review grants and immutable final decisions. Native
and Studio types, localization, transport boundary and Settings checks passed;
the scoped security critic found no P0/P1 blockers. PR #137 merged as `70d4cd7`
and is included in verified TestFlight 0.49.8 (442). This does not close area 10:
content removal, suspension, bans,
restricted-account appeals remain required. The owner approved an in-house no-op moderation port on
2026-10-12 and deferred automated classification for the trusted-content phase.

### Native expired-session timer recovery (released; device acceptance open)

Cold and foreground expiry now preserve the verified account's local Stop-only
recovery, without restoring API access, new starts, or replay. Android acceptance
verified cold expiry, local Stop offline, restart, and same-account sign-in
replaying exactly one activity with its original timestamps. Foreground expiry
also exposes and completes local Stop. Emulator clock changes were restored
before stopping; production session settings were unchanged. Focused restoration,
owner isolation, delayed-account recovery and native tracking tests pass, along
with mobile types, localization and client-boundary checks. Issue #148 tracks
this slice. PR #149 merged as `01a4939`; exact API/web deployment through
GitOps `f18bd8e` is verified. TestFlight 0.49.7 (438) is VALID, IN_BETA_TESTING
and internally assigned (status run `37989424308`). Area 2 remains open for its
remaining acceptance criteria.

## October 17 owner follow-up

The owner reported Live Activity Stop failure. PR #201 is now available for
retest as described above; native control acceptance remains open. Automatic
provider-photo import with removal/replacement is implemented in PR #202. External backup
retention verification is temporarily deferred at the owner’s request; it must
not be represented as completed. Account deletion and other acceptance stay open.
