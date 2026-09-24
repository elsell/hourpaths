# Notifications

Status: Approved for implementation

## Product status

- The initial product must include in-application and operating-system push
  notifications as specified here.
- Required workflows must remain usable by opening the application even when a
  notification is delayed, disabled, unsupported, or not delivered.

## General behavior

- Social and access notifications must be delivered both in the application's
  notification experience and through operating-system push notifications.
- The in-application notification experience must be a dedicated secondary
  screen reached from the bell entry point and must not expand inline on Home.
- A push notification and its in-application counterpart must represent the
  same underlying event rather than appearing as unrelated notifications.
- A notification must identify the event that caused it in concise,
  understandable language.
- Opening a notification should take the user to the relevant in-application
  context when that context remains accessible.
- Notification content must not disclose private path or activity information to
  a device user who is not authorized to see it.

## Urgency and actionability

- Notification presentation must distinguish actionable or time-sensitive
  alerts from informational notices.
- Urgent presentation must be reserved for conditions that require or strongly
  benefit from timely user action; a completed state change must not appear
  urgent merely because it is important to disclose.
- An informational notice must state concisely what changed, which relevant
  object was affected, and the practical result for the user.
- Completed Path role changes must be informational notices. Invitations,
  pending ownership-transfer requests, or other events that require a decision
  may use an actionable presentation.
- Push delivery for an informational notice must be requested as quiet delivery,
  without an application-requested sound, vibration, time-sensitive
  interruption, or high-priority presentation.
- The in-application notification experience must place informational notices
  in a distinct section from notifications that are awaiting or recommending a
  user action.
- The separate informational section must make the difference in urgency clear
  without implying that informational changes are errors or warnings.
- A notification's informational or actionable presentation class must remain
  distinct from its user-controlled notification channel; for example, a role
  change and a Path invitation may share the membership/access channel while
  appearing in different in-application sections.

## Push permission

- Denying or revoking operating-system push-notification permission must not
  disable in-application notifications.
- Push permission state must not prevent starting, continuing, stopping, or
  saving timers.
- Product workflows must continue to function normally apart from the
  operating-system notification presentations the user has disallowed.
- The application must provide an easy-to-find notification-permission control
  in settings at all times.
- When the operating system permits another permission request, that control
  must allow the user to request permission again.
- When the operating system requires the user to change permission externally,
  the control must explain this plainly and open the relevant system settings.
- The application may show a quiet, non-blocking indication that push is
  disabled but must not repeatedly interrupt ordinary tracking to request it.

## Recurring unavailable period

- While the profile-wide recurring unavailable period is active, the
  application must suppress operating-system push delivery for every
  notification category.
- The suppression must apply equally to social interactions, access and
  membership events, timer activity, nudges, achievements, goal-aware
  reminders, no-longer-achievable notices, and long-running-timer notices.
- An enabled notification channel or context-specific preference must not
  override the active unavailable period.
- The unavailable period must not prevent the underlying in-application
  notification from being created and shown when its channel is enabled.
- A push suppressed because of an active unavailable period must be discarded,
  not queued for later delivery.
- Ending or disabling the unavailable period must not cause catch-up push
  delivery for notifications that occurred while it was active.
- The corresponding in-application notifications must remain available under
  their ordinary retention, visibility, and channel rules.
- The period's profile behavior and time-zone interpretation are defined in
  [User Preferences](../accounts/preferences.spec.md#recurring-unavailable-period).

## Deletion of referenced content

- Deleting a path or feed event must automatically remove every related
  in-application notification.
- A removed notification must not remain as an unavailable historical notice.
- Removing related notifications because their target was deleted must not
  generate another notification about that removal.
- Deleting one notification target must not remove notifications for an
  unrelated path or feed event.
- A standalone informational notice that a Path was deleted must not be treated
  as a notification targeting the deleted Path and must remain readable without
  providing a link to the former Path.

## Loss of access to referenced content

- When a user loses access to content without that content being deleted, every
  in-application notification whose purpose is to open that content must be
  removed.
- An informational notification whose purpose is to explain the access, role,
  or visibility change must remain under its ordinary retention rules.
- An operating-system push that was already delivered cannot be required to
  disappear from the device.
- Opening an already-delivered push whose target was deleted or became
  inaccessible must use the ordinary opaque unavailable result and must not
  disclose stale private content.

### Manager-removal acceptance slice (`NOTE-03A`)

- Given a participant has a Path-targeting nudge notification and an unrelated
  notification, when a creator or administrator removes that participant from
  the Path, the nudge notification must disappear and the participant's unread
  count must converge to the remaining authoritative notifications.
- The informational `path_member_removed` notification must remain and identify
  the Path, actor, removed role, and practical loss of access without exposing
  content from the now-inaccessible Path.
- The unrelated notification must remain unchanged.
- Reading the removed nudge by its exact notification identifier must return
  the same opaque unavailable response as reading a nonexistent notification.
- Push delivery for the removed nudge that has not been handed off to the push
  provider must be suppressed; a push already handed off remains subject to the
  general loss-of-access rule above.

### Voluntary-leave acceptance slice (`NOTE-03B`)

- Given a participant has a Path-targeting nudge notification and an unrelated
  notification, when that participant voluntarily leaves the private Path, the
  nudge notification must disappear and the participant's unread count must
  converge to the remaining authoritative notifications.
- The informational `path_member_left` notification for the creator and current
  administrators must remain and identify the Path and departing participant.
- The unrelated notification must remain unchanged.
- Reading the removed nudge by its exact notification identifier must return
  the same opaque unavailable response as reading a nonexistent notification.
- Push delivery for the removed nudge that has not been handed off to the push
  provider must be suppressed; a push already handed off remains subject to the
  general loss-of-access rule above.

### Retained-activity interaction acceptance slice (`NOTE-03C`)

- Given a participant has a target-opening interaction notification for their
  activity on a private Path and an unrelated notification, when the participant
  voluntarily leaves while retaining that activity, the interaction
  notification must disappear and the participant's unread count must converge
  to the remaining authoritative notifications.
- The creator's informational `path_member_left` notification and the
  participant's unrelated notification must remain unchanged.
- Reading the removed interaction notification by its exact identifier must
  return the same opaque unavailable response as reading a nonexistent
  notification.
- Push delivery for the removed interaction notification that has not been
  handed off to the push provider must be suppressed.
- Rejoining the Path may restore the retained activity and its feed event under
  ordinary access rules, but must not resurrect the removed interaction
  notification or its suppressed push work. A later eligible interaction is a
  new event and may create a new notification normally.

### Visibility-contraction interaction acceptance slice (`NOTE-03D`)

- Given a nonmember can open a Path-targeting interaction notification through
  the Path's public or followers audience, when the creator narrows visibility
  so that recipient loses effective Path access, the interaction notification
  must disappear and the recipient's unread count must converge to the
  remaining authoritative notifications.
- Unrelated notifications and informational `path_visibility_changed`
  notifications that remain accessible must remain unchanged.
- Reading the removed interaction notification by its exact identifier must
  return the same opaque unavailable response as reading a nonexistent
  notification.
- Push delivery for the removed interaction notification that has not been
  handed off to the push provider must be suppressed, and stored delivery-token
  material must be scrubbed.
- Re-expanding the Path may restore access to its eligible history under current
  authorization rules, but must not resurrect the removed notification or its
  suppressed push work. A later eligible interaction is a new event and may
  create a new notification normally.

## In-application retention and user deletion

- In-application notifications must be retained indefinitely regardless of age
  or read state unless the user deletes them or another specified lifecycle rule
  requires their removal.
- The application must not automatically expire an ordinary in-application
  notification after a fixed number of days.
- A user must be able to permanently delete any notification from their own
  in-application notification history.
- Deleting a notification must not delete, undo, accept, reject, or otherwise
  mutate the underlying event or content.
- Deleting an actionable notification must not resolve its underlying request;
  an unresolved request must remain reachable from its relevant product surface.
- User deletion of an in-application notification cannot be required to recall
  an operating-system push that has already been delivered.

## Ordering and read state

- Within each actionable or informational section, in-application notifications
  must be ordered chronologically with the newest notification first.
- An authenticated notification-history read must return only the receiving
  user's non-deleted notifications. The initial Path-invitation projection must
  expose the notification ID and type, actionable or informational presentation
  class, read state, creation time, the actor's always-public user ID, canonical
  username, and display name, and the referenced Path ID and name, invitation ID,
  and offered participant or supporter role. It must not expose provider email,
  profile visibility, or other profile data.
- Notification-history traversal must use signed, receiving-user-bound,
  snapshot-stable keyset pagination ordered newest first. Soft-deleted
  notifications must be excluded, and malformed or mismatched actor, Path, or
  invitation context must fail closed.
- Every notification-history page must expose the receiving user's
  authoritative unread in-application notification count. The count must be
  independent of the requested page size and cursor, must exclude deleted or
  lifecycle-hidden notifications, and must describe the same authorized
  snapshot as the returned page.
- Viewing notification history must emit the authenticated read audit required
  by the platform architecture and must not itself change read state.
- Opening a specific notification must mark that notification as read.
- Marking an already-read notification as read must succeed without changing
  its original read timestamp.
- Opening the notification bell or viewing the notification list must not, by
  itself, mark any notification as read.
- The notification experience must provide a `Mark all as read` action that
  marks every currently unread in-application notification as read across both
  sections.
- Marking a notification as read must not delete it, resolve an underlying
  request, or change the underlying event.
- A read or delete mutation must be scoped to the authenticated receiving user.
  A missing, already-deleted, or other user's notification identifier must
  produce the same opaque unavailable result.
- `Mark all as read` must be idempotent, must affect only the authenticated
  receiving user's non-deleted notifications, and must succeed when there are
  no unread notifications.
- Read, delete, and `Mark all as read` mutation responses must expose the
  receiving user's resulting unread in-application notification count so every
  signed-in client can converge its badge without inferring state locally.
- Each permitted authenticated notification mutation attempt must use the
  principal limiter and emit the platform audit event; a successful mutation
  and its audit event must commit atomically.
- Read state must belong to the receiving user and remain synchronized across
  their signed-in devices.
- The web notification history and bell must converge to server truth after a
  successful notification mutation in another tab signed in as the same user.
  A successful local mutation must publish only a content-free convergence
  signal scoped to that user; it must not publish notification identifiers,
  content, counts, or mutation results, and a signal for another user must have
  no effect.
- The web notification history and bell must also refresh from server truth
  when the page regains focus or becomes visible so changes from another device
  converge without requiring a reload. Repeated convergence triggers must not
  create duplicate concurrent refreshes.
- Every successful, still-owned web notification mutation must queue a refresh
  through that same serialized convergence boundary after applying and
  signaling the mutation. This trailing authoritative refresh must prevent a
  page-one response admitted before the mutation from remaining visible after
  the mutation succeeds.
- A web convergence refresh must request the first history page and replace the
  visible history from an empty state only after that request succeeds, so
  remote deletions, read state, newest-first ordering, next-page cursor, and the
  authoritative unread count converge together. It must not mark any
  notification as read.
- A failed web convergence refresh must preserve the visible history, cursor,
  and last authoritative unread count and expose the ordinary history retry.
  Refresh completion must be ignored after the session or signed-in user
  changes, and web convergence listeners and channels must be removed when the
  page unmounts.
- The native notification experience must allow the user to refresh the
  current history using the platform's standard pull-to-refresh interaction and
  must refresh when the application returns to the foreground while that
  experience is active.
- A successful refresh must replace the visible history from the first page so
  read changes and deletions made on another signed-in device appear promptly,
  while preserving the server's newest-first ordering and next-page cursor.
- A failed refresh must preserve the already-visible history, cursor, and last
  authoritative unread count and must allow the user to retry. Refreshing must
  not itself mark a notification as read.
- The application-icon badge, where supported and permitted, must represent the
  receiver's current unread in-application notification count and no unrelated
  metric.
- Reading, deleting, or lifecycle-removing a notification must update the badge
  count promptly.

## Foreground presentation

- When the application is in the foreground, a new notification must still
  update in-application notification state and unread counts.
- If the user is already viewing the directly relevant content, the application
  must update that content quietly rather than presenting a redundant intrusive
  foreground banner.
- An actionable notification for content the user is not currently viewing
  must retain the ordinary operating-system foreground presentation permitted
  by its notification channel and operating-system settings.
- An informational notification must remain quiet in the foreground even when
  the user is viewing unrelated content.
- Foreground handling must not mark a notification read unless the user opens
  or is already viewing the specific relevant item.
- If the current session, receiving user, notification target, or foreground
  route cannot be resolved authoritatively, foreground handling must refresh
  notification history without navigating, disclosing target content, or
  marking the notification read.

## Native notification history and controls

- The notification bell must open a dedicated titled native-stack destination
  with the platform back control and ordinary edge-swipe behavior. Its bar and
  actions must inherit the system material and scroll-edge behavior rather than
  apply an opaque custom background.
- Pending Invitations must remain a compact, independently recognizable native
  navigation row. Mark All Read must remain a stable native bar action while
  work is admitted, with disabled or busy state and progress announced
  separately rather than making the action disappear.
- Actionable and informational history must use distinct flat native sections
  with intrinsic-height rows and restrained separators. Heavy rounded cards,
  decorative elevation, and custom glass must not wrap either section.
- Each row must expose actor and event summary, read state, creation time,
  available disclosure, and the standard delete action in logical VoiceOver
  order. Read state must remain perceivable without depending on the unread dot,
  font weight, or accent color alone.
- Loading, stale useful history, empty, unavailable, retry, pull-to-refresh,
  foreground refresh, pagination, and mutation progress must remain distinct and
  localized. A failed refresh or page must preserve the last useful history,
  cursor, and authoritative unread count.
- Notifications and notification-specific settings must mount visible localized
  loading or recovery on a cold or direct route rather than return blank or
  silently replace the intended destination with Home while their presentation
  context resolves.
- Notification settings must present push-permission state and its
  request-again or open-system-settings action with standard native rows and
  controls. The existing Nudges channel must use one native switch with
  selected, busy, failure, and retry semantics.
- UI-28J is presentation work over the controls that currently exist. It does
  not satisfy or implement the complete ten-channel catalog in
  [User-controlled notification channels](#user-controlled-notification-channels),
  and it does not add the recurring unavailable-period control. Those gaps must
  remain explicit for UI-28K or their owning functional slices.
- Route and mutation work must remain owned by the receiving account/profile
  session lineage and exact notification, permission, channel, or admitted
  intent. Same-account credential rotation must preserve valid work, while
  replacement or sign-out cancels presentation ownership and ignores late
  results.
- Long localized notification copy, dates, permission explanations, and channel
  labels must reflow without clipping or horizontal scrolling at maximum Dynamic
  Type. VoiceOver, Reduce Motion, Reduce Transparency, and Increase Contrast
  must preserve section identity, selection, progress, failure, and actions
  without depending on translucency, animation, or color alone.

## Disabled comments and reactions

- When an event owner disables comments, in-application notifications targeting
  comments hidden by that setting must be removed permanently.
- When an event owner disables reactions, in-application notifications targeting
  reactions hidden by that setting must be removed permanently.
- Re-enabling comments or reactions must restore the underlying hidden
  interactions but must not restore their removed notification entries.
- The application cannot require an already-delivered operating-system push to
  be recalled from the device.
- Opening an already-delivered push for a hidden interaction must open the event
  when it remains accessible and explain that comments or reactions are
  currently disabled without revealing the hidden interaction.
- No new comment or reaction notification may be generated while that
  interaction type is disabled because the corresponding new interaction is not
  permitted.

## Social-interaction notification grace period

- Notifications caused by a comment heart, a reaction placed directly on a
  feed event, or a newly posted comment must wait five seconds after the
  interaction before becoming eligible for in-application or push delivery.
- If the originating heart or reaction is removed during that grace period, its
  notification must never be created or delivered.
- If the originating comment is deleted during that grace period, its
  notification must never be created or delivered.
- The delay must apply to the notification only; the interaction itself must
  appear immediately to authorized users.
- Notification types outside these reversible social interactions, including
  timer activity, access events, nudges, reminders, and long-running-timer
  notices, must not inherit this delay merely because they share the
  notification system.

## Initial notification categories

- The initial notification plan must include social and access events.
- Social and access notifications must support events such as follow requests,
  path invitations, ownership-transfer requests, reactions, and comments.
- The initial notification plan must include optional practice reminders.
- Practice reminders are defined in
  [Goal-Aware Practice Reminders](reminders.spec.md).
- The initial notification plan must include the average-based long-timer
  notification defined in
  [Recorded Activity](../tracking/recorded-activity.spec.md#unusually-long-timer-notification).

## User-controlled notification channels

- Every notification type must belong to a clearly named notification channel.
- A user must be able to enable or disable each channel independently through
  notification settings stored with their account profile.
- Every notification channel must default to enabled for a new account.
- Each channel must have one user-facing toggle that controls both
  in-application and push delivery; the initial product must not expose separate
  delivery toggles for the two methods.
- Operating-system push permission remains an additional delivery constraint:
  an enabled channel may still produce its in-application notification when
  push permission is unavailable.
- The complete initial channel catalog must contain:
  - `Following` for new followers, follow requests, and accepted follow requests;
  - `Path access` for Path invitations, ownership transfers, membership changes,
    role changes, and broader Path-visibility changes affecting a
    private-profile participant;
  - `Tracking activity` for eligible timer-start activity;
  - `Achievements` for interval-goal and overall-target achievements;
  - `Event comments` for comments on the user's feed events;
  - `Event reactions` for reactions placed directly on the user's feed events;
  - `Comment hearts` for hearts on the user's comments;
  - `Nudges` for direct Path-specific encouragement;
  - `Goal reminders` for last-chance reminders and no-longer-achievable notices;
    and
  - `Timer health` for unusually long running-timer notices.
- One channel's toggle must not enable or disable another channel.
- A disabled account-level channel must suppress notifications in that channel
  even when a more specific preference for a followed user or shared path would
  otherwise allow them.
- A more specific followed-user or shared-path preference may further narrow an
  enabled channel but must not override a disabled account-level channel.
- Context-specific controls must be offered only where the corresponding
  relationship or product context calls for them; a globally enabled channel
  means notifications are allowed through that channel, not that every
  possible source must notify the user.
- Disabling a channel must not disable or remove the underlying product feature
  or prevent the user from reaching the corresponding information in the app.

## Timer-start notifications

- Starting a timer must create an eligible social notification event for other
  users who are authorized to see that timer's path and activity.
- A follower must receive that notification only when they have enabled
  activity notifications for the participant they follow.
- A participant in the same shared path must default to receiving notifications
  when another participant starts a timer on that path.
- Each participant must be able to disable or re-enable the default shared-path
  timer-start notifications they receive.
- The participant default must not require the two users to follow one another.
- A user who is eligible through both an enabled follow preference and shared
  path participation must receive only one notification for the timer start.
- A block between two users must suppress timer-start notifications in both
  directions, including when they remain participants in the same path.
- A participant must not receive a notification for starting their own timer.
- Timer-start notifications must support operating-system push delivery.

## Exact event behavior

### Following

- A new follower of a public profile must notify the followed user.
- A new follow request must create an actionable notification for the private
  profile owner.
- Accepting a follow request must notify the requester.
- Every following notification must identify the user whose action caused the
  follow or request as both its actor and follow subject; it must not identify
  the receiving user as the follow subject.
- Rejecting or canceling a follow request, unfollowing, and removing a follower
  must not create a notification.

### Path invitations and ownership

- A participant or supporter invitation must create an actionable notification
  for the recipient.
- An ordinary Path invitation notification must identify the inviter, Path, and
  offered role without granting access to the Path before acceptance.
- Opening an unresolved ordinary invitation notification must open the
  invitation decision context; deleting the notification must leave the
  invitation reachable from the recipient's invitation list.
- Accepting an invitation must create exactly one informational notification
  for the inviter that identifies the accepting user, Path, and accepted role.
- Rejecting or canceling an ordinary invitation must not create a notification.
- Canceling an ordinary invitation must remove its actionable in-application
  notification and suppress any not-yet-handed-off push delivery atomically
  with the cancellation; unrelated notifications and push work must remain.
- An ownership-transfer request must create an actionable notification for the
  proposed recipient.
- Accepting a transfer must notify the former creator.
- Declining, canceling, or expiring a transfer must update its in-application
  state for the current creator.
- Transfer acceptance and expiration must support push delivery; decline and
  cancellation must remain in-application only.

### Membership and Path lifecycle

- A completed role change or access removal must notify the affected user
  informationally and state its practical effect.
- When a user voluntarily leaves a Path, the creator and current administrators
  must receive an informational notification.
- Archiving or unarchiving a Path must notify every current participant and
  supporter other than the creator who performed the action.
- Deleting a Path must notify every former participant and supporter other than
  the creator who performed the deletion, using a standalone notice that cannot
  navigate to the deleted Path.
- The standalone Path-deletion notice must be an informational access notice
  delivered in application and requested as quiet push delivery. It must retain
  the former Path name and deleting creator's always-public identity as
  snapshots, must not retain a Path target identifier, and must remain readable
  after every notification that targeted the deleted Path is removed.
- A broader visibility change affecting private-profile participants or
  administrators must use
  the behavior defined in
  [Profile and Path Visibility](../social/visibility.spec.md#expanding-path-visibility).

### Tracking, achievements, and social interactions

- Stopping a timer must not create a direct timer-stop notification; an
  eligible completed session must appear through its feed event instead.
- Achieving an interval goal or overall target must notify the participant who
  achieved it.
- Other users must discover another participant's achievement through the feed;
  the achievement itself must not send them a separate push notification.
- A new comment or feed-event reaction must notify the event owner, and a
  comment heart must notify the comment author, subject to the existing grace
  period and channel rules.
- A user must not receive a notification for their own follow, Path, tracking,
  achievement, comment, reaction, or heart action unless a requirement above
  explicitly identifies a distinct outcome they need to know.

## Notification grouping

- Every in-application notification must remain an individually readable
  record except for the explicitly defined reminder bundles.
- Operating-system notifications must use platform-appropriate grouping by
  Path, feed event, or notification channel when multiple related notices would
  otherwise create clutter.
- Operating-system grouping must not merge, delete, or change the read state of
  the underlying in-application records.
- The initial product must not add another semantic notification-bundling rule.
