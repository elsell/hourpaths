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

### Path lifecycle parity

- Studio must expose archive, unarchive, and permanent deletion only when the
  authoritative Path capability permits lifecycle management. Archived Paths
  must remain reachable through the existing Archived filter.
- Each action must review the exact selected Path before submission, using the
  warnings and confirmation rules in [Path lifecycle](../paths/lifecycle.spec.md).
  Cancel must perform no mutation. A pending request must prevent duplicate
  submission and ordinary navigation from abandoning the admitted operation.
- Retrying a failed confirmed request must preserve its idempotency key and
  reviewed name or archive state. A stale review must fail visibly, leaving the
  user able to close it, refresh, and review the current Path again.
- Successful lifecycle changes must clear stale tracking, history, social,
  profile, and statistics projections before refreshed data is displayed.
  Account replacement must not allow an old completion to change the new account.

### Sharing and sent invitations

- Authorized creators and administrators must reach an addressable Share page
  from a Path's actions. Direct navigation and reload must enforce the current
  `inviteMembers` capability and active-Path state before showing its contents.
- The page must use the Studio shell and shared fields, buttons, notices, and
  confirmation styling. Exact-username lookup must show the reviewed public
  identity and offered participant/supporter role before sending.
- Sending must bind the reviewed user ID and canonical username, and must not
  grant membership. Editing the username must invalidate its previous review.
- Managers must see all pages of pending invitations with recipient, inviter,
  role, and sent time. Cursors must not repeat and entries must not duplicate.
- Canceling must review the recipient and role with Cancel initially focused.
  Dismissal must not mutate state. Failed send/cancel retries must preserve the
  original idempotency key; pending mutations must prevent duplicate submission
  and navigation. Disposed-account completions must not alter another account.
- Success must announce the result and refresh the pending list. Errors must
  retain the reviewed request and expose the existing localized failure state.
  A transient background context refresh failure must retain the draft and retry
  state; confirmed permission loss or archival must hide stale content.
- Invitation authority, duplicate rules, and role effects remain governed by
  [Path membership](../paths/membership.spec.md). Recipient acceptance and
  rejection remain independently reachable; this page must not replace
  or weaken their existing behavior.

### Recipient invitations

- Studio must provide a dedicated, addressable pending-invitations page linked
  from its shell, independent of notification read state or retention.
- The inbox must paginate without duplicate entries or repeating cursors and
  show Path name, inviter's canonical public identity, offered role, and date.
  Joined context must be validated before display; provider data must not leak.
- Accept and decline must show the selected Path, inviter, and offered role in
  a cancel-first confirmation. Participant visibility warnings must show the
  reviewed audience, exposure, unchanged profile privacy, and retained-activity
  consequence where applicable. Acceptance must acknowledge only that audience.
- Cancel must leave the invitation pending. Pending decisions must prevent
  duplicate submission and navigation. Retry must preserve its operation key;
  a changed visibility requirement must leave the invitation pending and offer
  refresh/review rather than acknowledging a different audience automatically.
- Acceptance must refresh Home and remove the resolved invitation, announce the
  granted role, and offer a link to the accepted Path. Participants must be able
  to track their time there; supporters must see its goals without tracking or
  management controls. Direct navigation and reload must revalidate access.
- Decline must remove the resolved invitation and announce completion without
  granting membership. Disposed-account completions must not affect new state.
- Membership and private-profile warnings remain governed by
  [Path membership](../paths/membership.spec.md) and
  [Visibility](../social/visibility.spec.md).

### Profile blocking

- A non-self profile must expose Block using shared Studio controls. Starting
  the action must obtain a fresh server review for that exact account before
  presenting the cancel-first confirmation required by
  [Blocking](../safety/moderation.spec.md).
- Confirmation must identify the canonical reviewed account, explain ended
  follows and requests, and list any shared Paths with the retained-visibility
  warning. It must offer links for managing those Paths separately; blocking
  must not silently leave them or delete activity.
- Confirm must submit the exact server acknowledgement. Failed transport retries
  must retain the reviewed identity and operation key. Stale review rejection
  must require a new review instead of acknowledging changed shared Paths.
- Pending review and mutation must prevent duplicate actions; pending mutation
  must prevent navigation from abandoning the admitted command. Account disposal
  must cancel owned work and suppress late completions.
- After success, the profile must disappear and stale social, notification,
  history, and analytics projections must be discarded before reloading current
  authorized data. Settings must remain the independent unblock destination.
- Acceptance must cover cancel without mutation, shared-Path warning, failed
  retry, stale review, successful profile hiding, retained Path membership,
  independently reachable unblock, and account disposal. Domain blocking and
  authorization rules remain unchanged.

### Notification history

- Studio must provide a notification destination and an unread-count navigation
  badge using the authoritative history and mutation counts. Listing history
  must not mark notifications read.
- History must present separate newest-first actionable and informational
  sections with pagination, explicit refresh, mark-all-read, individual opening,
  and deletion, following [Notifications](../notifications/notifications.spec.md).
- Opening an item must revalidate it through the read mutation before navigating
  to the relevant invitation, ownership, follow-request, profile, or Path
  surface. Informational tombstones must be readable without offering a target.
  Destinations must apply their current authorization; unavailable targets must
  not reveal stale private content.
- Deleting a notification must not resolve its underlying request. Invitation
  and ownership pages must remain independently reachable.
- The badge and history must share one account-owned state. Same-account
  content-free cross-tab signals, focus/visibility refresh, serialized reads and
  mutations, and trailing refresh after successful mutations must follow the
  authoritative notification convergence rules. Failed refreshes must retain
  visible history and the last authoritative badge with a retry action.
- Account disposal must abort pending requests, close convergence listeners,
  and suppress late results. Core orchestration must be framework-independent
  and reusable; Studio models must not expose generated API DTOs.
- Acceptance scenarios must cover list-without-reading, open/read, mark-all,
  deletion without request resolution, pagination, same-account cross-tab
  convergence, wrong-account signal isolation, retry, and account disposal.

### Ownership transfer

- Every accessible Path must expose an addressable Studio ownership page,
  independently of notification read or deletion state. The page must revalidate
  Path access and load the authorized pending transfer; it must not treat a
  failed pending read as proof that no transfer exists.
- Creator initiation, candidate pagination, server-authored review, immutable
  expiration, recipient acceptance/decline, creator cancellation, and archived
  restrictions must follow [Path roles](../paths/roles.spec.md).
- Confirmation must identify the Path and canonical counterpart, explain role
  changes and unchanged activity, and show the exact expiration in the viewer's
  configured time zone. Initiation must also show the complete reviewed lifetime.
- Canceling a review must make no mutation. Retries of an unresolved confirmed
  command must retain its operation identifier and reviewed input. Pending
  commands must prevent duplicate submission and navigation; account disposal
  must prevent subsequent commands and ignore late results.
- Successful commands must discard stale role-dependent projections and reload
  authorized Path and transfer state. A failed refresh must remain retryable and
  must not leave old creator actions available after acceptance.
- Acceptance scenarios: a creator reviews and sends to an existing participant;
  the recipient opens the Path ownership page after reload and accepts; only then
  the creator becomes administrator and the recipient gains creator controls.
  Decline, cancel, expired review, archived Path, and inaccessible Path must not
  change roles. Failed transport retries must preserve command identity.

### Shared Path People

- Every viewer authorized to read a Path must be able to open an addressable
  People destination independently of invitation or management capabilities.
  Direct navigation and reload must revalidate Path and member-list access.
- People must paginate canonical identities and roles in compact rows. Selecting
  a row must show Path-scoped sessions, tracked time, and available interval and
  overall goal progress; it must not navigate to the member's social profile.
- The shared-Path exception in [Blocking](../safety/moderation.spec.md) must
  preserve authorized member visibility. Only `blockedByViewer` may expose an
  Unblock action; the presentation must not disclose who blocked the viewer.
- Unblock must use the shared cancel-first confirmation, preserve its operation
  key on failure, prevent duplicate or abandoned admitted mutations, and discard
  stale projections before refreshing. It must preserve membership and activity.
- Management actions must reuse the existing member-access commands and remain
  limited by the server-provided member capabilities. Archival must suppress
  role/removal actions without suppressing authorized read access or Unblock.
- Loading, denied access, empty results, refresh, pagination, failure, and retry
  must remain localized and operable with keyboard and narrow layouts. Account
  disposal must cancel owned work and suppress late completion.
- Acceptance must include an ordinary participant opening People after reload,
  reading another member's available progress without management controls,
  preserving shared-Path visibility across a block, and unblocking from the
  member detail without leaving the Path or restoring removed follow relations.

### Current member access

- Share must show paginated current members, their canonical identity, role,
  session count, and tracked time. Member-specific server capabilities must
  control removal and ordinary/administrator role actions.
- Before a destructive change, the client must obtain a fresh removal review
  for the exact Path and member. Confirmation must identify the person, current
  role, sessions, tracked time, and running-timer state and explain all data,
  offline, and reinvitation consequences defined in Path membership.
- Administrator grants, revocations, and voluntary step-down must require
  explicit confirmation and preserve participation and recorded activity.
- Cancel must receive initial focus. Pending mutations must prevent duplicate
  submissions and navigation. A retry must reuse the reviewed expected role and
  original operation key; changing or dismissing the review must cancel it.
- Successful changes must refresh member access and affected account projections.
  Stepping down must return to Paths with confirmation, without retaining
  manager controls. Late completions after account disposal must have no effect.
- Unavailable reviews and denied/stale operations must retain a localized error
  and retry/dismiss controls without claiming success or changing local roles.

## Connected activity timeline

- A tracking participant must be able to add activity from a Path row and edit
  their own entry from activity details. Both actions must use the same compact
  form with date, start time, hours/minutes/seconds, and a secondary private note.
- Forms must follow [Recorded Activity](../tracking/recorded-activity.spec.md)
  for participant-local defaults, original occurrence zone on edits, daylight
  saving transitions, exact seconds, note normalization, and future-end checks.
  Changing duration before occurrence is touched must keep the default end at now.
- An edit must retain the exact original occurrence and duration until changed.
  Errors must retain the draft. Retries of the same submission must retain its
  idempotency key; pending saves must block duplicate submission and navigation.
  Leaving unsaved changes must require an explicit discard choice.
- A successful save must open the saved detail, announce success, and refresh
  affected progress, timeline, statistics, and social projections. Completion
  from a disposed account must not change another account's presentation.

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
- Each timeline entry must open an addressable activity detail page with its
  Path, exact occurrence start/end, duration, occurrence time zone, and revision.
  Private notes and prior notes must be shown only to their owning participant.
- Edit history must retain all pages of revisions without repeating a cursor.
  Direct navigation and reload must recover the same authorized activity.
- The owning participant must be able to delete activity after reviewing the
  existing permanent-deletion warning. Cancel must receive initial focus.
  Failed deletion must retain the reviewed identity and retry key; an admitted
  request must prevent duplicate submission and navigation until it settles.
  Successful deletion must remove the deleted entry and exact removed feed events,
  apply authoritative progress, preserve unrelated loaded data, and announce
  success before returning
  to the timeline. An old account's completion must not affect a new account.

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

## Path encouragement and audience controls

- Studio must expose Path-specific encouragement from the selected participant
  in People, subject to the authoritative eligibility and visibility rules in
  [Nudges and Direct Encouragement](../social/nudges.spec.md).
- The composer must identify the recipient and Path, load current eligibility,
  and present the five approved presets as a keyboard-accessible single-choice
  group. Selecting a row must select its preset and enable Send; selection alone
  must not send. Cancel and Escape must leave the server unchanged.
- Goal-complete and rate-limited eligibility must display their localized reason
  without an enabled send action. Missing or rejected eligibility must fail
  closed with localized recovery and must not infer hidden audience or block data.
- An admitted send must prevent duplicate submission and navigation until its
  result is reconciled. An uncertain failure must retain the selected preset and
  reuse the same idempotency key for the same intent; changed recipients or
  presets must not reuse that intent. Success must be announced and refresh the
  selected participant's eligibility.
- Participants must be able to open their personal audience controls from their
  Path. The addressable destination must recover on direct navigation and reload,
  identify the Path, and offer Nobody, Path members, Followers, and Everyone.
  Supporters and nonmembers must not receive an editable personal audience.
- Audience changes must use the authoritative revision and stable retry intent.
  A conflict must require fresh server state before another save. Failed saves
  must retain the chosen audience for recovery; success must show the persisted
  value. Audience controls must never edit another participant's preference.
- All reads and writes must remain owned by the activated account and exact
  Path/recipient intent, cancel on disposal, and ignore late completions. Studio
  must adapt the shared client-core validation and command ownership behind its
  own domain ports, without importing generated API models into presentation.
- Composer and audience controls must reuse Studio form/dialog styling and shared
  locale catalogs, remain keyboard-operable, and reflow on narrow screens.

Acceptance: select a preset without sending, cancel without a write, retry an
uncertain send with the same key, observe the recipient's notification after
success, reject completed goals and unauthorized recipients, persist a personal
Nobody audience across reload, and prevent stale-revision audience overwrites.

## Voluntary Path leaving

- Studio must expose Leave Path only when the current authoritative capability
  permits it, never for the current creator or an archived Path. Opening the
  action must revalidate the Path and capabilities before admitting a command.
- The shared confirmation dialog must identify the Path. For participants and
  administrators, retaining activity must be the default. It must explain the
  loss of access, hidden retained activity, restoration on rejoining, and that
  a running timer is stopped and saved before departure.
- Delete My Activity must open a distinct destructive confirmation. It must
  explain permanent removal of that user's activity, progress, statistics,
  achievements and activity-derived feed events, no restoration on rejoining,
  and stopping/discarding any running timer without a final activity.
- Supporters must receive an ordinary leave confirmation without an activity
  choice or claims that their activity is retained/deleted.
- Cancel and Escape before admission must not change membership, activity or
  timers. Once admitted, duplicate submission and navigation must be guarded.
- An uncertain failure must retain the exact reviewed choice and idempotency
  key for retry, even if the server already removed membership. Switching
  activity outcomes must not be allowed after an uncertain admitted request.
  Dismissing and reopening the task in the same activated account must preserve
  that unresolved intent; a definitive server rejection may release it for a
  fresh capability review.
- Successful leave must clear stale Path, timer, activity, social and notification
  projections before returning Home with a localized completion notice. Late
  completion after account replacement or disposal must not affect another view.
- Studio must use its domain ports and shared client-core leave validation and
  operation ownership. Server authorization, atomic data changes, retention,
  timer behavior and notification cleanup remain authoritative under
  [Path Membership](../paths/membership.spec.md#leaving-a-path).

Acceptance: cancel preserves a running timer and membership; ordinary leave
saves that timer and hides retained history until rejoining; delete-and-leave
discards a running timer and removes prior activity permanently; supporters see
no irrelevant data choice; creator/archived capability rejection and uncertain
receipt retries remain fail-closed and deterministic.

## Existing Path visibility in Studio

- The creator's active Path management surface must show effective visibility
  and link to an addressable Studio visibility editor. Other roles and archived
  Paths must not expose that action.
- The editor must load the authoritative Path capability and authenticated
  profile privacy before offering choices. Direct routes must fail closed when
  that review is unavailable; API DTOs must remain inside adapters.
- The editor must follow [Visibility](../social/visibility.spec.md#changing-an-existing-paths-visibility):
  unchanged choices send nothing, narrowing is explicitly saved, and expansion
  uses the shared Cancel-first confirmation naming the reviewed transition and
  historical exposure. Cancellation sends no mutation.
- Submitted changes must retain the reviewed current/proposed audiences and a
  stable retry identity. Conflicts and loss of capability must require a fresh
  authoritative review. Transient failure must preserve the draft and support
  retry without implying that an uncertain response proves no server change.
- Pending changes must prevent duplicate admission and navigation. Account
  replacement or disposal must prevent late responses from changing the new
  account. Successful writes must publish the returned client-owned Path and
  invalidate affected account projections without discarding the active editor.
- Acceptance must cover private-profile audience limits; creator versus other
  roles and archival; expansion cancellation; explicit narrowing; same-intent
  retry; changed-state conflicts; and stale completion after disposal. Real API
  verification must show the saved audience and its visibility effects.
