# Social Sharing

Status: Approved for implementation

## Purpose

Define how users deliberately share their personal progress for accountability
and motivation.

## Product intent

Social features must help users encourage one another and remain accountable to
the practice they intended to do. A shared path must keep each participant's
recorded time, progress, and achievements attributed to that participant.

## Sharing control

- A user must be able to track a path without sharing it with anyone.
- Access-based sharing must be configured at the Path level through explicit
  membership and Path visibility.
- A recorded activity session, interval goal, overall target, progress value,
  achievement, or feed event must not have an independent audience, visibility
  setting, or access grant.
- Child content must inherit access from its Path together with any additional
  feed-visibility, blocking, or content-type rules that apply to the observing
  user.
- Granting access to one child item must not be supported as a way to bypass the
  Path's access rules.
- If the product exposes a copied or direct link to a Path or child item, the
  link must act only as navigation and must not grant permission by possession.
- The initial Path Share experience must provide a `Copy link` action for the
  Path.
- The copied Path link must open the Path for a recipient who is authorized
  under its current membership and visibility rules.
- The initial product must not provide copy-link sharing controls for an
  individual recorded activity session, goal, achievement, or feed event.
- Omitting an item-level copy-link control must not prevent authorized internal
  navigation to that item's detail experience.
- Opening a link without sufficient access must use the opaque unavailable
  behavior defined in
  [Profile and Path Visibility](visibility.spec.md#inaccessible-paths).
- Sharing one user's information must not allow another user to record time,
  complete goals, or claim achievements on that user's behalf.

## Related behavior

- General audiences and profile defaults are defined in
  [Profile and Path Visibility](visibility.spec.md).
- Explicit Path access is defined in
  [Path Membership](../paths/membership.spec.md) and
  [Path Roles](../paths/roles.spec.md).
- Feed-event visibility and interactions are defined in
  [Social Feed](feed.spec.md).
- Direct encouragement and its audience controls are defined in
  [Nudges and Direct Encouragement](nudges.spec.md).
- Notification delivery and user-controlled channels are defined in
  [Notifications](../notifications/notifications.spec.md).
