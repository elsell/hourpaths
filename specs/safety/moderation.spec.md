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

### Reporting acceptance (`SAFE-REPORT-01`)

1. Native and Studio overflow menus on an accessible profile, Path, feed event,
   comment, and nudge open the same reporting flow: one required catalog reason
   and an optional explanation. A submitted report receives an acknowledgment
   and offers the existing block flow without hiding or deleting the target.
2. An explanation containing 1,000 Unicode characters after trimming is accepted;
   1,001 is rejected with editable input retained. Whitespace alone is absent.
3. An authenticated reporter cannot submit a report for a target they cannot
   currently access. Invalid sessions, access loss, and unavailable authorization
   fail closed without revealing the target's existence or content.
4. Retrying one submission after a lost response creates one restricted case,
   one evidence snapshot, and one successful mutation audit. Reusing that
   submission identity for different content does not overwrite the case.
5. The case preserves the reason, normalized explanation, target and report-time
   evidence. Editing the original content afterward does not rewrite evidence.
   Ordinary users, including Path administrators and the reported user, cannot
   retrieve cases or reporter identity through product APIs.
6. Declining the post-report block offer leaves the accepted report intact.
   Accepting it uses the existing reviewed block behavior and shared-Path warning.
   Signing out or changing accounts clears pending reporting and block state.
7. Restricted operational review can locate accepted cases; reporter-facing UI
   remains acknowledgment-only. Evidence retention and account deletion follow
   [Account Deletion](../accounts/deletion.spec.md), including its minimum-evidence
   exception and expiry deadline.

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

### Enforcement and appeal acceptance (`SAFE-ENFORCEMENT-01`)

These scenarios exercise the approved enforcement lifecycle above; reporting
acknowledgments must remain unchanged.

1. An authorized operational reviewer can act on a reviewed case with a policy
   reason. The original report and evidence remain intact, and the decision
   records the reviewer, time, affected target, and temporary duration where
   applicable. Ordinary application and Path roles cannot perform enforcement.
2. The affected user can read a private notice identifying the action and policy
   reason, its temporary duration when applicable, and the appeal entry point.
   Neither the notice nor an appeal response reveals the reporter's identity or
   the restricted report evidence. Another user cannot read that notice.
3. Within the specified 30-day window, the affected user can submit one appeal
   with an explanation. Retrying a submission after a lost response must not
   produce a second appeal or replace the original explanation. An appeal after
   the window or by a different account is rejected without altering evidence.
4. An authorized reviewer can decide the appeal with a reason. The original
   enforcement decision and evidence remain preserved. The affected user sees
   the final outcome and reason; the reporter receives no outcome notification.
5. Content removal, warning, temporary suspension, and permanent ban must each
   have their specified effect; recording a decision alone is not enforcement.
   A restricted account must retain access to its own notice and appeal process
   without regaining ordinary access prohibited by that restriction.
6. Native and Studio Settings expose Account notices with the same notice and appeal lifecycle. Signing out or
   replacing the account clears pending notice and appeal presentation, and late
   completions cannot display the previous account's private information.

### Profile-field removal acceptance (`SAFE-PROFILE-REMOVAL-01`)

The owner approved removing offending profile fields while preserving the profile
and its history on October 13. This must not hide the entire profile or suspend
its account.

1. A restricted reviewer must select the reported display name, description,
   picture, or a combination. Unselected fields, immutable identity, username,
   relationships, Paths, timers and recorded activity must remain unchanged.
   A removed display name must use a neutral replacement supplied as data;
   removed descriptions and pictures must become absent.
2. Selected current content must still match the retained report evidence.
   Removal must atomically update the profile, private enforcement notice, case
   and attributed review event. Runtime and retention roles must not receive
   enforcement or restoration capability. The subject-visible policy reason
   must identify the profile fields removed; it must not disclose the reporter
   or restricted evidence.
3. Both clients must expose the existing private Account notices and 30-day
   appeal flow. Profile edits must remain available. Stale pre-removal edits
   must conflict instead of restoring removed content inadvertently.
4. Reversal must restore only retained fields still owned by that removal.
   Subsequent edits, including editing away and back, must not be overwritten;
   text edits must not prevent restoration of an unchanged removed picture or
   vice versa. Deleted accounts must never be recreated. Appeal decisions must
   remain final and require a different reviewer or recorded justification.
5. Removed picture bytes must cease to be publicly available and remain only
   in restricted evidence/restoration storage until case expiry. Removal must
   suppress pending push containing the subject's old display name; already
   delivered notifications cannot be recalled. Reversal must not resend push.

### Comment-removal acceptance (`SAFE-COMMENT-REMOVAL-01`)

1. An operational reviewer can remove the comment identified by an open or
   reviewing comment case. Removal, the private enforcement notice, case
   transition and decision event must commit together. Runtime credentials and
   ordinary users cannot invoke this operational action.
2. Removed comments must not appear in comment lists, feed comment counts,
   comment history, heart rosters, notifications or pending push deliveries.
   Direct identifiers and lost-response mutation retries must not reveal removed
   text or allow edits to make it visible again. Other comments remain available.
3. The author's private notice must identify the affected comment without
   disclosing the reporter or restricted evidence. The existing 30-day appeal,
   immutable decision and reviewer requirements apply.
4. Reversing a removal must lift that decision's visibility restriction, subject
   to ordinary access and any other active restriction. It must not recreate
   content deleted through ordinary Path, activity or account lifecycle actions,
   bypass a block, or resend a suppressed notification.
5. Concurrent comment mutation and removal must serialize so that no edit or
   heart committed after removal can republish or interact with removed content.
   A removal committed before push handoff must suppress that delivery.

### Path-name removal acceptance (`SAFE-PATH-NAME-REMOVAL-01`)

Owner approval, 2026-10-13: replace an offending Path name while preserving
members, running timers and recorded history. This action must not hide or delete
the shared Path.

1. An authorized operational reviewer must replace the name identified by an
   open or reviewing Path case with a neutral replacement name. The replacement
   is stored Path data supplied by the reviewer (normally the shared catalog's
   localized “Name removed”), not an API-generated presentation sentence.
   The current owner and name must still match the reviewed subject and reported
   name; otherwise the stale case must not replace newer content.
2. Removal must preserve Path identity, ownership, visibility, membership, goals,
   appearance, running timers, recorded time and social history. Subsequent
   authorized reads and push composition must use the replacement name. Already
   composed or queued OS notifications containing the old name must be suppressed
   when removal wins the final handoff; no old push is resent on appeal. Already
   delivered notifications and offline copies cannot be recalled; refresh uses
   the current name. An administrator may rename the Path through the existing
   authorized edit flow; stale rename retries must not restore the removed name.
3. Removal, its private owner notice, case transition and attributed review event
   must commit atomically. The notice must identify the affected Path without
   revealing the removed name, reporter or restricted evidence. Native and Studio
   clients must reuse the existing notice and appeal components.
4. The existing 30-day appeal and immutable final-decision rules apply. Reversal
   must restore the retained original name only if this removal still owns the
   name revision and the same account owns the Path. Later renames (including a
   rename away and back), ownership transfer and ordinary deletion must not be
   undone. Unrelated goal edits must not prevent restoration. A deleted Path
   must never be recreated. Reversal must not resend old notifications.
5. Removal and reversal must serialize with Path editing through the Path row
   lock. Runtime and retention credentials must not invoke reviewer functions
   or read retained removed names. Wrong-kind appeal functions and repeated
   final decisions must fail atomically. Retention must follow the existing
   case/notice and ordinary Path/account deletion lifecycle.

### Feed-event removal acceptance (`SAFE-FEED-REMOVAL-01`)

These scenarios apply the approved content-removal lifecycle to the social post
identified by a reported feed event. The post is a social projection of recorded
activity or a goal achievement; removing the post must not delete that source.

1. An authorized operational reviewer must be able to remove a feed event from
   an open or reviewing case. The decision must retain the case, subject,
   reviewer and policy reason. The subject's private notice must identify the
   affected event and its publication time without exposing reporter identity
   or restricted report evidence.
2. A removed event must not appear in Following, profile activity history,
   direct event reads, comment/reaction interactions or their retained mutation
   retries, notification history or unread counts. Its associated comments and
   reactions must not be independently accessible through the removed event.
   Missing and removed targets must use the same opaque unavailable behavior.
3. Removal must preserve recorded activity, goal achievements, Path membership,
   timers and unrelated social content. Editing the underlying recorded
   activity must not lift the social-post restriction.
4. Removal must serialize with event interactions and push handoff. If removal
   commits first, a competing mutation or retry must not republish or interact
   with the post, and the provider must not receive its queued notification.
   Pending delivery must be suppressed without retaining its encrypted token.
5. The existing notice and appeal process must work in native and Studio
   clients. A successful appeal lifts only this decision's visibility
   restriction, subject to current access. It must not recreate a deleted event
   or source activity, restore separately removed comments, or resend old push.
6. Removal, the case transition, private notice and reviewer event must commit
   atomically. Ordinary runtime and retention credentials must not perform
   removal or decide an appeal. Final appeal decisions remain immutable, and
   a decision function for another content kind must not finalize this appeal.

### Nudge-removal acceptance (`SAFE-NUDGE-REMOVAL-01`)

These scenarios apply the approved content-removal and appeal lifecycle to a
reported received nudge; they do not add a new enforcement action.

1. An operational reviewer must be able to remove the nudge identified by a
   reviewed nudge case. The subject notice must identify the sent nudge and its
   sent time without exposing the reporter or restricted case evidence. The
   original nudge, case evidence, decision and reviewer record must be retained
   according to the existing lifecycle and retention rules.
2. A removed nudge must not appear in recipient notifications, unread counts,
   report-target resolution, or lost-response send retries. Unrelated nudges and
   their notification state must remain unchanged. Removal must not reset the
   sender's existing rate limit.
3. Removal must suppress undelivered OS push for that nudge. Removal and push
   handoff must serialize: if removal commits first, the provider must not be
   called. An already handed-off push cannot be recalled.
4. The existing private notice and one-appeal workflow must apply. Reversal must
   lift only the moderation visibility restriction, subject to current audience,
   blocking, retention and ordinary deletion rules. It must not recreate deleted
   records or resend an old OS push.
5. The removal, case transition, subject notice and attributed review event must
   commit atomically. Ordinary application and retention credentials must not
   receive enforcement or appeal-decision capability. A repeated removal must
   not create a second decision or partially change the case.

## Account suspension and ban enforcement

Owner decision, 2026-10-13: when an account is suspended or banned, its running
timers must stop and save elapsed time. Suspension and ban are restrictions on
ordinary account use, not account deletion.

- An authorized operational reviewer must issue a suspension with an end instant
  or a permanent ban against a reviewed case. The decision, subject notice,
  case transition, reviewer record and server-known timer completion must commit
  atomically. Failure must leave the prior account and timer state intact.
- Every server-known running timer for the subject must stop at the enforcement
  instant. Recorded activity must retain its start instant and occurrence time
  zone. Existing subsecond-stop rules apply; another participant's timers and
  recorded activity must not change.
- Enforcement must serialize with timer creation/stopping and offline replay so
  that races cannot leave a new active timer after enforcement, duplicate saved
  time, or overwrite the enforcement stop with a later stop.
- Ordinary authenticated use must be denied while any unreversed restriction is
  active. Suspension covers its issue instant up to, but not including, its end;
  a ban has no expiry. A dependency failure must not grant ordinary access.
- The restricted user must retain authenticated access to their own enforcement
  notices and existing appeal process, minimal account/session lifecycle,
  sign-out, and the existing account export/deletion controls. Other accounts
  must not gain access to these private records. A restriction must not be
  represented as an expired or invalid credential.
- Mobile and Studio must show the restriction reason, temporary end when
  applicable, and the existing notice/appeal controls instead of ordinary
  application actions. Account replacement must clear the previous account's
  restriction state. Native timer surfaces must be removed when the client
  learns of the restriction.
- Clients must pause ordinary synchronization when restricted and must not
  silently discard retained pending work. Offline clients cannot learn a new
  restriction until they reconnect; their later requests must obey the current
  server restriction and canonical timer-stop result.
- A reversed decision or expired suspension must cease restricting access only
  when no other active restriction remains. It must not restart stopped timers,
  erase saved elapsed time, or erase the original decision/appeal evidence.
- Existing moderation review, appeal deadline, independent-reviewer preference,
  account deletion, and evidence-retention requirements remain authoritative.

### Acceptance (`SAFE-ACCOUNT-ENFORCEMENT-01`)

1. Restricting a subject with two active Paths saves both elapsed intervals at
   the same instant and stops both timers; a peer's timer stays running.
2. A timer start/stop or replay racing enforcement either commits before the
   restriction or fails safely; no running timer or duplicate elapsed time
   survives an enforcement-first outcome. Transaction failure rolls back the
   notice, restriction and timer changes together.
3. A restricted subject cannot use ordinary routes, including sync. They can
   read and appeal their own notice, export/delete their account and sign out.
   Invalid credentials, cross-account notice access and dependency failures
   remain denied without leaking private evidence.
4. At suspension expiry, ordinary use resumes unless another active restriction
   exists. Reversal removes only the reversed restriction. Neither case restarts
   timers or removes completed activity.
5. Studio and mobile present the same private decision and appeal behavior;
   refresh, restart, expiry, reversal and account switching converge safely.

## Public-text moderation boundary

Owner decision, 2026-10-12: text processing must stay entirely on HourPaths
infrastructure. During the current trusted-content phase, automated classification
is explicitly deferred; the application must use an injected moderation port with
an explicitly selected no-op adapter that accepts content without screening.
No external moderation service may receive submitted text.

- New and edited comments and profile descriptions must pass through this port
  before persistence. Authentication, authorization, normalization, rate limits,
  idempotency, atomic audit and existing visibility rules must remain intact.
- The no-op adapter must not classify, store, log or transmit submitted text.
  Acceptance must not be represented as a safety classification.
- Missing moderation dependencies or adapter failures must prevent a new write;
  they must not silently select the trusted-content adapter.
- Reporting, blocking, human review, enforcement and appeals remain required.
  Accepted content must remain reportable.
- This phase does not add a model, keyword filter, external provider or client
  moderation UI. Introducing actual classification requires a subsequent approved
  policy and implementation slice; it is not a blocker for this approved phase.

### Acceptance (`SAFE-TEXT-PORT-01`)

1. Valid authorized comment creation, editing and profile-description updates
   retain their normal behavior with the explicitly injected trusted-content
   adapter. All clients use the same server-side boundary.
2. The port receives normalized text and its surface type before the mutation.
   An unavailable adapter prevents the mutation and does not publish text or
   queue a notification. Existing comment replay returns the committed result.
3. Unauthenticated or unauthorized submissions cannot bypass existing checks or
   reach a content processor before authorization. No text leaves our infrastructure
   or is added to logs, audit metadata or a new persistence store by this adapter.

### Deferred classifier behavior

When actual classification is approved, clearly disallowed text under the
published Community Guidelines must be rejected before publication. Rejection
must be neutral and editable, permitting retry. Classification must not replace
reporting, blocking, human review or appeals. The classifier must run entirely on
HourPaths infrastructure.

## Administrator attention (October 17, 2026)

New reports and appeals must create private in-app notifications for active
platform administrators in the same transaction that accepts the submission.
Retries must not duplicate notices. Notice text must not include report evidence,
reporter identity or appeal text. Opening a notice must check current platform
administration and SpiceDB authorization, audit the read, and mark only that
administrator's notice read. A revoked or suspended administrator must lose
access. The private record may display the report reason/explanation or appeal
explanation and existing decision reason; this does not add enforcement actions
or change the existing operator review process. Retention of the underlying
record must remove its notices. See [private support](../support/private-support.spec.md)
for the shared platform-admin grant.
