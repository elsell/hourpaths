# Home

Status: Approved for implementation

## Purpose

Define Home as the minimal, low-friction place where users see their paths and
record time where permitted.

## Primary content

- Home must focus on paths against which the user can track time.
- A path with multiple participants must appear on Home for each participant just
  as an individually used path does.
- Home must show each path's current goal progress when an interval goal exists.
- Home must continue to show accumulated time for a path without an interval
  goal.
- Every trackable Path card must show the Path name and the participant's total
  accumulated time.
- When an interval goal is configured, the card must show the participant's
  progress in the current interval.
- When an overall target is configured, the card must also show the
  participant's progress toward that overall target.
- A card must omit an interval-goal or overall-target progress element when the
  corresponding goal is not configured rather than showing a false zero-value
  goal.
- Paths in which the user is an explicitly added supporter must also appear on
  Home, but in a separate supporter-only section from Paths in which the user
  can track time.
- A supporter-only Path must not expose start, stop, or manual time-entry
  controls to that supporter.
- Selecting a supporter-only Path must open its non-tracking Path details and
  progress experience; separately authorized social actions such as nudging
  remain available from their appropriate surfaces.

## Empty state

- A new user with no paths must reach an empty Home after completing onboarding.
- The empty state must provide a prominent Create Path action.
- The empty state must briefly explain that paths are used to track time.
- The application must not create sample or placeholder paths in the user's
  account.
- Path creation must not be mandatory within the profile-onboarding workflow.

## Path creation presentation

- Path creation from Home must use a compact native form sheet owned by the
  current authenticated navigation stack rather than a second application
  shell or a full-screen custom header.
- Path creation must prioritize the Path name and keep name-only creation as the
  obvious fast path; configuring either goal must not be required to create a
  Path.
- Optional interval and overall goals must be grouped separately and must reveal
  their configuration only when the user enables the corresponding goal. Each
  goal section must expand and collapse independently without changing the
  other section's draft.
- Goal durations must be enterable with recognizable time units while preserving
  whole-second precision at the product boundary.
- The same sheet must present Path visibility using only choices permitted by
  the creator's authoritative profile privacy. It must use the profile-derived
  default when available and a private fail-safe while that profile state is
  unresolved; it must not submit a more public visibility based on stale or
  missing profile state.
- Recurrence, duration-unit, and calendar-alignment choices must expose their
  selected value through native platform selection semantics.
- Every visible goal switch row must be a complete operable control rather than
  requiring the user to hit only the switch thumb.
- The creation sheet must keep native leading Cancel and trailing Create-or-Retry
  actions persistently recognizable and reachable when content reflows, Dynamic
  Type is enabled, or the software keyboard is visible. Submitting must prevent
  duplicate creation while communicating progress without removing either
  action from the sheet chrome.
- A failed creation attempt must preserve the entered draft, identify the
  problem in text, move accessibility focus or announce the failure as
  appropriate for the platform, and expose Retry without requiring the user to
  re-enter the Path name or either goal configuration.
- The sheet's reading and focus order must proceed from Path name through the
  independently enabled goal sections and any validation or submission status;
  Cancel, Create, Retry, goal switches, and native selection controls must
  expose their name, role, value or state, and availability to VoiceOver and
  equivalent platform assistive technology.
- Progressive disclosure and submission feedback must respect the operating
  system reduced-motion preference and must not depend on animation to
  communicate a goal section's state or creation outcome.
- Before a create request is admitted, Cancel or interactive sheet dismissal
  must send no request. After admission, the sheet must not be swipe-dismissed
  while the mutation is unresolved; its progress, late success, or retryable
  failure remains owned by the session and profile that submitted it.
- A late completion for the current owning session and profile must reconcile
  that profile's Home and dismiss the sheet exactly once. Sign-out or profile
  replacement must prevent that completion from navigating or mutating the new
  session's presentation; the original profile must recover the authoritative
  result through its ordinary refresh when it next becomes active.

## Quick tracking

- Each path on Home must provide a direct start/stop timer control.
- Starting or stopping a timer from Home must not require opening the path's
  detail view first.
- The primary timer action must be reachable with one deliberate interaction
  from Home.
- Home Path cards must not expose a manual time-entry action.
- Manual time entry must be available after opening the relevant Path's detail
  experience.
- Keeping manual entry in Path details must not add an extra confirmation step
  to the Home start-or-stop timer action.
- When a Path has a running timer, its Home card must show that timer's live
  elapsed duration.
- The running card must replace its start control with a visually prominent
  stop control.
- The elapsed duration must continue to advance without requiring the user to
  refresh or reopen Home.

## Separation from social activity

- Home must not contain the social feed.
- Home must not display comments, reactions, nudges, or unrelated follower
  activity.
- The fact that a path has multiple participants must not cause social feed
  content to appear on Home.
- Social features must be accessed outside Home.

## Simplicity

- Home must prioritize quick time tracking over administration, analytics, or
  social discovery.
- Home must not embed notification history or pending-invitation workflows;
  their dedicated secondary destinations must leave Path identity, progress,
  and quick tracking as Home's visible content.
- Secondary details must not obscure the path identity, current progress, or
  timer control.

## Native Home presentation and recovery

- Activated Home must use the authenticated native tab and navigation stack
  without adding a second application title, account identity heading, or
  custom navigation bar inside its content.
- Create Path, Notifications, and Account must remain recognizable native
  header actions. Path-view, ordering, and arrangement actions must use compact
  progressive disclosure rather than crowding or wrapping the primary toolbar.
- Empty, active, pinned, trackable, and supporter-only content must use a flat,
  compact grouped hierarchy with clear localized section headings and restrained
  separators. Repeated elevated cards, decorative borders, or spacing must not
  obscure the Path name, progress, disclosure action, or direct timer control.
- The initial Home load must pair localized status text with native progress
  semantics. It must not present an empty-Path result before the authoritative
  collection resolves.
- A retryable Home failure with no useful collection must present an explicit
  error and reachable Retry action. When useful Home data already exists, a
  refresh or transient service failure must preserve that data and communicate
  the stale, offline, or failed state without replacing it with an empty result.
- The no-Path empty state must explain Paths briefly and keep Create Path
  prominent. A selected filter with no matching Paths must instead identify the
  filtered-empty result and provide a direct way to return to `All`; it must not
  imply that the account has no Paths.
- Header actions, recovery actions, filter controls, section headings, Path
  identity, progress, and timer controls must remain readable and operable at
  every supported Dynamic Type size. Reflow or native overflow behavior must
  preserve logical reading order and at least 44-by-44-point touch targets.

Home's native navigation title must remain visible and distinct from collection
section headings. A Path's name, summary, and progress must share one disclosure
hit area, with its timer and menu remaining separate controls. Recorded interval
progress must be labeled distinctly from a currently running timer.

## Native Home control and row fidelity

- Home navigation and global actions must remain in the system navigation and
  tab bars. Home must not place an oversized floating toolbar, pill, or custom
  control island over the Path collection or reserve content space for one.
- Standard bars and their actions must inherit Liquid Glass automatically while
  Path content remains on the ordinary content layer. Custom glass, blur,
  shadows, borders, or backgrounds must not be added behind Home rows or
  filters.
- Home filters must use a standard native filter control. A system segmented
  picker may expose all four values where they fit; compact width or Dynamic
  Type reflow must use a native menu that exposes the selected value. Home must
  not imitate these components with hand-drawn filter chips.
- Ordering, arrangement, and other secondary collection actions must use a
  standard toolbar menu or picker with native selected-state semantics. They
  must not compete visually with Create Path, Notifications, Account, or the
  direct timer action.
- Active, pinned, ordinary, and supporter-only Paths must render as flat native
  rows with restrained system separators and intrinsic height. Heavy custom
  Path cards, repeated rounded containers, decorative elevation, and nested
  card backgrounds must not surround each Path.
- A trackable Path row must establish one stable reading hierarchy: Path name,
  accumulated time, configured interval and overall progress, then the direct
  Start-or-Stop action. Disclosure and secondary metadata must not interrupt or
  duplicate that sequence.
- Accumulated, interval-goal, and overall-target values must each have one
  localized label-and-value representation. A progress indicator must not
  overlay its label or value, and overlapping goal or progress copy must never
  be used to compress the row.
- At larger Dynamic Type sizes, the progress cluster and trailing actions must
  stack or move onto additional intrinsic-height lines rather than overlap,
  clip, truncate required values, or use absolute positioning. The timer action
  must retain its minimum touch target without covering Path content.
- VoiceOver reading order must follow the same Path-name, progress, status, and
  action hierarchy, with each configured goal exposed once. Reduced Motion,
  Reduce Transparency, and Increase Contrast must preserve the grouping and
  selected filter without depending on animated or translucent effects.

## Ordering and pinning

- A user must be able to choose how active paths are ordered on Home.
- The ordering control may be presented as a dropdown or another compact
  control that does not interfere with quick tracking.
- Available ordering methods must include recent activity, alphabetical order,
  and manual ordering.
- Home must default to ordering unpinned paths by most recent activity.
- A user must be able to pin selected paths so they remain above unpinned paths
  regardless of the selected ordering method.
- The user must be able to arrange their pinned paths manually.
- When manual ordering is selected, the user must be able to arrange the
  remaining active paths manually as well.
- Ordering and pinning choices must be personal to the user and must not change
  another participant's Home.
- Saving ordering or pinning must treat Paths the user created and Paths they
  joined as one complete accessible set. A stale save that omits an accessible
  Path or includes an inaccessible Path must fail with a conflict and preserve
  the prior preferences.
- Removing a user's membership from a Path must also remove that Path's saved
  pinning and manual-order placement for the user.

## Filters

- Home must provide compact filter chips labeled `All`, `Solo`, `Shared`, and
  `Supporting`.
- `All` must be the default and must show every active Path available on Home.
- `Solo` must show trackable Paths that have no other explicitly assigned
  participant or supporter.
- `Shared` must show trackable Paths that have at least one other explicitly
  assigned participant or supporter.
- `Supporting` must show Paths in which the current user has an explicitly
  assigned supporter role rather than a tracking role.
- Public or follower visibility by itself must not make a Path `Shared`; the
  filter classification must be based on explicit Path membership.
- Filtering must not change the user's saved pinning or ordering choices.

## Active timer position

- A Path with a running timer must move into a temporary active-timer section at
  the top of Home, above pinned Paths.
- When multiple Paths have running timers, the temporary active-timer section
  must order them by timer start time with the most recently started timer
  first.
- When its timer stops, the path must return to the position determined by the
  user's pinning and selected ordering method.
- Moving a running path temporarily must not change the user's saved pinning or
  manual ordering.

### Active Home acceptance slice (`EXP-02A`)

- Home must place every trackable Path with a valid running timer in one compact
  native **Active** section above the ordinary trackable-Path list.
- The Active section must appear only while at least one valid running timer is
  present. A supporter-only or otherwise non-trackable Path must never enter it.
- Active Paths must be ordered by authoritative timer start time, most recently
  started first. Equal start times must preserve the ordinary list's stable
  relative order.
- Each Path must appear exactly once across the Active and ordinary sections.
  Temporarily elevating an active Path must not mutate the ordinary list order
  that will apply after its timer stops.
- A successful start must move only that Path into Active without disturbing
  another running Path. A successful stop must return only that Path to its
  ordinary position while every other timer continues independently.
- A missing or invalid timer start instant must not elevate a Path. A failed,
  stale, or superseded start or stop response must not replace newer Home state
  or move a card according to an operation that did not become authoritative.
- Active cards must retain the existing live elapsed duration and prominent
  accessible stop control. The section heading and reading order must remain
  localized, text-scalable, and meaningful without relying on color.
- Pinning, saved ordering controls, Home filters, supporter grouping, and web
  presentation remain in later `EXP-02` slices. Home must continue to exclude
  social feed content and social interaction controls.

### Home organization acceptance slice (`EXP-02B`)

- The native Home must present `All`, `Solo`, `Shared`, and `Supporting` as one
  compact, horizontally scrollable system selection control. Changing the
  selection must not navigate away from Home or mutate saved ordering.
- The API projection used by Home must authoritatively classify each Path as
  `solo`, `shared`, or `supporting` from the viewer's explicit membership and
  expose the viewer's most recently completed activity instant. A visibility
  relationship alone must not affect this metadata.
- Recent-activity ordering means the signed-in participant's own most recently
  completed activity on the Path. Another participant's activity must not
  reorder that participant's Home. Paths without completed activity must follow
  Paths with completed activity in stable alphabetical order.
- Home must render trackable and supporter-only Paths in separately labeled,
  compact native sections. The Supporting filter must show only the
  supporter-only section; the other filters must not mix supporter-only Paths
  into trackable results.
- Home must offer a compact system menu for recent activity, alphabetical, and
  manual ordering. Pin and unpin must be available from each ordinary Path's
  native context menu, and pinned Paths must remain above unpinned Paths.
- A dedicated native reorder destination must allow manual arrangement of
  pinned Paths and, while manual ordering is selected, unpinned Paths. Reorder
  controls must expose move semantics to assistive technology and must not be
  represented by color alone.
- Home organization preferences must be stored per signed-in user, survive an
  application restart, reject malformed stored data without changing the Path
  collection, and never be applied to another account on the device.
- A running Path must still temporarily lead the filtered Home according to
  `EXP-02A`. Starting or stopping it must not mutate its saved pinned or manual
  position.
