# Path Membership

Status: Approved for implementation

## Purpose

Define how creators and administrators share paths with participants and
supporters and revoke that access.

## Invitations

- The creator must be allowed to invite another user as a participant or
  supporter.
- An administrator must be allowed to invite another user as a participant or
  supporter.
- An ordinary participant must not be allowed to invite another user.
- A supporter must not be allowed to invite another user.
- An invitation must identify whether it offers participant or supporter access.
- Sending an invitation must not immediately give the recipient path access or a
  path role.
- The recipient must explicitly accept the invitation before becoming a
  participant or supporter.
- A private-profile recipient accepting participant access to a more broadly
  visible Path must first receive the access warning defined in
  [Profile and Path Visibility](../social/visibility.spec.md#private-profile-participants-joining-broader-paths).
- Accepting must grant the role offered by the invitation.
- The recipient must be able to reject the invitation.
- Rejecting must not grant path access or a path role.
- A participant invitation must not add the path to the recipient's Home until
  the recipient accepts it.
- A participant or supporter invitation must not expire automatically.
- A pending invitation must remain actionable until it is accepted, rejected,
  explicitly canceled, or invalidated because the path is deleted.
- A recipient's pending invitations must appear on a dedicated secondary screen
  that remains directly reachable without relying on notification retention or
  unread state.
- A recipient's pending-invitation list must identify the Path by name and the
  inviter by always-public user ID, canonical username, and display name. It
  must not expose provider email, profile visibility, or other profile data,
  and malformed or mismatched joined context must fail closed.
- The creator or any current administrator must be able to cancel any pending
  participant or supporter invitation for the Path, regardless of who sent it.
- The creator or any current administrator must be able to list every pending
  ordinary invitation for the active Path from its Share experience.
- The manager's pending-invitation list must identify each invitation by its
  opaque invitation ID, offered role, creation time, inviter's always-public
  identity, and recipient's always-public identity. It must not expose either
  user's provider email, profile visibility, or other profile data.
- Manager pending-invitation traversal must use actor-and-Path-bound,
  snapshot-stable keyset pagination ordered newest first. A malformed,
  mismatched, terminal, or inaccessible row or cursor must fail closed rather
  than leaking partial context.
- Canceling from the Share experience must require confirmation that names the
  recipient and offered role. Dismissing confirmation must leave the invitation
  pending and otherwise change nothing.
- Ordinary participants and supporters must not be allowed to cancel pending
  invitations.
- An ordinary invitation must target one active application user selected by
  username; provider email must not be used as the invitation identity.
- A user must not be allowed to invite themselves or a user who is already a
  current member of the Path.
- A Path must have at most one pending ordinary invitation for a given
  recipient, regardless of the role offered.
- Attempting to create another invitation while one is pending must leave the
  existing invitation and offered role unchanged.
- Only the intended recipient must be allowed to accept or reject an
  invitation.
- A request made by another user, or against a nonexistent, consumed, rejected,
  canceled, or otherwise inaccessible invitation identifier, must use the same
  opaque unavailable result and must not change membership or invitation state.
- Canceling an invitation must prevent the recipient from accepting it later
  and must not grant or remove current Path access.
- Canceling an invitation must atomically mark it canceled, remove its
  actionable in-application notification, suppress any pending push delivery
  for that invitation, and record exactly one successful audit event without
  creating a cancellation notification.
- Retrying the same cancellation with the same actor, Path, invitation,
  idempotency key, and request must return the original successful result
  without duplicating audit events or any other mutation.
- Reusing a cancellation idempotency key for a different Path or invitation
  must fail with an idempotency conflict and leave all state unchanged.
- A new cancellation request after the invitation became accepted, rejected,
  canceled, or otherwise unavailable must use the ordinary opaque unavailable
  result and leave all state unchanged.
- Concurrent cancellation, acceptance, and rejection attempts must produce
  exactly one successful terminal mutation. Every losing request with a
  different idempotency key must use the ordinary opaque unavailable result.
- An archived Path must remain view-only: cancellation must fail closed and
  leave the invitation pending until the Path is active again or deleted.
- The expiration behavior for ownership-transfer requests must not apply to
  ordinary participant or supporter invitations.
- Accepting an invitation must atomically consume the pending invitation, grant
  exactly its offered role, enqueue the corresponding durable authorization
  change, record the successful audit event, and create the inviter's
  informational notification.
- Acceptance must not report success until the queued authorization
  relationship has been applied. A dependency failure must remain safely
  retryable from the durable acceptance state without granting incomplete
  application access.
- Retrying the same acceptance with the same idempotency key and request must
  return the original successful result without duplicating membership,
  authorization relationships, audit success, or the inviter's informational
  notification.
- A new acceptance request made after the invitation was consumed must use the
  ordinary opaque unavailable result.
- Concurrent acceptance attempts must produce at most one successful mutation;
  a losing request with a different idempotency key must use the ordinary opaque
  unavailable result.
- Rejecting an invitation must atomically consume the pending invitation and
  record one successful audit event without creating membership, authorization
  relationships, or notifications.
- Retrying the same rejection with the same idempotency key and request must
  return the original successful result without duplicating audit events or any
  other mutation.
- A new rejection request made after the invitation was consumed must use the
  ordinary opaque unavailable result.
- Concurrent acceptance and rejection attempts must produce at most one
  successful terminal mutation; a losing request with a different idempotency
  key must use the ordinary opaque unavailable result.
- Until the private-profile visibility-warning workflow is available, accepting
  participant access to a broader Path must fail closed with a stable
  warning-required result and leave the invitation pending. Supporter
  invitations and participant invitations that do not require the warning must
  remain actionable.
- After participant acceptance, the Path must appear on the recipient's active
  Home and the recipient must be able to track their own time.
- After supporter acceptance, the recipient must be able to open the Path under
  supporter visibility but the Path must not offer tracking controls.

## Invitation workflow

- A path must exist before a participant or supporter invitation can be
  created.
- The path creation workflow must not invite specific users.
- After creation, the creator and administrators must have access to a Share
  action for the path.
- The Share action must allow an authorized user to select another user and
  offer participant or supporter access.
- Selecting a user must show enough always-public profile identity to verify the
  intended recipient before sending; provider email must not be shown.
- Sending after that review must bind the request to both the reviewed user ID
  and canonical username. The server must transactionally verify that the same
  active user still owns that exact username before creating any invitation,
  notification, audit event, or idempotency result; a changed or reassigned
  username must use the ordinary opaque unavailable result.
- Ordinary participants and supporters must not be shown invitation controls
  they are not authorized to use.

## Removing access

- The creator must be allowed to remove an ordinary participant or supporter
  from the path.
- An administrator must be allowed to remove an ordinary participant or
  supporter from the path.
- An ordinary participant or supporter must not be allowed to remove another
  user.
- An administrator must not be able to use membership removal to remove or
  modify the creator.
- An administrator must not be able to use membership removal to revoke another
  administrator's role.
- Removing or changing administrator status must follow
  [Path Roles](roles.spec.md#administrator-role-changes).
- Removing an ordinary participant must permanently delete all recorded
  activity attributed to that participant within the path.
- Before removal, the manager must be shown the participant's identity, current
  session count, and total tracked time on the Path.
- The warning must explain that the participant's recorded activity, progress,
  statistics, achievements, and activity-derived feed events will be
  permanently deleted.
- The warning must explain that reinviting the participant will not restore the
  deleted data.
- If the removed participant has one or more timers running on the path, removal
  must stop those timers as part of the removal taking effect.
- Elapsed time from a timer stopped by participant removal must be deleted with
  the participant's other path activity and must not be saved as a new entry.
- When a timer is running, the warning must state that its elapsed time will be
  discarded.
- The warning must state that pending offline activity for the Path will be
  rejected when the removed participant's device synchronizes.
- Removal must require an explicit `Remove participant and delete data` action;
  the initial product must not require typed-name confirmation.
- Canceling or dismissing the warning must leave membership, timers, recorded
  activity, and derived data unchanged.
- Participant removal must not stop that user's timers on another path.
- Progress, statistics, achievements, and feed events derived from the removed
  participant's deleted path activity must no longer include that activity.
- Reinviting a removed participant must not restore activity deleted by their
  removal.
- A removed participant or supporter must be eligible to receive a new
  invitation under the ordinary invitation rules.
- Reinvitation must create a new pending invitation and must not silently
  restore the former membership.
- Removing a supporter must remove their role and access but cannot delete
  recorded activity because a supporter cannot track time on the path.
- A denied membership-management action must leave membership and roles
  unchanged.
- A device that was offline during removal must apply the permanently rejected
  synchronization behavior when it later learns that membership ended.

## Changing participant and supporter access

- The creator or an administrator must be able to change an ordinary member
  directly from participant to supporter or from supporter to participant.
- This role change must not require removing the member, sending a new
  invitation, or waiting for the member to accept again.
- An ordinary participant or supporter must not be allowed to change their own
  access role or another member's role.
- This workflow must not change the creator's or an administrator's underlying
  participant role; administrator changes must follow the separate
  administrator-role rules.
- Completing the change must preserve the user's continuous Path membership and
  take effect immediately.
- Changing a participant to a supporter must permanently delete that user's
  recorded activity and progress for the Path.
- Before that destructive role change, the managing creator or administrator
  must be shown a prominent warning that identifies the affected user and
  explains that their activity and progress will be lost.
- The warning must require explicit confirmation; canceling or dismissing it
  must leave the user's participant role and data unchanged.
- If the participant has a running timer on the Path, completing the change must
  stop it and discard its elapsed time rather than save a final activity entry.
- Pending offline activity for that Path must be rejected and removed when the
  demoted user's device learns that they became a supporter.
- Progress, statistics, achievements, and feed events derived from the deleted
  activity must be removed or recalculated accordingly.
- Changing that supporter back to a participant later must not restore the
  deleted activity or progress.
- After any completed Path role change, the affected user must receive an
  eligible in-application and push notification identifying the Path, their new
  role, and the practical effect of the change.
- A participant-to-supporter notification must specifically state that the
  user's Path activity and progress were removed.
- A completed role-change notification must be informational rather than
  presented as an urgent warning because the role change has already occurred
  and the recipient is not being asked to approve it.

## Leaving a path

- The current creator must not be allowed to leave a Path while retaining
  ownership.
- Initiating an ownership-transfer request must not make the creator eligible to
  leave; the selected participant must first accept and complete the transfer.
- After an accepted transfer changes the former creator to an administrator,
  that former creator must be allowed to leave under the ordinary non-creator
  rules, and leaving must remove their administrator role with their membership.
- If the creator does not complete an ownership transfer, permanently deleting
  the entire Path must be the available alternative to leaving it.
- Archiving a Path must not remove its creator or make the creator eligible to
  leave without transferring ownership.
- A non-creator participant or supporter must be able to leave a path
  voluntarily.
- Voluntary leave must be unavailable while the Path is archived.
- Leaving must remove the user's current path membership and access.
- By default, leaving must retain the departing user's recorded activity on the
  path.
- The leaving experience must present retention as the ordinary default and
  explain that the retained activity will be hidden while the user is outside
  the Path.
- If the departing participant has a timer running on the Path and retains their
  activity, leaving must stop the timer and save its elapsed time as recorded
  activity before removing membership; the resulting activity must then follow
  the same hidden-retention rules as their earlier activity.
- While the former participant is not a member of the Path, retained activity
  must not be visible to the remaining creator, administrators, participants,
  or supporters.
- Hidden retained activity must not appear in Path history, participant
  comparisons, Path statistics, achievements, or feed events while the former
  participant remains outside the Path.
- When retained activity becomes hidden after voluntary leave, target-opening
  notifications derived from its activity, feed events, comments, reactions,
  or comment hearts must be removed under the notification loss-of-access
  rules. Informational membership-change notices and unrelated notifications
  must remain.
- The leaving workflow must also offer the user an explicit option to
  permanently delete their own recorded activity from the path.
- The delete-data option must clearly distinguish deleting activity from merely
  leaving and must require confirmation.
- The destructive confirmation must explain that the user's recorded activity,
  progress, statistics, achievements, and activity-derived feed events for the
  Path will be permanently deleted and will not return if they rejoin.
- If the departing participant has a timer running on the Path and chooses to
  delete their activity, leaving must stop that timer and discard its elapsed
  time without saving a final activity entry.
- A supporter with no recorded activity on the Path must receive an ordinary
  leave experience that does not claim activity will be retained or deleted
  and must not be asked to make an inapplicable activity choice.
- Canceling or dismissing the leaving experience or its destructive
  confirmation must leave membership, roles, timers, recorded activity, and
  derived data unchanged.
- If the former participant later rejoins the path, activity retained when they
  left must again be associated with their membership and history and become
  visible under the Path's ordinary access rules.
- Rejoining must not resurrect a target-opening notification or pending push
  work removed when the retained activity became hidden. A new eligible
  interaction after rejoining may create a new notification normally.
- Rejoining must not restore activity that the user chose to delete when
  leaving.
- A successful leave must atomically remove membership and any administrator
  role, apply the chosen timer and activity outcome, remove or hide derived data
  as applicable, enqueue durable authorization removal, record the successful
  audit event, create informational notifications for the creator and remaining
  administrators, and persist the idempotency result.
- The creator and remaining administrators must each receive at most one
  eligible in-application and push notification identifying the departing user
  and Path. The notification must not disclose whether the departing user
  retained or deleted their activity.
- Leaving must not report success until the queued authorization removal has
  been applied. A dependency failure must fail closed and remain safely
  retryable from durable committed state without restoring incomplete access or
  duplicating side effects.
- Retrying the same leave with the same actor, Path, idempotency key, and request
  must return the original result without duplicating timer effects, activity
  deletion, derived-data changes, authorization changes, audit events, or
  notifications.
- Reusing a leave idempotency key for a different request must fail with an
  idempotency conflict and leave all state unchanged.
- Concurrent leave attempts must produce at most one successful mutation; a
  losing request with a different idempotency key must use the ordinary opaque
  unavailable result.
- A leave attempt by the current creator, a nonmember, or any user against an
  archived, deleted, or otherwise inaccessible Path must fail closed and leave
  membership, roles, timers, activity, derived data, authorization, and
  notifications unchanged.
