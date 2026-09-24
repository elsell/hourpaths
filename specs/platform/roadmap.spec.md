# Platform Roadmap Specification

## Current focus

Deliver UI-28L as the sole active product slice under UI-28 and the core
experience epic. The existing Path nudge composer and per-Path audience
destination must use inherited native chrome, standard single-choice controls,
safe dismissal, retained retry state, authoritative session-owned operations,
and nonblank cold-route recovery. SOC-07 and incomplete notification,
reporting, profile/provider, deletion, and Stats functionality remain deferred
to their owning functional slices.

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
- ACCT-01's duplicate-prevention handoff and ACCT-03A's dual-provider recovery
  boundary must be explicit before recovery behavior expands.

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

- Polish the existing Path nudge composer and per-Path audience destination
  through UI-28L. Deliver SOC-07 acceptance, notification delivery and
  stale-target gaps, the missing channel catalog, recurring unavailable period,
  reporting, profile/provider work, and account deletion only through their
  owning functional slices. Stats enters the polish sequence only when its
  specified product surface exists.
- Keep ACCT-01 open and preserve duplicate-recovery, provider-picture, and
  provisional-lifecycle decisions outside UI-28B unless one demonstrably blocks
  its existing presentation path.
