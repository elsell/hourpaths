# Studio web experience

Status: Approved for implementation, September 29, 2026

## Purpose and authority

Replace the web presentation with the approved Studio design: a light sidebar,
pastel Path rows, clear time controls, and connected activity timelines.
The reference screens are in [docs/design/web-studio](../../docs/design/web-studio/).
This specification incorporates two corrections to the pictures: timelines remain
connected across day boundaries, and Following has no people-list sidebar.

This spec governs web presentation. Existing domain specifications remain
authoritative for calculations, permissions, lifecycle, privacy, and persistence.
Illustrative names, dates, numbers, biographies, and settings in generated
pictures are not new product capabilities or changes to those domain rules.

## Actors and preconditions

A signed-out visitor can access account entry and provider sign-in. An activated
account holder can use Paths, Following, Stats, Settings, and permitted profiles.
Account recovery and onboarding retain their existing prerequisites.
A Path is the existing owned or shared time-tracking resource. A timeline event
is an authorized recorded activity; a running session is not a completed event.

## Shared shell and visual system

- Desktop primary navigation must be a persistent light left sidebar, with
  HourPaths identity, Paths, Following, and Stats; Settings and the current
  account appear in its lower area.
- Paths is the web label for Home. Mobile retains Home.
- The selected destination must have a visible indicator and semantic current-
  page state. Each page must have an explicit title.
- The shell must preserve entry points for notifications, invitations, and
  contextual administration already specified elsewhere.
- Main surfaces must use white/off-white backgrounds, dark readable text,
  restrained pastel Path colors, consistent rounded surfaces, and flat controls.
- Shared typography, spacing, colors, radii, borders, focus, loading, empty,
  error, button, menu, field, dialog, and timeline components must govern all
  screens. Page-local copies of these primitives must not diverge.
- Start, Stop, Follow, Unfollow, Save, and destructive actions must remain
  unmistakable, named actions. Color alone must not communicate state.
- Text and control contrast must meet WCAG AA. Keyboard operation, visible
  focus, semantic labels, reduced motion, and translated copy remain required.
- Narrow layouts must reflow controls and collapse navigation without losing
  destinations. Secondary panels move below primary content; tables must not
  make primary actions inaccessible. Browser zoom must not truncate controls.
- Large-screen composition should match the reference proportions and density.
  Generated shadows, gradients, promotional filler, and sample data are not
  required design elements.

## Paths

- Paths must present stable, ordered horizontal pastel rows, not a desktop
  tile grid. Each row contains the chosen emoji, Path name, period total,
  goal and recurrence, progress, timer control, and trailing overflow.
- Period totals, goals, and timer controls must align in consistent columns.
  The period caption must reflect each Path's recurrence; a weekly column
  heading must not mislabel daily or hourly goals.
- The dominant time must consistently represent accumulated goal-period time;
  a Path without a goal shows total time with an explicit total label.
- An active timer must update period and lifetime totals live. Its Stop control
  contains elapsed session time. Start/stop must not reorder the Path.
- Personal color, emoji, pinning, and manual ordering must use persisted
  account preferences shared with other clients.
- Row navigation opens Path details/editing. Timer and overflow controls must
  perform their own actions without also opening the row.
- Search and All/Pinned/Archived filters must preserve existing visibility.
  New Path and the complete existing Path lifecycle remain reachable.
- The right panel must show the viewer's recent authorized activity using the
  shared connected timeline, with an entry to full history.

## Connected activity timeline

- One vertical line must connect all visible events, including across day
  boundaries. Day labels and distinct day markers attach to that same line.
- Days must not be separate cards or be split by horizontal divider lines.
- Events must be chronological, newest first, with readable occurrence times,
  Path identity, and duration. Day grouping must follow the authoritative
  time-zone rules, including events around midnight.
- Pagination must retain ordering and avoid duplicate day headings at page
  boundaries. Refresh must not unexpectedly move the reader's scroll position.
- Empty and unavailable history must explain the state without fabricated
  entries. Recovery must be possible without restarting the application.

## Following

- Following must show active people above the chronological social timeline.
- The avatar ring must be segmented by active Path count for that person,
  without a printed active-count caption.
- Selecting an active avatar must open the approved live Path viewer, retaining
  manual left/right navigation, live goal progress, full-bleed Path color, and
  no transition animation between Paths belonging to the same person.
- The feed must show authorized activity with author/profile links, Path
  summaries, comment counts, multiple emoji reactions, a full emoji picker,
  and accessible reaction participant rosters.
- Following must not render the reference's People you follow sidebar.
  The feed should use a readable centered content width in the remaining area.
  Search and Find people retain access to discovery and relationships.

### Social slice acceptance

- Direct Following and username-profile URLs must resolve through the Studio
  router, including reload and browser history.
- Refresh and pagination must keep publication order and deduplicate event IDs;
  comment and reaction changes must update counts without moving events.
- A running Path removed by authoritative refresh must close its live page or
  advance to another currently permitted Path, never retain stale access.
- Comment drafts must survive recoverable submission errors; editing must carry
  the reviewed comment version. Interaction rosters must paginate through the
  authorized endpoint rather than infer identities from counts.

## Stats

- Stats must include date and Path filters, period summary, activity-over-time
  chart, a GitHub-like daily contribution grid, and aligned Path breakdown.
- Contribution grids must use seven weekday rows and week columns with month
  labels, an intensity legend, and accessible values independent of color.
- Activity-over-time must initially expose the most recent date; selecting
  another range must preserve an intelligible date context.
- Aggregations and goal calculations follow the analytics spec, including
  differing recurrences. Sample image totals are not calculation requirements.
- The secondary activity timeline uses the same connected day-marker component.

### Stats slice acceptance

- The Studio Stats route must retain the existing Week/All Paths initial
  selection and all five specified range presets. Period navigation must use
  the server-provided adjacent anchors.
- Summary cards must derive from returned recorded time, active calendar days,
  and Paths with recorded activity; illustrative session counts or cross-Path
  goal percentages must not be invented from incomplete client history.
- Changing a filter must never display the previous selection’s aggregate as
  current. Refresh must preserve chart scroll positions and selected controls.

## Profiles

- Profile identity must combine avatar, name, username, Path count, follower
  count, and following count in an Instagram-familiar hierarchy.
- Active Paths must appear as segmented rings around the profile avatar;
  selecting it opens that person's permitted active Paths.
- Follow/Unfollow must be prominent and reflect actual relationship state,
  including any request, privacy, or blocking rules.
- Activity must be the default profile view, showing chronological history
  along the connected timeline, comments, and emoji interactions.
- Permitted Paths may appear as a secondary list and a Paths view.
  Another person's Path summary must not expose the viewer's Start/Stop control.
- Counts and history must not reveal inaccessible Paths or activities.

## Settings

- Settings must use a secondary section navigation and compact, consistently
  aligned labeled fields and grouped rows matching the reference.
- Account, preferences, appearance, notifications, privacy/interactions,
  blocked accounts, sign-out, and deletion must remain reachable according to
  their existing specs. Mockup-only fields do not create unsupported settings.
- Provider-managed identity data must be visibly read-only where appropriate.
- Save must communicate pending/success/error state and preserve drafts on
  recoverable failures. Unsaved changes follow existing loss-prevention rules.
- Sign-out and deletion must retain their existing timer-resolution,
  confirmation, credential revocation, and authorization behavior.

### Settings preference slice acceptance

- Settings sections must use the shared Studio navigation and compact grouped
  fields. The first slice exposes authoritative account identity, time zone,
  personal Path appearance, nudge delivery, interaction controls, and blocked
  accounts. Remaining account lifecycle controls remain required before cutover.
- Time-zone changes must display the exact current/proposed zones for confirmation
  and submit that reviewed value with a stable retry identity. A conflict must
  require reloading and reviewing the current setting before another submission.
- Notification changes must carry the loaded revision. Failed saves must retain
  the user's draft; successful saves must replace it with the authoritative result.
- Browser navigation away from an unsaved preference form must offer a choice
  to retain the draft. Unblocking must identify the account and require confirmation.
- All preference reads, mutations, drafts, and pending confirmations must belong
  to the active Studio account lifetime and be disposed on account replacement.

### Session control slice acceptance

- Account settings must expose sign-out with an explicit confirmation. Before
  offering Stop and save, the client must load the account's running timers.
  A failed review must allow retry or an explicit keep-running sign-out.
- Stop and save must retain the session if any stop fails. Retrying must reuse
  the operation identity for an unresolved timer and recheck authoritative
  running timers before clearing the credential. A newly started timer must
  require another review instead of being silently stopped or abandoned.
- Same-client timer mutations must settle before a sign-out review can begin.
- Expiring renewable sessions must refresh without losing navigation. Temporary
  refresh failure must preserve an unexpired credential and use bounded retry.
  A refresh that does not advance expiry must not schedule a refresh loop or
  extend the server's absolute session-family lifetime.

## Failure, security, and acceptance

- API DTOs must not become presentation or client domain models; see the
  [web architecture](../platform/web-studio-architecture.spec.md).
- Account change and sign-out must cancel owned requests and clear user-scoped
  caches so delayed results never populate another account's screen.
- Temporary failures retain valid credentials and offer local retry.
- Direct navigation, reload, Back, and Forward must resolve the correct page.
- Verify real tracking, preference persistence, feed/profile visibility,
  interactions, Stats, and account workflows against the API before cutover.
- Verify desktop and narrow screenshots against the stored references, including
  long names, missing goals, empty states, and active sessions.
- Tests should target behavior, authorization, data mapping, and account
  isolation. Source-string assertions and tests duplicating markup should not
  be added merely to freeze implementation.

## Delivery sequence

1. Authenticated shell, isolated Path models/adapters, and live Paths workflow.
2. Following, live viewer, profile, comments, and reactions.
3. Stats with contribution grid and connected personal history.
4. Settings and remaining account/Path lifecycle parity; complete web cutover.

Only one implementation slice is active at a time. Each slice must be usable,
verified, and merged; temporary compatibility entry points must be removed at
final cutover. A styled demo or a partial feature set is not completion.

## Open questions

None blocking the approved layout. Further visual refinements follow user
feedback; existing domain specifications resolve behavior absent from pictures.
