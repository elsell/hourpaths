# Path Permission Matrix

Status: Derived from approved specifications

## Purpose

Provide a consolidated verification view of Path authorization already defined
by the authoritative role, membership, visibility, lifecycle, activity, and
social specifications. If this matrix conflicts with a linked authoritative
specification, the authoritative specification must be corrected first and this
matrix must then be regenerated.

## Role matrix for an active Path

`Own` means the action applies only to the actor's own activity, preferences, or
interactions. `Conditional` means another current rule, such as visibility,
blocking, event-owner settings, or nudge audience, must also permit it.

| Action | Creator | Administrator | Participant | Supporter | General authorized audience |
| --- | --- | --- | --- | --- | --- |
| View Path identity and goals | Yes | Yes | Yes | Yes | Conditional on visibility |
| View current participant progress and sessions | Yes | Yes | Yes | Yes | Conditional on visibility |
| Track time | Own | Own | Own | No | No |
| Edit or delete recorded activity | Own | Own | Own | No | No |
| View another user's private activity note | No | No | No | No | No |
| Rename active Path | Yes | Yes | No | No | No |
| Create, change, or remove goals | Yes | Yes | No | No | No |
| Change Path visibility | Yes | No | No | No | No |
| Invite participant or supporter | Yes | Yes | No | No | No |
| Cancel ordinary pending invitation | Yes | Yes | No | No | No |
| Remove ordinary participant or supporter | Yes | Yes | No | No | No |
| Change participant/supporter role | Yes | Yes | No | No | No |
| Grant or revoke administrator | Yes | Self-step-down only | No | No | No |
| Transfer ownership | Yes | No | No | No | No |
| Archive or unarchive | Yes | No | No | No | No |
| Permanently delete Path | Yes | No | No | No | No |
| Leave Path | Only after transfer | Yes | Yes | Yes | Not applicable |
| Configure personal Home/reminder/nudge settings | Own | Own | Own | Own where applicable | No |
| Comment or react on eligible feed event | Conditional | Conditional | Conditional | Conditional | Conditional |
| Nudge participant | Conditional | Conditional | Conditional | Conditional | Conditional |
| Report accessible Path content or user | Yes | Yes | Yes | Yes | Yes when accessible |

## Hierarchy constraints

- The creator must remain exactly one current participant and cannot leave,
  become a supporter, or lose creator status except through an accepted
  ownership transfer.
- An administrator must remain a participant while holding administrator
  status.
- An administrator must not remove or change the creator or another
  administrator through ordinary membership management.
- Only the creator may grant or revoke another user's administrator status.
- An administrator may voluntarily step down to ordinary participant.
- Path-management authority never grants authority to edit another
  participant's activity, private note, comment, or personal preferences.

## Lifecycle overlays

- An archived Path is view-only for every role except that the creator may
  unarchive or permanently delete it.
- A deleted Path grants no continuing Path permission and must use opaque
  unavailable behavior for stale references.
- A removed participant or supporter has no continuing access through the
  former role.
- A former participant's retained activity remains hidden while they are not a
  current member and becomes visible again only if they rejoin.
- Blocking suppresses ordinary social visibility and interaction but does not
  weaken creator or administrator authority needed to manage a Path the blocked
  users still share.

## Authoritative specifications

- [Path Roles](roles.spec.md)
- [Path Membership](membership.spec.md)
- [Path Lifecycle](lifecycle.spec.md)
- [Profile and Path Visibility](../social/visibility.spec.md)
- [Recorded Activity](../tracking/recorded-activity.spec.md)
- [Social Feed](../social/feed.spec.md)
- [Blocking and Reporting](../safety/moderation.spec.md)
