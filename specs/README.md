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

Approved account-session precision: new sign-ins last 30 days with a fixed
absolute deadline and immediate explicit revocation; see
[Authentication](accounts/authentication.spec.md#remembered-sign-in-lifetime).

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
| Native ongoing timer surfaces | Covered | Covered | Covered | Covered | [`platform/clients.spec.md`](platform/clients.spec.md#active-timer-visibility) |
| Widgets | Deferred | Deferred | Deferred | Deferred | [`integrations/widgets.spec.md`](integrations/widgets.spec.md) |
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

September 2026 Home revision: approved stable colorful mobile tiles, native flat
controls, consistent period totals, and Running jump shortcuts. Appearance choices are personal to each user and persist server-side across
all clients, as approved in the Home specification.

Approved mobile refinement: compact one-page Path detail/edit surface and Home tile navigation; see [Path details](experience/path-details.spec.md#compact-mobile-detailedit-page-approved-september-2026).

Approved social redesign: [mobile feed, profiles, and live activity viewer](experience/social-mobile.spec.md).

Approved Studio web rebuild: [experience and delivery slices](experience/web-studio.spec.md),
[domain isolation and TanStack architecture](platform/web-studio-architecture.spec.md).
Following omits the people-list sidebar; timelines remain connected across days.

Delivery completion is tracked separately from specification coverage in the
[ordered gap-closure checklist](platform/completion-checklist.spec.md). A Covered
cell above does not establish implementation, release, or device acceptance.

Profile-picture delivery boundaries: [safe processing, storage and cleanup](platform/profile-picture-storage.spec.md).

Profile connection-list browsing and inbound follower removal acceptance is specified in [SOC-01C](social/following.spec.md#profile-connection-lists-and-follower-removal-soc-01c).
