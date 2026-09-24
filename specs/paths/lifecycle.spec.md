# Path Lifecycle

Status: Approved for implementation

## Purpose

Define the high-level creation and lifecycle behavior of a path.

## Creation

- A path name must be the only user-entered value required to create a path.
- Leading and trailing whitespace must be removed from a Path name before it is
  validated and stored.
- A Path name must contain between 1 and 100 characters after trimming.
- A whitespace-only Path name must be rejected.
- A Path name may contain printable Unicode characters, including emoji and
  characters from any writing system.
- A Path name must not contain line breaks or control characters.
- Path names must not be required to be unique, including among Paths created
  by the same user.
- Renaming a Path must apply the same validation rules as creation.
- Interval goals and overall targets must be optional during creation.
- Sharing with another user must be optional during creation.
- When creation succeeds, the creating user must become the path's creator and
  first participant.
- The newly created path must be available for time tracking without requiring
  any additional setup.

## Creation workflow

- Path creation must use one primary screen rather than a required multi-step
  wizard.
- The screen must present the required path name.
- The same screen must present optional interval-goal and overall-target
  controls.
- The same screen must present the path visibility control.
- Sharing during creation must mean selecting the path's visibility level; it
  must not require selecting or inviting specific users.
- Optional controls must not prevent a user from quickly creating a name-only
  path.

## Paths without goals

Goal-optional tracking and reporting behavior is defined in
[Time Goals](../goals/time-goals.spec.md#optional-goals).

## Editing an active Path

- The creator and administrators must be able to rename an active Path and edit
  its non-access settings.
- Ordinary participants and supporters must not be able to rename the Path or
  edit those settings.
- This authority must not grant administrators creator-only actions such as
  archiving, ownership transfer, administrator appointment or removal, or
  permanent Path deletion.
- Visibility, invitations, roles, and membership remain access-management
  concerns governed through the separate Share experience and their
  authorization rules.

## Archiving

- A path must be archivable without deleting its history or recorded activity.
- Archiving must be the lifecycle action for retaining a path and its activity
  while removing it from ordinary active use.
- Only the path creator must be allowed to archive the path.
- Administrators, ordinary participants, and supporters must not be allowed to
  archive the path.
- An archived path must not appear on any user's profile to followers, the
  public, or other users who lack a path role.
- Existing participants and explicitly assigned supporters must retain access to
  the archived Path and its history.
- Archiving must not remove participant or supporter roles or recorded activity.
- An archived Path must be view-only for its creator, administrators,
  participants, and supporters.
- The creator's actions to unarchive or permanently delete the Path are the only
  ordinary Path mutations available while it is archived.
- While archived, users must not change the Path name, goals, visibility,
  membership, or roles; create or alter activity; or add, edit, or remove
  comments, reactions, or nudges associated with the Path.
- If any participant has a timer running on the path when it is archived, that
  timer must stop at the time the archive takes effect.
- Elapsed time from each timer stopped by archiving must be saved as recorded
  activity before the path becomes read-only.
- Offline timers unknown to the server when archival takes effect must use the
  synchronization and split rules defined in
  [Offline Behavior](../sync/offline.spec.md#synchronizing-after-path-archival).
- Archiving one path must not stop timers running on another path.
- Participants must not be able to start a timer, add a manual time entry, or
  edit or delete recorded activity on an archived path.
- An archived path must not appear in the default active-path list on Home.
- Participants and supporters who retain access must be able to reach archived
  Paths through a separate archived-Path view or filter.
- The archived path must remain available to its participants unless the creator
  deletes it or another later lifecycle action removes their access.

## Unarchiving

- The path creator must be able to unarchive an archived path.
- Administrators, ordinary participants, and supporters must not be able to
  unarchive it.
- Unarchiving must restore the path to active, trackable use.
- The path must retain the same members, roles, goals, visibility, recorded
  activity, and history it had while archived.
- Unarchiving must return the path to participants' active Home lists according
  to each participant's personal ordering and pinning rules.
- Timers that were stopped by archiving must not restart automatically when the
  path is unarchived.
- Unarchiving must not create recorded activity or change historical time.

## Deletion

- Only the path creator may delete the path.
- Deleting a path must be permanent and irreversible through the product.
- Deletion must remove the path, its configuration, memberships, and all
  recorded activity belonging to every participant in that path.
- If any participant has a timer running on the path, deletion must stop that
  timer as part of the deletion taking effect.
- Elapsed time from timers stopped by path deletion must be discarded with the
  path and must not be saved as final recorded activity.
- Deleting one path must not stop timers running on another path.
- Derived progress, statistics, achievements, and feed events must no longer
  include data deleted with the path.
- Before deletion, the creator must be shown a prominent warning that every
  participant will lose their path data and recorded activity, not only the
  creator's own data.
- The warning must state that any running timers on the Path will stop and their
  elapsed time will be lost. This warning may be shown unconditionally so the
  client does not need to disclose or preflight another participant's timer
  state.
- The warning must distinguish permanent deletion from archiving, which retains
  participant data while removing the path from active use.
- The creator must explicitly confirm deletion after seeing the warning.
- The confirmation must be bound to the current canonical Path name shown in
  the warning. If that name changes before deletion commits, the request must
  fail with a conflict so the creator can review the current Path again.
- Canceling or dismissing the warning must leave the path and all participant
  data unchanged.
- The deletion request must carry an explicit confirmation and an idempotency
  key. An unconfirmed request must be rejected without mutation.
- Retrying the same confirmed deletion with the same creator, idempotency key,
  Path, and request must return the original successful result without
  duplicating notices, authorization changes, or audit events.
- Reusing a deletion idempotency key for a different request must fail with an
  idempotency conflict and must not mutate either Path.
- The server must revalidate creator authority against the current Path before
  deletion. A request by an administrator, participant, supporter, former
  member, or other user must fail closed and leave all data unchanged.
- Path data removal, timer removal, standalone deletion notices, the successful
  audit event, the idempotency result, and durable authorization cleanup must be
  committed atomically.
- Immutable security audit records governed by [Audit](../audit/audit.spec.md)
  must remain under their retention policy; they are not restorable Path data
  or a user-visible Path history.
- A standalone deletion notice must retain only the former Path name and the
  deleting creator's always-public identity needed to explain what changed. It
  must not retain a navigable Path target or expose deleted configuration,
  membership, activity, timer, goal, or visibility data.
- After deletion succeeds, every ordinary Path read or mutation using the old
  identifier must use the opaque unavailable result and must not recreate data.
