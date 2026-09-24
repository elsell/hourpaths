# Social Feed

Status: Approved for implementation

## Purpose

Define the activity users may encounter in the social feed for accountability,
motivation, and encouragement.

## Feed event types

- The social feed must include eligible recorded practice sessions.
- The social feed must include eligible goal achievements.
- A practice-session event must identify the participant and path and summarize
  the time recorded.
- A goal-achievement event must identify the participant, path, and achievement.
- Exact interval and overall-target achievement transitions are defined in
  [Time Goals](../goals/time-goals.spec.md#achievement-event-lifecycle).
- Every eligible completed timer session must create its own practice-session
  event.
- Every eligible manual time entry must create its own practice-session event.
- Multiple entries by the same participant on the same path must not be combined
  into a daily or other aggregate feed event.
- A private activity note must never be copied into or exposed through its feed
  event.
- A feed event derived from activity first accepted after offline use or manual
  backdating must use the instant at which the server first accepts and
  publishes the event as its publication time, not the activity occurrence
  time.

### Chronological practice feed acceptance slice (`SOC-03A`)

- The initial Following feed must publish one stable practice-session event for
  every saved timer session and manual entry from an eligible source.
- Eligible sources for this slice are a followed participant whose underlying
  Path remains visible to the viewer, or a fellow tracking-capable participant
  on the same Path. Eligibility through both sources must still produce one
  event.
- Pending follow requests, supporters who do not independently follow the
  participant, blocked users in either direction, unrelated Paths, inactive
  accounts, and arbitrary users must not make an event eligible.
- Each event must show only the participant's safe public identity, Path name,
  recorded duration, server publication time, and whether a public activity
  field was edited. It must never expose the activity note or occurrence time
  as the publication time.
- The completed practice feed must use pure newest-first publication order with
  event ID as the deterministic tie breaker. Refresh and opaque cursor
  pagination must preserve that order, deduplicate events, and re-evaluate
  current authorization on every page.
- A public activity edit must update the existing event in place, display an
  **Edited** indicator, and preserve its identity and publication position. A
  note-only edit must not change the public event or mark it edited.
- Deleting the source activity must remove its event. Activity creation,
  idempotent replay, migration backfill, and deletion must not produce an
  orphaned or duplicate event.
- Following must present the feed as a compact native list with localized,
  accessible loading, empty, refresh, retry, and pagination behavior. People
  search and Follow Requests must remain directly reachable as dedicated
  native-stack destinations.
- Selecting a practice event must open its existing authorized activity-detail
  destination. A stale or newly unauthorized selection must use the ordinary
  opaque unavailable behavior.
- Goal-achievement events, currently running timers, reactions, comments,
  nudges, and visibility-editing controls remain outside this slice and keep
  the broader `SOC-03` feed outcome open.

### Goal-achievement feed acceptance slice (`SOC-03B`)

- Saving a completed timer session or manual entry must publish an interval-goal
  achievement when that activity first moves the participant from below the
  currently configured target to meeting or exceeding it in the applicable
  participant-calendar interval.
- The same activity must publish an overall-target achievement when it first
  moves the participant's accumulated time from below the currently configured
  target to meeting or exceeding it. One activity may legitimately publish
  both achievement types together with its ordinary practice-session event.
- Publication of the activity, practice-session event, and every newly earned
  achievement must commit atomically. An idempotent replay or additional time
  above the same supported target must not publish a duplicate achievement.
- Creating, lowering, or otherwise reconfiguring a goal must not publish an
  achievement by itself. A later genuine activity-driven crossing under the
  current configuration must remain eligible.
- Each achievement row must identify the safe participant and Path, distinguish
  interval from overall achievement, report the achieved target in whole
  seconds, and for an interval achievement retain the exact half-open occurrence
  boundaries used to establish it. Private activity notes must remain absent.
- Achievement and practice-session events must share one newest-first
  publication timeline with event ID as the stable tie breaker. Publishing an
  achievement must not reorder an older event, and refresh and opaque cursor
  pagination must deduplicate both event types.
- The current profile, Path, follow/shared-participation, block, and SpiceDB
  visibility rules must be re-evaluated for every achievement candidate. A
  hidden or inaccessible achievement must not disclose that it exists.
- Editing or deleting recorded activity must recalculate support for affected
  achievements atomically. An unsupported achievement and all of its engagement
  must be permanently removed without a removal notification; a later genuine
  crossing must create a new event identity rather than restore the removed
  event or engagement.
- The achievement itself must not create a notification for another user.
  Following must render it as a compact, localized, accessible native row with
  the HourPaths accent used sparingly for the achievement marker.
- Reactions, comments, comment hearts, reporting, and goal-reconfiguration
  presentation on achievement events remain outside this slice and keep
  `SOC-04`, `TIME-06`, and `TIME-08` open.

## Active following

- The top of the feed must show followed users who are currently tracking time
  with a running timer.
- Each active item must show how long the followed user has been tracking.
- An active timer must appear only when the current user is authorized to see
  its underlying path and activity.
- Stopping a timer must remove it from the active display and allow its completed
  session to appear as its own feed event when eligible.
- The active display must not include arbitrary users whom the current user does
  not follow.
- The active display must contain at most one grouped item per followed user.
- When that user has multiple active timers, their grouped item must list every
  active Path and live elapsed duration the current user is authorized to see.
- An inaccessible active timer must be omitted without hiding another timer from
  the same user that remains visible.
- Stopping one of several timers must remove only that timer from the grouped
  item; the item must remain while at least one visible timer is still running.

### Active following acceptance slice (`SOC-03C`)

- Following must present active tracking as a compact native section above the
  completed chronological feed. It must not interleave running timers with
  completed events.
- This slice must include only established followed users. Pending follow
  requests, shared-Path-only participants, blocked users in either direction,
  inactive accounts, and arbitrary users must not appear.
- Profile, Path, activity, and block visibility must be evaluated from current
  authoritative state. An inaccessible timer must be omitted without revealing
  that it exists and without hiding another timer for the same followed user
  that remains accessible.
- Each followed user must appear at most once. Their item must list every
  currently visible running Path with a live elapsed duration derived from the
  timer's authoritative start instant.
- Stopping one timer must remove only that timer. Stopping the last visible
  timer must remove the followed user's active item; the saved session may then
  appear exactly once through the existing completed-event feed behavior.
- Refresh, retry, foreground return, and late responses from a replaced session
  or profile must converge on the latest authoritative active set without
  replacing newer state.
- The native section must provide localized and accessible loading, empty,
  failure, and grouped active states using compact native controls.
- Goal-achievement events, feed interactions, nudges, timer-start notification
  preferences, web product UI, background polling, and generalized presence
  infrastructure remain outside this slice.

## Native Following presentation

- Following must use the system tab and navigation stack. Its navigation bar
  must inherit the system material and scroll-edge behavior rather than apply an
  opaque custom background, while ordinary feed content remains on a flat
  content layer without custom glass.
- Visible active tracking must form one compact native section above the
  completed chronological feed. Active people, their authorized Paths, and
  elapsed durations must remain grouped without visually merging with completed
  events.
- Practice-session and goal-achievement events must use flat,
  intrinsic-height native rows with restrained separators. Heavy rounded cards,
  decorative elevation, and nested content backgrounds must not surround every
  event.
- Each event's reading order must be participant identity, Path and event kind,
  duration or achievement, publication metadata, then available disclosure and
  encouragement actions. Each value and action must be exposed once to
  VoiceOver without a parent accessible group suppressing its children.
- Long names, Path names, localized durations, goal descriptions, and action
  labels must stack or reflow at maximum Dynamic Type without clipping,
  overlapping, fixed-height compression, or horizontal content scrolling.
- Initial loading, stale useful content, account-empty feed, active-empty state,
  retryable failure, refresh, and pagination must remain visibly distinct. One
  section's failure must not erase useful authoritative content in the other.
- Cold or direct activity, comments, and interaction-roster routes must mount a
  visible localized loading or recovery state even when Following presentation
  context has not yet been installed. They must not return a blank route while
  their authoritative target and owning account/profile lineage resolve.
- Full reaction, comment-composer, comment-editing, edit-history, and heart-roster
  presentation polish remains a separate UI-28 interaction slice. UI-28H must
  preserve those authorized entry points and their state without expanding
  their secondary workflows.

## Feed sources

- The feed must include eligible events from users the current user follows.
- Follow-based events must still satisfy the profile, path, and activity
  visibility rules applicable to the current user.
- The feed must include eligible events from fellow participants within a shared
  path even when those participants do not follow one another.
- Shared-path participation must expose only eligible events from that shared
  path; it must not expose the participant's unrelated paths.
- An event eligible through both following and shared-path participation must
  appear only once in the feed.
- The product must not provide a global feed of activity from arbitrary users
  whom the current user neither follows nor shares a path with.

## Ordering

- The completed-event feed must be purely chronological and ordered by feed
  event publication time, newest first.
- The product must not rank, recommend, pin, or reorder completed feed events
  according to engagement, predicted relevance, Path relationship, or event
  type.
- Adding a comment or reaction must not move an existing event upward.
- Editing an activity or comment must not change the event's publication time or
  position in the chronological feed.
- Events with the same publication time must use a deterministic stable order so
  pagination and refresh do not make them jump relative to one another.
- Currently running timers must remain in the separate active display above the
  completed chronological feed and must not be interleaved as completed events.

## Visibility

- A feed event must be shown only to users authorized to see the underlying
  profile, path, and activity.
- Feed events must not be shown between two users when either has blocked the
  other, even when they remain participants in the same path.
- A feed event must not make an otherwise inaccessible path or activity
  discoverable.
- A later reduction in visibility must prevent the affected event from being
  shown to users who no longer have access.
- When path visibility expands, all existing eligible events must become
  available to the newly admitted audience automatically.
- Historical events made available by expansion must remain subject to current
  authorization and blocking rules.

## Source activity changes

- Editing a recorded activity entry must update its existing feed event in place
  rather than create a replacement or additional event.
- An event updated because its source activity changed must display an
  **Edited** indicator.
- An authorized user must be able to inspect the source activity's edit
  history from the edited event.
- Editing the source activity must not send a notification about the edit.
- Deleting a recorded activity entry must delete its feed event and all comments
  and reactions attached to that event.
- Deleting a feed event must also invalidate every interaction replay receipt
  associated with that event. A later exact retry must fail opaquely and must
  not recreate, return, or reveal deleted engagement.
- Deleting the source entry or its feed event must not send a notification about
  the deletion.
- If an activity edit or deletion invalidates a goal achievement, the associated
  achievement event and all of its comments and reactions must also be deleted.
- Invalidating an achievement must not send a notification about the removal.
- A later genuine achievement must create a new event without restoring
  engagement from the invalidated event.

## Encouragement interactions

- An authorized user must be able to react to a practice-session event.
- An authorized user must be able to react to a goal-achievement event.
- Available reactions must come from a curated, positive-only set intended to
  encourage the participant.
- The initial feed-event reaction set must be `Heart`, `Applause`, `Fire`,
  `Strong`, and `Celebrate`, presented respectively as ❤️, 👏, 🔥, 💪, and 🎉.
- The initial product must not allow custom reaction types outside that set.
- The product must not offer negative or discouraging reactions.
- A user may hold at most one reaction on a feed event. Selecting a different
  reaction must replace that user's existing reaction without affecting other
  users' reactions, and removing it must leave no reaction from that user.
- A reaction replacement must update the visible counts immediately without
  moving the event in chronological order or creating an additional
  notification for the superseded reaction.
- An authorized user must be able to comment on a practice-session event.
- An authorized user must be able to comment on a goal-achievement event.
- An authorized user must be able to heart an individual comment.
- A heart must be a Boolean state per user per comment: a user may contribute
  at most one heart to a comment and may remove that heart.
- Each comment must display its current heart count next to the heart control.
- Selecting the heart count must show the users whose hearts make up that
  count, subject to the current user's authorization to see those users.
- Hearting a comment must create an eligible notification for the comment
  author in the dedicated comment-heart notification channel.
- Removing a heart must update the comment's count and heart roster silently.
- Removing a heart must permanently remove the corresponding in-application
  heart notification when one has already been created.
- Removing a heart must not create a new notification about the removal.

### Practice-event reaction acceptance slice (`SOC-04A`)

- An authorized user may add, replace, or remove one of the five curated
  reactions on an eligible practice-session event.
- The feed must return authoritative per-reaction counts and the current user's
  selected reaction without exposing private reaction-roster data.
- Missing, deleted, blocked, or newly inaccessible events must fail opaquely and
  must not create, replace, or remove a reaction or reveal that an event exists.
- Adding or replacing a reaction must not change the event's identity,
  publication time, or chronological position. Idempotent retries must converge
  on one reaction from that user.
- A reaction by someone other than the event owner must become eligible for one
  owner notification only after the five-second social-interaction grace period.
  Replacing it during that period must retain one pending notification for the
  current reaction; removing it before eligibility must prevent delivery, and
  removing it later must delete its in-application notification without sending
  a removal notice.
- A user must not receive a notification for reacting to their own event.
- Following must expose reaction selection as a compact, localized, accessible
  native control with the HourPaths accent reserved for the selected state.
- Goal-achievement reactions, reaction rosters, comments, comment hearts,
  reporting, and profile-wide interaction controls remain outside this slice
  and keep `SOC-04` open.

### Practice-event comment acceptance slice (`SOC-04B`)

- An authorized user may open a dedicated comments destination from an eligible
  practice-session event and page its currently visible comments in stable
  chronological order, oldest first.
- An authorized user may post a comment only when its normalized text satisfies
  the comment validation rules below. A successful post must return the
  authoritative comment and must not change the feed event's identity,
  publication time, or chronological position.
- A comment author may edit their own comment. Every successful edit must retain
  the prior version, mark the comment as edited, and make its immutable edit
  history available from the ordinary comment row without moving the comment.
- A comment author or the owner of the source event may delete the comment.
  Path ownership or administration alone must not grant deletion authority.
- Missing, deleted, blocked, or newly inaccessible events and comments must fail
  opaquely without creating, editing, deleting, or revealing a comment.
  Idempotent mutation retries must converge on one authoritative result.
- A comment by someone other than the event owner must become eligible for one
  owner notification only after the five-second social-interaction grace period.
  Editing during that period must retain one pending notification for the
  current comment; deleting before eligibility must prevent delivery, and
  deleting later must remove the related in-application notification silently.
- A user must not receive a notification for commenting on their own event, and
  editing a comment must not create an additional notification.
- Mobile must present comments as a dedicated native-stack destination with a
  compact chronological list, native text input and actions, keyboard-safe
  composition, localized loading/empty/error states, and accessible edit,
  history, and deletion controls. The HourPaths accent must remain reserved for
  the primary or selected action.
- Goal-achievement comments, comment hearts and rosters, reporting, and
  profile-wide comment controls remain outside this slice and keep `SOC-04`
  open.

### Practice-comment heart acceptance slice (`SOC-04C`)

- An authorized user may add or remove one Boolean heart on a currently visible
  comment attached to an eligible practice-session event. Idempotent retries
  must converge on one heart from that user without changing comment or feed
  ordering.
- Every visible comment must return its authoritative heart count and whether
  the current user has hearted it. Mobile must reconcile optimistic heart state
  with that authoritative result without allowing an older response to
  overwrite a newer user intent.
- Missing, deleted, blocked, or newly inaccessible events and comments must
  fail opaquely and must not create, remove, count, or reveal a heart.
- Hearting another user's comment must become eligible for exactly one
  comment-author notification in the dedicated comment-heart channel after the
  five-second social-interaction grace period. A user must not receive a
  notification for hearting their own comment.
- Removing a heart before eligibility must prevent delivery. Removing it after
  eligibility must permanently remove its in-application notification and must
  not create a removal notification.
- Selecting a positive heart count must open a dedicated native-stack roster
  destination that pages current hearters in stable order and exposes only
  public-profile fields the viewer remains authorized to see. A stale page must
  not restore a removed heart or an identity that has become inaccessible.
- Deleting the source comment or event must permanently remove its hearts and
  heart notifications without notifying the former hearters or comment author.
- Mobile must present a compact, localized, accessible heart control and roster
  using native actions and rows, with the HourPaths accent reserved for the
  current user's selected heart state and primary actions.
- Goal-achievement comment hearts, reporting, moderation, and profile-wide
  interaction controls remain outside this slice and keep `SOC-04` open.

## Native feed interaction presentation

- Reaction selection must use one compact standard native menu or popover with
  the five curated choices, the current selection, removal when applicable, and
  busy or unavailable state. It must not render five competing custom pills or
  add glass to the event content layer.
- Comments and each comment-heart roster must use dedicated native-stack
  destinations with a localized current-view title, platform back control, and
  ordinary edge-swipe behavior whenever dismissal is safe. Navigation and task
  chrome must inherit Liquid Glass without an opaque custom bar background.
- Comments and roster identities must use flat, intrinsic-height native rows
  with restrained separators. Author, edited state, text, heart state and count,
  available actions, and privacy-safe roster identity must follow logical
  reading order and each be exposed once to VoiceOver.
- Comment creation must use a keyboard-safe native composer whose text and send
  action remain visible and reachable at every supported Dynamic Type size. It
  must use one native keyboard-inset strategy and must not combine a hard-coded
  keyboard offset with automatic keyboard insets.
- Comment editing must use a standard single-view native task with leading
  Cancel and trailing Save-or-Retry actions. Validation and network failure must
  preserve the complete draft, identify the error in text, and keep retry
  reachable without dismissing the keyboard.
- A clean composer or edit task must permit ordinary back or swipe dismissal. A
  meaningful dirty draft must be preserved or receive confirmation before it is
  discarded, and an admitted mutation must not be dismissed until its
  authoritative success or failure reconciles. Dismissal protection must exist
  only for the duration of that concrete risk.
- Cold or direct Comments and heart-roster routes must mount localized loading,
  unavailable, or retry presentation before authoritative target resolution and
  must never render a blank route because Following state is absent. Results and
  dismissal state must remain owned by the account/profile session lineage and
  exact event, comment, roster, draft, and admitted intent.
- Reaction, comment, edit-history, heart, and roster presentation must reflow at
  maximum Dynamic Type and remain operable with VoiceOver, Reduce Motion, Reduce
  Transparency, and Increase Contrast. Selection, progress, errors, and success
  must not depend on animation, translucency, emoji, or accent color alone.

## Comment ownership and deletion

- A comment must contain between 1 and 2,000 characters after surrounding
  whitespace is trimmed and its text is normalized to Unicode NFC.
- A comment may contain line breaks, ordinary Unicode text, and emoji but must
  not contain control characters other than ordinary line breaks.
- A whitespace-only or over-length comment must be rejected rather than posted
  or silently truncated.
- Comment creation and editing must use the same validation rules.

- A comment author must be able to edit or delete their own comment.
- An edited comment must display an `Edited` label in the ordinary comment
  display.
- A user who can view an edited comment must be able to open and view that
  comment's edit history.
- Editing a comment must retain its earlier versions so that the visible edit
  history is not reconstructed from only the latest text.
- The participant whose recorded activity or achievement produced an event must
  be treated as that event's owner.
- An event owner must be able to delete any comment on their event.
- An event owner must not be able to edit another user's comment.
- Creating or administering the path must not grant comment-deletion authority
  over an event owned by another participant.
- A path creator or administrator may delete a comment on an event only when
  they are independently the comment author or the owner of that event.
- Users who cannot delete a comment must still be able to report it when they
  otherwise have access to it.

## Profile-wide comment control

- Each user must have one profile-wide setting that allows or disallows comments
  on every feed event they own.
- The setting must default to comments enabled for a new user.
- The initial product must not require separate comment settings for individual
  events or paths.
- When comments are off, other users must not be able to add a new comment to
  any event owned by that user.
- Turning comments off must hide every existing comment on events owned by that
  user from the ordinary event display.
- Turning comments off must not delete the hidden comments from storage.
- Turning comments back on must restore the hidden comments to their events.
- A comment deleted by its author, the event owner, source-event deletion, or
  another deletion rule must not be restored when comments are turned on.
- Deleting a comment must permanently delete every heart on that comment.
- Deleting a comment must permanently remove every in-application notification
  whose target is the comment or one of its hearts.
- These cascade removals must not notify the comment author or users who hearted
  the comment.
- Changing the setting must not notify commenters or other users.
- Turning comments off must permanently remove in-application notifications
  whose target is a comment hidden by that setting.
- Turning comments back on must not restore the removed notifications.
- The profile-wide setting must not disable reactions unless reactions are
  separately disabled by another specified rule.
- The setting must not remove, hide, or prevent comments the user writes on an
  event owned by someone else.
- Path creators and administrators must not be able to override a participant's
  profile-wide comment choice for events that participant owns.

## Profile-wide reaction control

- Each user must have a separate profile-wide setting that allows or disallows
  reactions on every feed event they own.
- The setting must default to reactions enabled for a new user.
- The initial product must not require separate reaction settings for individual
  events or paths.
- When reactions are off, other users must not be able to add a new reaction to
  any event owned by that user.
- Turning reactions off must hide every existing reaction on events owned by
  that user from the ordinary event display.
- Turning reactions off must not delete the hidden reactions from storage.
- Turning reactions back on must restore the hidden reactions to their events.
- A reaction removed by its author, source-event deletion, or another deletion
  rule must not be restored when reactions are turned on.
- Changing the setting must not notify users who reacted or other users.
- Turning reactions off must permanently remove in-application notifications
  whose target is a reaction hidden by that setting.
- Turning reactions back on must not restore the removed notifications.
- The reaction setting must not change the user's comment setting.
- The reaction setting must govern only reactions placed directly on feed
  events; it must not disable or hide hearts placed on individual comments.
- The setting must not remove or prevent reactions the user leaves on an event
  owned by someone else.
- Path creators and administrators must not be able to override a participant's
  profile-wide reaction choice for events that participant owns.

## Native interaction-settings presentation

- The two profile-wide controls must appear together in a titled Interactions
  native-stack destination reached from the compact Settings hierarchy.
- Comments and reactions must use independent standard native switches with
  explicit accessible names and values; updating one must not visually or
  semantically change the other.
- Loading, retained authoritative values, saving, rollback, failure, and retry
  must remain distinct and localized. A cold or direct route must mount visible
  recovery and must never render blank because Home's in-memory presentation
  context is absent.
- State and admitted mutations must remain owned by the activated account and
  profile session lineage and exact setting intent. Same-account credential
  rotation may preserve owned work; replacement or sign-out must clear the
  prior account's values and cancel late load, save, rollback, or retry results.
- The hierarchy and controls must reflow at maximum Dynamic Type and remain
  operable with VoiceOver, Reduce Motion, Reduce Transparency, and Increase
  Contrast without relying on custom glass, animation, or color alone.
