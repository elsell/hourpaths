# Client Platforms

Status: Approved for implementation

## Purpose

Define the client platforms included in the initial product without prescribing
the client implementation.

## Initial platform support

- The initial product must support Android.
- The initial product must support iOS.
- Android must support Android 10/API level 29 and later and must target API
  level 36 or later.
- iOS must support iOS 18 and later and must be built with the current
  App-Store-required SDK or later.
- The core product workflows specified for the initial product must be available
  on both platforms unless a specification explicitly documents a
  platform-specific exception.
- Platform support requirements must not, by themselves, mandate a particular
  client framework or code-sharing strategy.
- Platform-specific differences must be limited to native navigation and
  presentation conventions, permission workflows, and operating-system timer
  surfaces; core capabilities, authorization, and stored product behavior must
  remain equivalent.
- On iOS, status-bar appearance must be owned by the native navigation view
  controllers so route-specific native-stack presentation never invokes
  conflicting legacy status-bar management or displays a development error.
- A native cold or direct route must render a localized loading or recovery
  presentation while its authoritative destination resolves. Route correctness
  must not depend on a prior in-memory visit to Home, and resolution for one
  signed-in profile must not populate a replacement profile's navigation stack.

## Apple-native presentation and Liquid Glass

The mobile client uses the repository-pinned React Navigation 7.3.8 and native
stack 7.17.10 already resolved by Expo Router. Expo's recommended-version check
excludes these direct declarations; the lockfile, type check, route tests, and
native builds verify this exact existing combination.

### Shared mobile design language

- Settings and secondary navigation must use the same grouped rows, disclosure
  indicators, spacing, and separators. Long identity text must have its own
  available width rather than compete with a second long trailing value.
- Social activity actions must form one compact group directly beneath their
  activity. Search empty-state headings must identify search, and notification
  copy must focus on the event rather than explain internal classifications.


- Mobile screens must use one shared navigation theme matching the existing
  charcoal content and yellow action palette. Native navigation titles, search,
  tab selection, and status bars must remain legible regardless of device appearance.
- Root and nested stacks must share navigation defaults. System materials must
  retain native rendering; per-screen painted bars must not compensate for a
  mismatched navigation theme.
- iOS task sheets must use a native navigation bar with intrinsic-size actions,
  so keyboard and text-size changes cannot hide completion or cancellation.
- Buttons must share primary, secondary, quiet, and destructive semantics across
  platforms, including visible disabled state and accessible busy/selection state.
- Shared controls and screen patterns must own typography, spacing, action
  emphasis, grouping, and loading/error presentation. Screens should compose
  these patterns instead of independently recreating their appearance.
- Primary actions must be evident, related content and actions must be grouped,
  and secondary metadata must not visually dominate the task. Copy should state
  the user's task or recovery action without explaining implementation details.
- Keyboard-open, empty, populated, loading, error, and enlarged-text states must
  preserve readable titles and reachable actions. Secondary tasks must follow
  the route hierarchy in [Navigation](../experience/navigation.spec.md).


- On current Apple platforms, standard system navigation bars, tab bars,
  toolbars, sheets, popovers, and controls must be allowed to inherit Liquid
  Glass automatically. Custom bar, sheet, or popover backgrounds that cover or
  interfere with the system material or scroll-edge effects must be removed.
- Liquid Glass is a functional navigation-and-control layer above content. The
  application must not add glass effects to ordinary content cards, lists, or
  decorative layers, and may apply custom glass only sparingly when no standard
  component provides the required control.
- Native sheets must use system presentation behavior, including suitable
  detents and a grabber when the content benefits from resizing. Single-view
  tasks must use the native leading Cancel or Close and trailing Done or Save
  actions rather than duplicate in-content chrome.
- Menus, pickers, confirmations, symbols, back controls, and bar actions must use
  the standard platform component and icon where one exists. Custom controls
  must preserve the same semantics, spacing, interaction, and accessibility
  behavior when a system equivalent cannot meet the specified task.
- Liquid Glass and every custom element near it must remain usable with Reduce
  Motion, Reduce Transparency, Increase Contrast, and all supported Dynamic Type
  sizes. The content hierarchy and state must not depend on translucency,
  morphing, or color alone.
- These requirements follow Apple's current
  [Adopting Liquid Glass](https://developer.apple.com/documentation/TechnologyOverviews/adopting-liquid-glass),
  [navigation-controller](https://developer.apple.com/documentation/uikit/uinavigationcontroller),
  and [sheet](https://developer.apple.com/design/human-interface-guidelines/sheets)
  guidance.

## Active timer visibility

- On supported iOS devices, running timers must be shown through one
  consolidated Live Activity,
  including the Dynamic Island where available.
- On Android, running timers must be shown through one grouped persistent
  ongoing notification.
- These operating-system surfaces must represent all timers currently running,
  including simultaneous timers for different paths.
- A platform or device that does not support a particular presentation, such as
  the Dynamic Island, must still preserve and run the underlying timers.
- Denial of notification or Live Activity permission must not prevent the
  underlying timers from functioning.
- Signing out while choosing to keep timers running must remove every
  operating-system timer surface without stopping its underlying timer, as
  defined in
  [Account Authentication](../accounts/authentication.spec.md#signing-out-with-running-timers).
- Each client must provide a platform-appropriate route for revisiting denied
  notification permissions, including opening system settings when the platform
  does not allow an in-app re-request.
- Permanently deleting the account must end and remove every operating-system
  surface associated with that account's running timers.
