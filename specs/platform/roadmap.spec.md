# Platform Roadmap Specification

## Current focus

The [ordered completion checklist](completion-checklist.spec.md) retains the
full approved audit scope: **11 of 13 acceptance areas remain open**. Studio
legacy journeys and the Studio cutover are closed; new cross-client behavior and
physical-device acceptance remain in their owning areas. Widgets remain excluded.

Comment-removal enforcement and private appeals merged in PR #155 as `d2bd82c`.
Release v0.50.0 is verified in production through GitOps `12e991b`, with exact
API/web images, Ready/Healthy Flux and public readiness. TestFlight 0.50.0 (450)
is VALID, IN_BETA_TESTING and assigned to internal testers; its release notes
were read back. Google Play internal version code 420 was committed by workflow
`38002647767` after signed-bundle verification. Physical-device acceptance is
still required; publication does not close the wider moderation area.

Bundled goal reminders and unavailable periods merged in PR #153 as `90b41af`,
including the approved rejection of equal enabled quiet-period endpoints.
Release v0.51.0 is verified in production through GitOps `3603732`; exact images,
Flux health, retention configuration and public endpoints pass. TestFlight
0.51.0 (453) is VALID, IN_BETA_TESTING and assigned to internal testers with
verified release notes (Apple status `38047918135`). Google Play internal version
code 423 was committed after signed-bundle verification (`38047890528`).
Physical-device delivery acceptance remains open; the 11-area count is unchanged.

No product implementation slice is currently active. Path-grid preparation has
identified reusable calendar layouts and the required trailing-12-month/quartile
differences; shared-Path participant aggregation awaits the owner decision.
The richer Live Activity proposal and public-text processing location also await
owner decisions. These unresolved decisions do not authorize inferred behavior.

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
