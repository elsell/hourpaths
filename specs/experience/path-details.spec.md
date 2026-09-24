# Path Details

Status: Approved for implementation

## Purpose

Define the focused view for understanding and managing one path.

## Entry

- Selecting a path from Home must open that path's detail view.
- Opening path details must not replace or navigate through the Stats/History
  primary surface.

## Native presentation and route recovery

- Path details, Path activity history, and activity-entry detail must use one
  native secondary navigation stack above Home. The native navigation title
  must identify the current Path or activity destination without repeating a
  decorative page title in the body. Standard native bars and controls must
  inherit the platform's Liquid Glass appearance without a custom navigation
  background that obscures or duplicates the system material.
- A cold launch, restored route, direct link, or notification route to an
  authorized Path, its history, or one of its activities must begin resolving
  that exact destination from authoritative account data. It must not require a
  previously populated Home collection in order to render or recover.
- Every route must immediately present a localized loading or recovery state in
  its native destination; no Path, history, or activity route may present a
  blank screen while authentication, the Path, or activity data resolves.
- A transient route failure must preserve any useful data for the exact target,
  explain its stale or failed state, and expose Retry and a route back to Home.
  With no useful data, it must show an explicit localized failure and those same
  recovery actions rather than an empty detail hierarchy.
- An unavailable, deleted, or unauthorized target must use the opaque Path- or
  activity-unavailable result and return safely to Home without disclosing
  target data. A signed-out, replaced, or newly provisional account must use its
  own account-entry destination; a late result owned by the previous session or
  profile must not populate or navigate the replacement account.
- The Path landing view must use a compact, flat native hierarchy: localized
  personal progress and primary tracking actions first for a participant,
  participant comparison next when applicable, followed by compact routes to
  Path activity history and other authorized secondary destinations. Repeated
  cards, borders, and headings must not obscure progress or tracking.
- Native Start-or-Stop and Add Activity controls must expose their complete
  action in text and accessibility semantics. Supporters and other users without
  tracking authority must not receive disabled or misleading tracking controls.
- Path history must use compact calendar-day sections and activity rows, with
  distinct localized loading, no-activity, retryable-error, pagination-progress,
  and end-of-history states. Activity detail must use a compact labeled-value
  hierarchy and expose edit actions only to the activity owner.
- Path detail, history, activity detail, and manual-entry or edit presentation
  must preserve native back behavior, logical VoiceOver reading order, at least
  44-by-44-point actions, reduced-motion behavior, and readable reflow through
  maximum Dynamic Type. Keyboard presentation must not cover required form or
  navigation actions.
- Manual-entry and edit sheets must use the native sheet container, appropriate
  system detents and grabber, and leading Cancel with trailing Save-or-Retry.
  They must allow interactive dismissal while the draft is safely discardable
  and no mutation is admitted; unsaved-loss confirmation or disabled interactive
  dismissal must appear only when that specific risk exists.

## Content

- For a participant, Path details must lead with that participant's own current
  progress before showing other participants.
- The personal summary must show total accumulated time, current interval
  progress when an interval goal exists, and overall-target progress when an
  overall target exists.
- On mobile, the native navigation title must identify the selected Path without
  repeating the same Path-name heading in the detail body.
- Mobile accumulated-time and goal-progress values must choose localized
  seconds, minutes, or hours appropriate to their magnitude instead of exposing
  storage-oriented seconds for every value.
- The mobile personal summary must present total time first, then only the goal
  progress indicators that are configured, followed by the primary activity
  action. A Path with no configured goal must not show an empty or failed-goal
  placeholder.
- The mobile primary activity action must use a native button, remain reachable
  after text-size reflow, and expose its purpose without relying on its icon.
- A shared Path must then show a comparison containing every current
  participant, including the current user when they participate.
- Each participant comparison must show current interval progress and
  overall-target progress when the corresponding goals are configured.
- The Path details landing view must show aggregate participant progress
  indicators only; it must not list any participant's individual sessions
  inline.
- A supporter has no personal tracking progress for the Path, so their detail
  view must begin with the participant comparison rather than an empty personal
  summary.
- Path details must provide a way to open the selected Path's recorded activity
  history without placing that session list on the landing view.
- On mobile, Path activity history and individual session detail must be
  separate native navigation destinations. Standard navigation-bar back
  controls and the platform back gesture must move from session detail to
  history, from history to Path details, and from Path details to Home.
- Path details must show statistics specific to the selected path.
- The default Path-specific statistics must include total tracked time, session
  count, and average completed-session duration.
- The default statistics must include a time-over-time chart for the selected
  Path.
- The default statistics must include a GitHub-style activity grid in which
  each cell represents one calendar day and visual intensity reflects the
  amount of time recorded on that day.
- The activity grid must default to the trailing 12 months through the current
  day.
- Mirroring GitHub's contribution-level model, the grid must use five total
  states: one `none` state for zero recorded time and four progressively
  stronger quartile levels for days with recorded time relative to other
  nonzero days in the visible range.
- The activity grid must represent the `none` state distinctly from every
  nonzero intensity level.
- Daily squares must be arranged into week columns, with consecutive days
  occupying consistent weekday rows.
- Month labels must appear above the grid at the horizontal transitions where
  the displayed dates enter each month.
- When the full grid does not fit its available width, it must scroll
  horizontally rather than shrink the daily squares until they become difficult
  to read or interact with.
- On initial display, a horizontally scrollable grid must be positioned at its
  rightmost edge so that the most recent dates are visible.
- The user must be able to scroll left to inspect older dates within the visible
  12-month range and return right to the present.
- Path details must not include activity from unrelated paths.
- Selecting a participant from the comparison must open a participant-specific
  detail view containing that participant's individual sessions on the Path.
- Each session-list row must show the session's calendar date, start time, and
  duration using the activity's retained occurrence time zone.
- A row must show `Edited` when a visible session field has been edited.
- A note-only edit must not expose an `Edited` indicator to another user because
  the note and its revisions are private to the activity owner.
- A mixed-participant Path history must identify the participant on each row;
  a participant-specific history must not repeat that identity on every row.
- Current participants and supporters with access to the Path must be able to
  inspect another current participant's sessions from that detail view.
- Another participant's sessions must be read-only; only the activity owner may
  reach editing controls for their own entries.
- An individual session detail must show its start, end, duration, and edit
  history.
- The session detail must not label the activity as timer-created or manually
  entered; its original recording method must not become a user-facing session
  classification after the entry can be edited.
- The initial product must not provide a session-specific title, tag, or
  category.
- The activity owner must be able to view and edit the session's optional
  private note from the session detail.
- Private activity notes must remain hidden from every other user as defined in
  [Recorded Activity](../tracking/recorded-activity.spec.md#recording-methods).
- The focused Path history must allow the activity owner to reach an individual
  recorded activity entry for inspection and editing.
- Path activity history must be grouped into calendar-day sections.
- Calendar-day sections must be ordered newest first.
- Entries within each day must also be ordered newest first.
- Path details must expose separate `Manage Path` and `Share` actions to users
  authorized to use them.
- `Manage Path` must contain Path configuration and lifecycle controls, such as
  its name, goals, archive state, and creator-only ownership-transfer and
  deletion actions.
- Creating, changing, or removing goals from `Manage Path` must use the warning,
  old-and-new comparison, explicit confirmation, and cancellation behavior
  defined in [Time Goals](../goals/time-goals.spec.md#confirming-goal-changes).
- After a confirmed goal change succeeds, the current goal configuration and
  progress shown on Home and Path detail must immediately reflect the new
  projection over unchanged recorded activity.
- `Share` must open a focused, Google Drive-style access-management experience
  rather than opening the general Path-management form.
- The Share experience must show the users who currently have explicit access
  and each user's Path role.
- The Share experience must allow authorized managers to invite participants or
  supporters and to manage or remove existing access using the role-change
  rules specified for Path membership.
- Path visibility and its resulting general audience must be managed from the
  Share experience because they determine access, not from `Manage Path`.
- Only the creator must receive an editable visibility control in the Share
  experience.
- Administrators who may manage explicit access must see the effective
  visibility without being able to change it.
- Creator-only and administrator-specific authorization rules defined by the
  Path role and membership specs must remain enforced within both experiences;
  the presence of a Share interface must not broaden a manager's authority.

## Native Path administration and access presentation

- `Manage Path` and `Share` must be separate native secondary destinations
  opened from the current Path detail route. Each must use the current-view
  native title and back control rather than a custom in-content header or one
  combined administration form.
- `Manage Path` must use a compact grouped hierarchy with configuration rows for
  Name and Goals and a separate lifecycle group for Archive or Unarchive,
  Ownership Transfer, and Delete. It must not place access, visibility, people,
  roles, or invitation controls in that destination.
- `Share` must use a compact grouped hierarchy for effective Visibility, People
  and Roles, Invitations, and Pending Invitations. It must not duplicate Path
  name, goal, archive, ownership-transfer, or deletion controls.
- Both destinations must derive every visible row and action from the current
  server-authoritative capability projection. Creators and administrators must
  receive only their permitted actions; ordinary participants and supporters
  must not receive disabled or misleading administration controls.
- An archived Path must remain visibly read-only. Its creator may receive only
  the specified Unarchive and Delete mutations; other roles must not receive an
  action that conflicts with archived-state rules.
- Initial loading, refresh, empty people or invitation collections, useful stale
  state, retryable failure, conflict, and unavailable state must be explicit and
  localized. A failure must preserve useful state for the exact Path, and a
  conflict must reload the authoritative Path before another review or attempt.
- Each edit or mutation must have one focused draft, review, confirmation, and
  completion model. Native leading Cancel or Close and trailing Save, Done,
  Confirm, or Retry actions must remain stable and must not be duplicated inside
  scrolling content.
- Native swipe dismissal must remain available when no meaningful draft would
  be lost and no mutation has been admitted. Dirty drafts must receive an
  explicit discard confirmation, and admitted mutations must temporarily block
  interactive dismissal so they cannot complete without authoritative visible
  reconciliation.
- Canceling or dismissing a review must send no request. A retryable failure
  must retain the exact draft or reviewed destructive context, while successful
  completion must apply the authoritative result once and recompute the
  available actions.
- Route and mutation ownership must remain bound to the initiating activated
  session, profile, and Path. Sign-out, profile replacement, or another Path
  becoming active must prevent a late completion from navigating or mutating the
  replacement presentation.
- Standard native bars, lists, sheets, menus, pickers, confirmation dialogs, and
  symbols must inherit Liquid Glass without custom glass in the content layer.
  The complete Manage Path and Share journeys must remain operable with the
  keyboard, maximum Dynamic Type, VoiceOver, Reduce Motion, Reduce Transparency,
  and Increase Contrast.
