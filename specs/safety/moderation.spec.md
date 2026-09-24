# Blocking and Reporting

Status: Approved for implementation

## Purpose

Define the basic user controls required to stop unwanted interaction and report
users or content for review.

## Blocking

- A user must be able to block another user.
- Blocking must remove any established follow relationship between the two
  users in either direction.
- Blocking must cancel any pending follow request between the two users in
  either direction.
- A blocked user must not be able to follow or send a follow request to the user
  who blocked them.
- The two users must not be able to find or open each other's profiles while the
  block remains in effect.
- Except for the minimum shared-path collaboration visibility defined below,
  neither user may encounter the other user's name, profile identity, or
  activity anywhere in the product while the block remains in effect.
- This identity-hiding rule must apply across profile search, feeds, comments,
  event reactions, comment-heart counts and rosters, active-tracking displays,
  statistics, and in-application notifications.
- Counts and summaries shown to either user must exclude the other user's hidden
  contributions when including them would reveal that the blocked relationship
  has activity on that surface.
- Blocking between two users must not hide either user's identity or activity
  from unrelated authorized users.
- Blocking must hide, rather than delete, existing comments, event reactions,
  comment hearts, and in-application notifications exchanged between the two
  users.
- If the blocker later unblocks the other user, those hidden interactions and
  notifications must become visible again wherever the current user is
  authorized to access their underlying content.
- Unblocking must not restore an interaction or notification whose underlying
  content was deleted while the block was in effect.
- Unblocking must not recreate follow relationships or pending follow requests
  that blocking ended.
- Settings must provide a blocked-accounts list from which the user can unblock
  an account.
- When the two users still share a path, that participant's shared-path menu
  must also offer an unblock action.
- Unblocking from either entry point must have the same effect.
- Outside a path they still share, neither user may see the other user's
  activity while the block remains in effect.
- A blocked user must not be able to send nudges or direct encouragement to the
  user who blocked them.
- Blocking must prevent new direct social interaction between the two users.
- Blocking must not notify the blocked user that the block occurred.

## Native blocked-account settings presentation

- Blocked Accounts must open from the compact Settings hierarchy as a titled
  native-stack destination with platform back and edge-swipe behavior.
- The list must use flat intrinsic-height identity rows, preserve readable full
  identity semantics at maximum Dynamic Type, and expose a standard native
  unblock confirmation and explicit admitted progress.
- Loading, stale-useful, empty, unavailable, refresh, pagination, unblock
  failure, and retry must remain distinct and localized. A cold or direct route
  must mount visible loading or recovery and must not wait forever for Home's
  in-memory blocking presentation context.
- List, pagination, reviewed target, and admitted unblock must remain owned by
  the activated account/profile session lineage and exact intent. Same-account
  credential rotation may preserve owned work; replacement or sign-out must
  clear prior-account identities and cancel late load, page, confirmation,
  unblock, or retry completions.
- The list and confirmation must remain operable with VoiceOver, Reduce Motion,
  Reduce Transparency, and Increase Contrast without relying on custom glass,
  animation, truncation, or color alone.

## Blocking within a shared path

- Blocking must not automatically remove either user from a path they share.
- Blocking must not alter either user's path role or delete either user's
  recorded activity.
- Before completing a block, the blocker must be warned when they will continue
  to share one or more paths with the blocked user.
- Completing a block must require a short-lived, server-issued acknowledgement
  of the immediately preceding block review; a client must not be able to skip
  the review even when the users share no paths.
- The acknowledgement must be bound to the blocker, blocked account, and exact
  set of paths they currently share. If that set changes before confirmation,
  the block must not complete and the client must refresh the review and warning.
- The warning must explain that the minimum identity, activity, and progress
  needed for those shared paths will remain visible to both users.
- Within a shared path, each user must remain able to see the other's participant
  identity and path-specific activity and progress needed for comparison.
- Shared-path visibility must not allow either user to open the other's profile
  or see activity from a path they do not share.
- Blocking must suppress feed events, comments, reactions, nudges, direct
  encouragement, comment hearts, and timer-start notifications between the two
  users, including events originating from a shared path.
- The blocker must be offered a separate action to leave a shared path.
- Leaving must follow the ordinary leaving behavior, including retaining the
  departing user's activity by default and separately offering deletion.

## Path administration across a block

- Blocking must not remove or weaken a creator's or administrator's existing
  authority to manage a Path they still share with the blocked participant.
- From the shared Path's access-management experience, an authorized creator or
  administrator must remain able to perform ordinary management actions on the
  blocked participant, including changing an ordinary participant/supporter
  role or removing access.
- Every action must continue to obey the ordinary role hierarchy, authorization,
  warning, confirmation, data-deletion, and notification rules; blocking must
  not grant additional management authority.
- The management interface must expose only the minimum blocked-user identity
  needed to select and understand the shared-Path action and must not restore
  access to the blocked user's profile or unrelated activity.
- Performing a Path-management action must not unblock either user or restore
  suppressed social interactions.
- If the action removes the final shared-Path relationship, the minimum
  shared-Path visibility exception must end with that relationship.

## Reporting

- A user must be able to report another user's profile.
- A user must be able to report user-generated content they can access, including
  comments and feed events.
- Every profile, path, feed event, comment, and nudge accessible to a user must
  expose a Report action through its overflow menu.
- Reporting must allow the reporter to identify a basic reason for the report.
- A report must require the reporter to select one reason from a predefined
  reason catalog.
- The initial reason catalog must contain:
  - `Spam or scam`;
  - `Harassment or bullying`;
  - `Hate or abusive content`;
  - `Sexual or inappropriate content`;
  - `Impersonation`;
  - `Privacy or personal information`;
  - `Dangerous or self-harm content`; and
  - `Something else`.
- A report must also allow, but must not require, a short free-text explanation
  that provides additional context.
- Leading and trailing whitespace must be removed from the optional explanation,
  and its remaining value must contain at most 1,000 characters.
- A whitespace-only explanation must be treated as absent.
- Submitting a report must not disclose the reporter's identity to the reported
  user through the product.
- The application must acknowledge that a report was submitted.
- The submission acknowledgment must be the only report-status feedback shown
  to the reporter in the initial product.
- The reporter must not receive notifications or in-application updates when
  the report is reviewed, resolved, dismissed, or results in enforcement.
- Submitting a report must not automatically hide the reported profile or
  content from the reporter and must not offer a separate report-specific hide
  action.
- After a report is accepted, the application must ask whether the reporter
  would also like to block the user responsible for the reported profile or
  content.
- Declining to block the user must not cancel or withdraw the submitted report.

## Internal report workflow

- Every accepted report must create a restricted moderation case with an
  internal state of `Open`, `Reviewing`, `Actioned`, or `Dismissed`.
- The case must retain the submitted reason, optional explanation, reported
  target, evidence representing the target at report time, reporter, creation
  time, current reviewer when assigned, decisions, and decision reasons.
- Moderation access must be restricted to authorized operational personnel and
  must not be granted through ordinary Path or application-user roles.
- A reviewer must be able to dismiss a report, remove content, warn a user,
  temporarily suspend an account, or permanently ban an account when
  proportionate to the reviewed behavior.
- Every moderation decision must retain who made it, when it was made, the
  policy basis, and the affected content or account.
- The initial product may operate this workflow through restricted operational
  tooling or a documented manual process; a dedicated in-application moderation
  dashboard is deferred.
- Deferring the dedicated interface must not defer timely review, enforcement,
  audit records, or the user-facing appeal behavior.

## Enforcement notice and appeal

- A user whose content or account is restricted through moderation must receive
  a concise notice identifying the affected content or account action, the
  applicable policy reason, the duration when temporary, and how to appeal.
- The notice must not identify the reporter.
- A user must be able to submit one appeal within 30 days of the enforcement
  notice.
- An appeal must allow a short explanation and must preserve the original
  decision and evidence.
- A different authorized reviewer must decide the appeal when practicable.
- The appeal decision must be communicated to the affected user with a concise
  reason and must be final within the initial product workflow.
- Reporter-facing behavior must remain limited to the submission acknowledgment
  already defined; moderation outcomes and appeals must not be disclosed to the
  reporter.

## Automated public-text checks

- A new or edited public comment or profile description must pass an automated
  text-safety check before it becomes visible.
- Text classified as clearly disallowed under the published Community
  Guidelines must be rejected before publication rather than silently or
  partially published.
- The rejection must use neutral language, identify that the text could not be
  posted, and allow the user to edit and retry it.
- Automated checks must not replace reporting, blocking, human review, or
  appeals.
- Content that passes an automated check must remain reportable.
