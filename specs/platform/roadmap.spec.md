# Platform Roadmap Specification

## Current focus

The [ordered completion checklist](completion-checklist.spec.md) retains the
full approved audit scope: **11 of 13 acceptance areas remain open**. Studio
legacy journeys and the Studio cutover are closed; new cross-client behavior and
physical-device acceptance remain in their owning areas. Widgets remain excluded.

The complete-range Stats fix is available in v0.51.1: production GitOps
`0fcf421`, TestFlight 457 (VALID, IN_BETA_TESTING and internally assigned), and
Google Play internal 427. TestFlight notes were read back. Empty days and entirely
empty periods remain visible; physical-device acceptance remains open.

Path-specific analytics merged in [PR #158](https://github.com/elsell/hourpaths/pull/158)
as `e9c3f3c` after all five checks. It uses shared Stats charts and calendars,
personal activity by default, participant selection, complete recorded-history
metrics and the specified trailing-year/quartile grid. Studio and Android
emulator acceptance passed. Production v0.52.0 is verified through GitOps
`06908ac`, with exact images, Ready/Healthy Flux and public readiness.
TestFlight 0.52.0 (460) is confirmed available to internal testers with
verified release notes; Google Play internal build 430 was committed after
signed-bundle verification. Area 11 stays open until current-year initial calendar positioning and remaining acceptance
are verified. Shared mobile analytics large-text corrections merged in PR #159;
Android 160% interaction checks passed and release verification is in progress. The
active slice now corrects current-period Stats ranges to end on participant-local
today, preserving complete historical periods and shared client charts.

Next unresolved implementation choices are provider-picture import during
onboarding, the richer Live Activity design, and public-text processing location.
Account suspension/ban enforcement and signed-device acceptance also remain.
NAS-level snapshot encryption and expiry need authenticated storage evidence;
Kubernetes configuration and the verified isolated restore rehearsal alone do
not establish external backup retention. These requirements remain in the
ordered checklist rather than expanding the analytics slice.

Only one product implementation slice may be active. Preserve completed
implementation and use the checklist's concrete acceptance criteria to select
independent work; do not restart delivered Studio journeys or repeat release
verification without a changed risk. Physical-iPhone encouragement selection and
the UI-28 acceptance matrix remain open.

## Gap-closure scope and evidence

The following work remains open until implementation, applicable acceptance,
merge, and release evidence establish the specified behavior. Specification
coverage alone must not mark an item complete.

| Area | Outstanding delivery scope |
| --- | --- |
| Studio replacement | Closed under areas 1 and 12; calendar correctness and new cross-client features remain in their respective areas |
| Offline | Durable timers and activity edits, retained history, causal synchronization, conflicts, and offline authentication lifecycle |
| Account lifecycle | Permanent deletion, retention/restore verification, provider linking/unlinking and duplicate-account recovery |
| Profiles and preferences | Editing and safe image uploads, privacy changes, follower lists/removal, editable week start, unavailable periods and policy reacceptance |
| Notifications | Complete channels and subscriptions, timer-start/achievement delivery, reminders, unachievable/long-timer notices and quiet-period enforcement |
| Safety | Reporting, moderation, enforcement, appeals and public-text checks |
| Native and analytics | iOS Live Activities, Android ongoing timer notification, Path-specific contribution grid |
| Verification and delivery | Physical-device encouragement and accessibility evidence, release availability, production revision, and reconciliation of stale open questions |

Global Stats and the mobile Home redesign have implementations; neither is a
future blank surface. Their remaining scoped gaps and native acceptance still
require evidence. Widgets remain explicitly deferred. Unresolved decisions below
must be reconciled with authoritative domain specs before implementing their
related behavior; they must not be silently treated as settled or as proof that
already-delivered behavior is missing.

## UI-28 Apple-native audit matrix

UI-28 must remain open until every journey row below has current physical-iPhone
evidence. Each child owns only its listed journey; the parent owns the final
cross-app consistency audit.

| Journey | Owning slice | Required parent acceptance evidence |
| --- | --- | --- |
| Sign in and account recovery | UI-28A | Native root title/actions, standard controls, no custom bar background, normal and maximum Dynamic Type |
| First-sign-in onboarding | UI-28B | Native form/sheet hierarchy, keyboard completion, safe dismissal, VoiceOver |
| Authenticated Home | UI-28C and UI-28G | Native tab/navigation/toolbar materials, flat Path rows, native filtering, nonoverlapping progress, loading and recovery |
| Create Path | UI-28D | Native sheet detents/actions, safe interactive dismissal, standard menus and pickers |
| Path detail, tracking, and history | UI-28E | Native title/back/toolbars, no blank direct route, native activity sheet, history hierarchy |
| Path management and sharing | UI-28F | Native sheets, menus, confirmations, role-safe actions, unsaved/admitted-mutation dismissal |
| Following and profiles | UI-28H | Native navigation and search, flat feed and identity rows, authoritative route recovery, content-first hierarchy |
| Feed interactions and comments | UI-28I | Native menus, keyboard-safe composer and edit tasks, comment and roster navigation, retained draft and recovery |
| Notifications and notification controls | UI-28J | Native list hierarchy, navigation title/actions, permission route, implemented channel switch, nonblank recovery |
| Account and preference settings | UI-28K | Native grouped hierarchy, account confirmation, time-zone picker, interaction switches, blocked-account recovery |
| Path nudges and audience controls | UI-28L | Native preset task, Cancel and Send-or-Retry, audience selection, safe dismissal, nonblank direct-route recovery |

For every row, evidence must cover ordinary presentation plus maximum Dynamic
Type, VoiceOver, Reduce Motion, Reduce Transparency, Increase Contrast, keyboard
use where applicable, native back and safe swipe dismissal, and absence of
custom backgrounds that interfere with system Liquid Glass. Custom glass must
remain absent from the content layer unless the evidence identifies a necessary
functional control with no suitable standard component.

## Unresolved product decisions

Only decisions required by the current acceptance outcome block ACCT-01. A
decision that governs optional later lifecycle maintenance must not expand or
delay the active successful-account-creation slice.

- The approved specifications do not choose an initial public/private profile
  privacy value or state that onboarding asks the user to choose one.
- The current policy version identifiers, published Terms of Service, Privacy
  Policy, Community Guidelines locations, and monitored support route have not
  been supplied.
- The specifications require an OIDC profile picture to pre-populate onboarding
  and require accepted images to be safely re-encoded, but do not decide
  whether an untouched provider picture is imported by default, imported only
  after an explicit choice, or used only as a preview.
- The first-day-of-week preference must follow the device locale, but fallback
  behavior is not defined when locale week information is unavailable.
- The specifications allow an incomplete account to return to onboarding, but
  do not define retention or cleanup for abandoned provisional users and
  provider identities. Until a retention policy is approved, the initial
  behavior is an indefinitely retained, non-discoverable, restricted
  provisional lifecycle that resumes onboarding.
- The 100-character display-name limit does not define whether a Unicode
  character means a Unicode scalar value or a user-perceived grapheme cluster.
- The identity-access, onboarding, and profile specifications must be reconciled
  on provisional identity ownership and whether later provider claims can update
  a saved profile.
- The verified-email duplicate-account check does not define correspondence and
  normalization rules. Email must remain non-authoritative and must not disclose
  or establish ownership of an existing account.
- The duplicate-prevention and dual-provider recovery boundary is now specified
  in [Account Authentication](../accounts/authentication.spec.md#duplicate-account-prevention-and-recovery)
  and implemented by PR #116. Real-provider and physical-device acceptance
  remain required; the boundary itself is no longer an undecided product choice.

## Required evidence

- Keep this file current only when focus, sequencing, completion evidence, or a
  known blocker changes.
- Record verification and adversarial boundary acceptance in the repository's
  normal review and CI records when a roadmap item is completed.
- Active work must have an authoritative specification, focused behavioral
  tests, affected integration checks, and user-visible acceptance evidence.
- Public release documentation must not include private project identifiers,
  internal host details, or external operational credentials.

## Next implementation slices

Use the [ordered completion checklist](completion-checklist.spec.md), rather than
an expanding list of Studio micro-features. Keep small PRs under their owning
acceptance area and close an area only with client, merge, and release evidence.
Existing implementation must be credited; calendar summaries and global Stats
must not be rebuilt simply because acceptance or documentation is incomplete.

Unresolved account and policy decisions remain above until reconciled with the
authoritative spec and actual implementation. They do not authorize new product
choices, and they must not stall independent areas of the checklist.
