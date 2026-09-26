# Primary Navigation

Status: Approved for implementation

## Purpose

Define the application's three primary product surfaces and their separation of
responsibilities.

## Primary surfaces

- The application must provide three primary surfaces: Home, Social, and
  Stats/History.
- Their initial user-facing navigation labels must be `Home`, `Following`, and
  `Stats`, respectively.
- The `Following` label must not narrow the already-specified Social feed rules:
  eligible activity from participants in shared Paths must still appear even
  when the participant is not followed.
- Each primary surface must be directly reachable through primary navigation.
- Moving between primary surfaces must not require navigating through another
  primary surface first.
- The specific navigation component used to expose these surfaces is a design
  decision, not required behavior.
- During incremental delivery, primary navigation must expose only primary
  surfaces whose specified behavior is implemented. It must not add empty or
  disabled destinations for unfinished surfaces.
- A system-native tab shell may initially contain only Home so the stable
  navigation hierarchy can ship before Following and Stats. Following and
  Stats must enter that same shell when their approved behavior is implemented.
- Secondary destinations must be presented above the primary tab shell so
  native back navigation and edge-swipe dismissal return to the previously
  selected primary surface.
- Every non-Home destination must identify the current view with the native
  navigation title and expose the platform back control or the appropriate
  leading navigation action. It must not replace the native bar with a custom
  in-content title, back button, or toolbar.

## Default launch destination

- The primary tab shell must be mounted only after authentication resolves to an
  activated application Home destination. Account entry, onboarding, and account
  recovery must use the root navigation stack without visible or operable primary
  tabs.
- A signed-in application launch without an explicit destination must open
  Home.
- An authorized deep link or opened notification may instead open its relevant
  destination.
- If an explicit destination is unavailable or the user is not authorized to
  access it, the application must fall back to Home after communicating the
  problem plainly when needed.
- Restoring or opening a secondary Path, activity-history, activity-detail,
  social profile, Follow Requests, comments, interaction-roster, Notifications,
  Settings, Account, Time Zone, Interactions, Blocked Accounts, Path nudge-
  audience, or notification-settings destination must mount its native route
  and visible localized loading state before authoritative target resolution
  completes. The
  client must not render a blank route or silently replace a valid explicit
  destination with Home only because an owning primary surface's in-memory
  collection or presentation context has not loaded yet.
- Route resolution must remain owned by the activated account and profile
  lineage that requested it rather than one exact credential object. Credential
  rotation for the same account must preserve valid route ownership, while
  account replacement, sign-out, or activation fallback must cancel it so a late
  target result cannot navigate or expose data in another account.

## Responsibilities

- Home must prioritize the user's paths, current progress, and quick time
  tracking as defined in [Home](home.spec.md).
- Social must contain feed activity, following, discovery, and encouragement
  interactions.
- Stats/History must contain historical activity, calendar views, and aggregate
  progress analysis.
- Social feed content must not be duplicated onto Home.
- Detailed historical analysis must not displace Home's quick-tracking focus.

## Secondary destinations

- Notifications must have a dedicated bell-icon entry point that opens a
  secondary native-stack screen above the primary tab shell rather than
  expanding notification history on Home or nesting it in the account menu.
- Pending Path invitations must have their own secondary native-stack screen
  and must remain reachable independently of notification history, including
  after the related notification is read or deleted.
- The Notifications screen must provide a direct, compact route to pending
  invitations without embedding the invitation workflow in notification rows.
- The user's own profile and account settings must be grouped behind a single
  account or avatar entry point.
- Path administration must be accessed contextually from the relevant Path's
  detail experience rather than from the global account menu.
- A native sheet opened from a secondary destination must be presented by that
  currently visible destination so it appears above the active navigation
  stack. Dismissing the sheet must reveal the destination that opened it rather
  than an earlier screen hidden beneath the stack.
- A single-view task sheet must place Cancel or Close in the native leading bar
  position and Done, Save, or its retry equivalent in the trailing position.
  It should support the native swipe-to-dismiss gesture when dismissal cannot
  abandon an admitted mutation or silently discard meaningful unsaved work;
  otherwise it must disable interactive dismissal or request confirmation only
  for the duration of that concrete risk.
- A keyboard-composing secondary stack destination must preserve its meaningful
  draft across ordinary state refresh. Native back or edge-swipe may dismiss a
  clean task, but a dirty draft must be retained or confirmed before discard and
  an admitted mutation must not be abandoned. Once authoritative success or
  failure reconciles, ordinary navigation behavior must resume.

## Mobile task layout

- People search, social activity details, comments, and comment-heart rosters
  must open above the primary tabs. The comment composer must remain above the
  bottom safe area and software keyboard without a tab bar covering its controls.
- Returning from a secondary task must preserve the existing primary-tab state.
  A cold social interaction must reconstruct a Following destination beneath it;
  a heart roster must additionally retain its parent comments destination.

## Primary-surface state

- Switching among Home, Following, and Stats must preserve each surface's
  current navigation position, selected filters, and scroll position.
- This state must remain independent per primary surface for the duration of
  the current application session.
- A fresh application launch may reset each surface to its default root state.
