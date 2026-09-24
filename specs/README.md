# Behavior Specification Coverage

This index tracks breadth across the application and prevents one product area
from being specified deeply while major behavior elsewhere remains undefined.
The detailed `*.spec.md` files remain authoritative.

## Interview passes

1. **Product shape:** purpose and major behavior in every area.
2. **Primary workflows:** normal user journeys, roles, and permissions.
3. **Lifecycle and failure:** transitions, deletion, errors, and cross-domain
   effects.
4. **Precision:** calculations, boundary conditions, synchronization conflicts,
   and implementation-level behavioral details.

## Coverage map

| Product area | Pass 1 | Pass 2 | Pass 3 | Pass 4 | Current authoritative specs |
| --- | --- | --- | --- | --- | --- |
| Product scope and terminology | Covered | Covered | Covered | Covered | [`paths/core.spec.md`](paths/core.spec.md) |
| Accounts, profiles, and preferences | Covered | Covered | Covered | Covered | [`accounts/authentication.spec.md`](accounts/authentication.spec.md), [`accounts/onboarding.spec.md`](accounts/onboarding.spec.md), [`accounts/profile.spec.md`](accounts/profile.spec.md), [`accounts/preferences.spec.md`](accounts/preferences.spec.md) |
| Path creation and lifecycle | Covered | Covered | Covered | Covered | [`paths/core.spec.md`](paths/core.spec.md), [`paths/lifecycle.spec.md`](paths/lifecycle.spec.md) |
| Membership, roles, and invitations | Covered | Covered | Covered | Covered | [`paths/roles.spec.md`](paths/roles.spec.md), [`paths/membership.spec.md`](paths/membership.spec.md), [`paths/permission-matrix.spec.md`](paths/permission-matrix.spec.md) |
| Recording time | Covered | Covered | Covered | Covered | [`tracking/recorded-activity.spec.md`](tracking/recorded-activity.spec.md) |
| Goals and progress | Covered | Covered | Covered | Covered | [`goals/time-goals.spec.md`](goals/time-goals.spec.md) |
| Visibility and privacy | Covered | Covered | Covered | Covered | [`social/visibility.spec.md`](social/visibility.spec.md) |
| Friends and discovery | Covered | Covered | Covered | Covered | [`social/following.spec.md`](social/following.spec.md) |
| Feed, reactions, comments, and encouragement | Covered | Covered | Covered | Covered | [`social/feed.spec.md`](social/feed.spec.md), [`social/nudges.spec.md`](social/nudges.spec.md) |
| Home and primary navigation | Covered | Covered | Covered | Covered | [`experience/home.spec.md`](experience/home.spec.md), [`experience/navigation.spec.md`](experience/navigation.spec.md) |
| Accessibility | Covered | Covered | Covered | Covered | [`experience/accessibility.spec.md`](experience/accessibility.spec.md) |
| History, calendar, and statistics | Covered | Covered | Covered | Covered | [`analytics/stats-history.spec.md`](analytics/stats-history.spec.md), [`experience/path-details.spec.md`](experience/path-details.spec.md) |
| Notifications | Covered | Covered | Covered | Covered | [`notifications/notifications.spec.md`](notifications/notifications.spec.md), [`notifications/reminders.spec.md`](notifications/reminders.spec.md) |
| Offline behavior and synchronization | Covered | Covered | Covered | Covered | [`sync/offline.spec.md`](sync/offline.spec.md) |
| Widgets and operating-system integration | Deferred | Deferred | Deferred | Deferred | [`integrations/widgets.spec.md`](integrations/widgets.spec.md) |
| Data deletion, safety, and moderation | Covered | Covered | Covered | Covered | [`accounts/deletion.spec.md`](accounts/deletion.spec.md), [`safety/moderation.spec.md`](safety/moderation.spec.md), [`safety/user-policies.spec.md`](safety/user-policies.spec.md) |
| Platform and configuration behavior | Covered | Covered | Covered | Covered | [`platform/clients.spec.md`](platform/clients.spec.md), [`platform/configuration.spec.md`](platform/configuration.spec.md), [`platform/api-behavior.spec.md`](platform/api-behavior.spec.md) |

## Readiness

Every initial-product area has completed all four specification passes and is
approved for implementation. The home-screen widget remains an explicitly
deferred enhancement and does not block the initial product.

## Acceptance verification

Cross-domain implementation scenarios are maintained in
[`acceptance/initial-product.scenarios.spec.md`](acceptance/initial-product.scenarios.spec.md).
They supplement the authoritative domain requirements and the derived
[`paths/permission-matrix.spec.md`](paths/permission-matrix.spec.md).
