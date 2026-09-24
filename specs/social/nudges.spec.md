# Nudges and Direct Encouragement

Status: Approved for implementation

## Purpose

Define a lightweight way for one user to directly encourage another user's
practice and accountability.

## Product scope

- Direct nudges or encouragements must be part of the initial product.
- A nudge must be a deliberate action from one user to another user.
- Nudges must be designed to encourage practice rather than shame or punish the
  recipient.
- Nudges must remain distinct from reactions and comments on feed events.
- User-sent nudges must remain distinct from automated goal-aware practice
  reminders.

## Path-specific nudges

- Every nudge must target one recipient in the context of one specific path.
- The product must not support a general nudge without a path.
- The sender must be authorized to see the targeted path and the recipient's
  relevant path progress.
- A nudge must identify the path clearly to the recipient.

## Current-interval eligibility

- When the path has an interval goal, a user must not be able to nudge a
  participant who has already completed that goal for their current interval.
- Completion must be evaluated using the recipient's progress and configured
  time zone, not the sender's interval or time zone.
- The application must hide or disable the nudge action when the recipient's
  current interval is already complete.
- Nudge eligibility must be checked again when the action is submitted so a
  nudge is not sent after concurrent activity completes the recipient's goal.
- A path without an interval goal may still be nudged, subject to the other
  authorization and safety rules.

## Per-participant nudge audience

- Each participant must control who may nudge them separately for each path in
  which they participate.
- A path creator or administrator must not be able to broaden another
  participant's personal nudge audience.
- The setting must offer **Nobody**, **Path members**, **Followers**, and
  **Everyone** audience levels.
- **Nobody** must prevent every other user from sending that participant a
  nudge for the Path.
- **Path members** must allow nudges from other participants in the same path,
  including the creator and administrators, and from supporters who were
  explicitly added to that path; it must be the default.
- **Followers** must additionally allow nudges from the recipient's followers
  who are authorized to see the path.
- **Everyone** must additionally allow nudges from any user authorized to see the
  path.
- Merely being able to see a Path because of its follower or public visibility
  must not qualify a user as a Path member; only an explicitly assigned
  supporter role grants supporter eligibility at the default level.
- Path visibility, membership access, and blocking rules must continue to limit
  every audience level; selecting **Everyone** must not reveal a private path to
  an otherwise unauthorized user.
- A participant's audience choice for one path must not change who may nudge them
  on another path.
- Changing the setting must take effect for subsequent nudge attempts.
- When **Nobody** is selected, the nudge action must be hidden or disabled for
  other users and a rejected attempt must not consume a rate limit.
- Selecting **Nobody** must not disable automated practice reminders or comments
  and reactions governed by their separate settings.

## Nudge content

- The initial product must allow the sender to choose from a curated set of
  preset encouragements.
- The initial preset catalog must contain:
  - `you_have_got_this`: “You’ve got this!”
  - `lets_go`: “Let’s go!”
  - `little_progress_counts`: “A little progress counts.”
  - `keep_it_going`: “Keep it going!”
  - `time_to_work`: “Time to put in some work!”
- Every available preset must be positive and supportive.
- The initial product must not allow a sender to enter or attach a custom
  free-form message.
- A sent nudge must retain which preset was selected even if the preset's
  presentation or localization changes later.

## Native nudge presentation and recovery

- The eligible participant action must open a compact native single-view task
  that identifies the recipient and Path, presents the five approved presets as
  one standard single-choice group, places Cancel in the leading native bar
  position, and places Send or its retry equivalent in the trailing position.
- The clean task may use ordinary swipe dismissal. A selected preset and visible
  failure must remain available for retry, and an admitted send must not be
  abandoned by Cancel, back, or swipe dismissal before authoritative success or
  failure reconciles.
- A participant's Path-specific nudge audience must open as a titled native-
  stack destination with platform back and edge-swipe behavior. Nobody, Path
  members, Followers, and Everyone must use one standard selected-state group
  with localized loading, retained authoritative value, saving, failure, and
  retry presentation.
- A cold or direct audience route must mount visible localized loading or
  recovery and must never render blank because Path or Home in-memory
  presentation context is absent.
- Composer, eligibility, audience, reviewed capability, send, save, and retry
  work must remain owned by the activated account/profile session lineage and
  exact Path, recipient, preset or audience intent. Same-account credential
  rotation may preserve owned work; replacement or sign-out must clear prior-
  account state and cancel late completion.
- Native bars and task chrome must inherit Liquid Glass without custom glass in
  the content layer. The hierarchy and controls must reflow at maximum Dynamic
  Type and remain operable with VoiceOver, Reduce Motion, Reduce Transparency,
  and Increase Contrast without depending on motion, translucency, or color
  alone.

## Rate limiting

- For a path with an interval goal, one sender may successfully nudge one
  recipient at most once during that recipient's current path interval.
- The limit must be scoped to the combination of sender, recipient, path, and
  recipient interval occurrence.
- A nudge from another sender must not consume or block the first sender's limit.
- Nudging the same recipient on another path must use a separate limit.
- The next recipient interval must make the sender eligible to nudge again,
  subject to goal completion and the other nudge rules.
- The recipient's configured time zone and the path goal's stored alignment must
  determine which interval occurrence applies.
- An attempt rejected before a nudge is sent must not consume the sender's limit.
- Concurrent attempts must not result in more than one successful nudge for the
  same sender-recipient-path interval grouping.
- For a Path without an interval goal, one sender may successfully nudge one
  recipient at most once in a rolling 24-hour period.
- That rolling limit must be scoped to the same sender, recipient, and Path; a
  different sender or Path must have an independent limit.
- The 24-hour window must begin at the instant the successful nudge is sent and
  must not depend on either user's calendar day or time zone.
- Concurrent no-goal attempts must not produce more than one successful nudge
  within the same rolling window.

## Forward-compatible content model

- Nudge content must be modeled as an explicit content kind rather than as an
  untyped text field.
- The data model must store a stable preset identifier for an initial-product
  nudge and must not rely solely on its rendered display text.
- The content model must be capable of adding a distinct custom-message kind in
  a future product version without changing the meaning of existing preset
  nudges.
- Forward-compatible storage must not expose or accept custom messages before
  that behavior is separately specified and released.

## Delivery

- A successfully sent nudge must create both an in-application notification and
  an operating-system push notification for the recipient.
- The notification must identify the sender, the Path, and the selected positive
  preset without exposing information the recipient cannot access.
- Nudges must belong to their own user-controlled notification channel.
- A disabled nudge notification channel must suppress both delivery methods but
  must not change who is allowed to send a nudge or the ordinary nudge rate
  limit.

## History

- A received nudge must appear only through the ordinary in-application
  notification history when its notification channel permits delivery.
- The initial product must not provide a separate received-nudges screen or a
  sent-nudges history for senders.
- The absence of a user-facing history must not prevent retention of the minimum
  records needed to enforce rate limits, blocking, deletion, abuse prevention,
  or moderation rules.
