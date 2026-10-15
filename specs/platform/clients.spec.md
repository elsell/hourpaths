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
- Replacing or normalizing a native route must not dismiss its shared content
  while another mounted screen still represents that route. Ordinary Back
  navigation must release the content when the last screen leaves. Delayed
  cleanup must not dismiss a replacement account's presentation.

## Apple-native presentation and Liquid Glass

The mobile client uses the repository-pinned React Navigation 7.3.8 and native
stack 7.17.10 already resolved by Expo Router. Expo's recommended-version check
excludes these direct declarations; the lockfile, type check, route tests, and
native builds verify this exact existing combination.

### Shared mobile design language

- Comment input and submission must remain visible above the Android keyboard,
  including at enlarged text sizes. Keyboard positioning must account for the
  native header and the window-to-screen status-bar inset.
- Multiline comment inputs must not attach React Native's per-field
  `InputAccessoryView`; the pinned React Native version documents multiline
  input as unsupported for that component. Comments must instead retain native
  keyboard dismissal gestures on iOS and Android.

- List recovery and pagination actions must use the shared button contract,
  preserving loading labels, busy state, and mutation-related disabling.
- Recovery and arrangement actions must use the shared button contract. Path
  names must retain their own row width when arrangement actions wrap.
- Invitation visibility warnings must use the shared sheet title and cancellation
  action. Explicit acceptance must follow the complete privacy disclosure.

- Android overflow actions must use a native scrollable menu so every action is
  available, including when there are more than three. Disabled actions must
  remain visible and disabled; destructive actions must be distinguishable.
- Symbols must express the same meaning on both platforms, using SF Symbols on
  iOS and Material symbols on Android. Tab destinations must have both platform
  symbol definitions. Selection must have a visible marker and checked or
  selected accessibility state, independent of color.
- Platform-specific controls must consume the same shared action, icon, and
  button contracts. Existing pinned `expo-symbols` 55.0.9, already shipped through
  Expo Router, supplies the Android symbol adapter without upgrading native code.
  The mobile app must directly declare its native peer `expo-font` at the existing
  resolved version 55.0.8 so native autolinking and Expo Doctor include it.

- Settings and secondary navigation must use the same grouped rows, disclosure
  indicators, spacing, and separators. Long identity text must have its own
  available width rather than compete with a second long trailing value.
- Social activity actions must form one compact group directly beneath their
  activity. Search empty-state headings must identify search, and notification
  copy must focus on the event rather than explain internal classifications.


- Mobile screens must use one shared navigation theme matching the existing
  charcoal content and yellow action palette. Native navigation titles, search,
  tab selection, and status bars must remain legible regardless of device appearance.
  Android sheet frames must apply that background to their header and top safe
  area as well as their scrolling content. Their content must resize for the
  keyboard so lower form fields can scroll into view while actions stay visible.
- Goal switches must respond to taps on both their label and native switch
  surface, with a single accessible checked state.
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
- Production iOS bundles must select the native Live Activity adapter rather
  than the unsupported-platform fallback. A running timer started in the
  foreground must appear on the Lock Screen when Live Activities are enabled.
- Native iOS builds must link the HourPaths timer module into the application
  and register it with Expo. Its minimum deployment target must be compatible
  with the containing app; unsupported Live Activity versions remain guarded
  at runtime. Simulator and signed-release checks must fail when that module
  or registration is absent.
- Foreground storage and native timer controls in the same iOS process must
  use the same SQLite library instance. Opening an existing WAL database during
  timer restoration must not crash or bypass an active writer transaction.
  Verification must exercise native controls against the Expo SQLite engine,
  including writer contention and an existing foreground reader.
- A failed native presentation update must receive bounded automatic retries
  without requiring another timer action. A newer projection, account change,
  or sign-out must cancel the obsolete retry; retries must never resurrect
  revoked native controls.
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

### Live Activity presentation and controls

- The approved iOS presentation must show each visible Path's personal emoji,
  palette accent, name, native elapsed session time, and current goal progress
  when a goal exists. Compact Dynamic Island presentation must prioritize the
  Path emoji and elapsed time instead of a bare timer count.
- Expanded Dynamic Island and Lock Screen presentations must provide clearly
  labeled Start/Stop controls for the displayed Paths, with state reflecting
  the authoritative tracking projection. Controls must remain usable when the
  application is suspended; a visual-only state change is not a successful timer
  command.
- Background synchronization after a native Stop must preserve that Path’s
  Start control while its account lease and Path access remain valid. Returning
  to the app may dismiss a surface with no running timers.
- Native actions must use the same account-scoped durable tracking semantics as
  in-app controls, including offline operation, synchronization, and saving
  elapsed time on Stop. Duplicate or stale actions must not toggle a newly
  started session or affect another signed-in account.
- Background commands must share the in-app timer mutations and persist to the
  same durable tracking ledger under an atomic revision check. The native
  execution adapter must check an unexpired account lease and the deletion
  fence inside the writer transaction. Sign-out and account replacement must
  invalidate that lease before stale native actions can commit. Stop-and-save
  sign-out must suppress lease rebinding and read the durable timers after
  revocation so a native Start immediately before revocation is included.
- Native presentation must use shared Path appearance and goal calculations,
  localized labels, accessible contrast and system-sized touch targets. A
  missing goal must omit goal presentation rather than invent a target.
- Acceptance must demonstrate Start and Stop from the Lock Screen and expanded
  Dynamic Island, including suspended-app execution, offline reconciliation,
  repeated taps, and account replacement. A successful foreground-only tap is
  insufficient evidence for these controls.

### Live Activity goal-period refresh

- A running Live Activity must advance to the current goal period without
  requiring the person to reopen HourPaths. Period boundaries and recorded-time
  attribution must use the same participant calendar and durable tracking
  projection as Home, including daylight-saving changes and pending offline work.
- A remote refresh must not start, stop, overwrite, or resurrect a local timer.
  The device’s durable tracking state remains authoritative for pending commands;
  delayed or duplicate refreshes must preserve newer local actions.
- The iOS adapter must keep the full presentation in the shared App Group and
  publish only its account/lease identity and refresh deadline to ActivityKit.
  Native actions and revocation must serialize presentation writes. Before a
  durable action commits, the previous mirror must be invalidated so a crash
  cannot leave a newer ledger paired with old actionable controls. Missing,
  expired, or mismatched local presentation must render no personal content;
  remote data must not be used as a fallback. Foreground reconciliation may
  republish the durable projection after such an interruption.
- Refresh delivery must use the APNs Live Activity update protocol over HTTP/2,
  with a bounded request deadline and a refresh deadline no later than the
  native presentation expiry. It must not follow provider redirects or include
  timer commands, Path content, tokens, or provider response bodies in logs.
  Invalid activity tokens must be distinguished from retryable transport/rate
  failures and signing-configuration failures. Provider credentials must be
  injected by bootstrap, never read by domain or application code.
- Registration must bound its lifetime to eight hours and carry no more than 64
  ordered goal-period boundaries across the displayed Paths. A delayed wake must
  select the next future deadline, never replay an obsolete period. Persistence
  must fence claimed work against registration changes and revocation before
  provider handoff, and must bound active registrations per account.
- Refresh registration and delivery must be scoped to the authenticated account
  and active native presentation. Sign-out, account replacement, deletion, lease
  expiry and presentation dismissal must invalidate that registration. Tokens
  must be encrypted at rest and must not appear in logs or audit metadata.
- Cleanup after process restart must recover the prior presentation identity from
  durable local storage before clearing it. Failed cleanup for a prior account
  must not prevent a different account from registering its own presentation.
- Client registration must serialize token rotation and revocation, discard queued
  updates from an obsolete lease, and retry failed registration when foreground
  reconciliation runs. Repeated presentation updates must not extend the original
  activity expiry. Registration failures must remain independent of local actions.
- An unavailable network or rejected refresh must not prevent Start/Stop or erase
  pending work. An expired goal period must not be displayed as the current goal;
  current-period progress must recover from retained state when rendering resumes.
- Acceptance must cover a running timer across an hourly boundary, a stopped
  timer followed by a delayed refresh, a new local Start followed by an older
  refresh, account replacement, token rotation and missed-network recovery.
  Passing a foreground-only test does not establish suspended-device acceptance.

### Native timer surface acceptance

- Native timer presentation must consume the durable account-scoped tracking
  projection, including offline starts and stops, instead of maintaining a
  second timer authority.
- Updating or denying an operating-system surface must not roll back a timer
  mutation or discard a pending synchronization operation.
- Restoring an authenticated account and returning to the foreground must
  reconcile its current timers with existing native surfaces. Reconciliation
  must not create duplicate consolidated surfaces.
- Sign-out, credential removal, account replacement, and permanent deletion
  must invalidate queued presentation updates before removing native surfaces.
  A delayed update from the prior account must not restore its Path names.
- Elapsed time must use the original session start instant and native clock
  presentation, so suspending JavaScript does not freeze the displayed timer.
- A consolidated surface must identify the number of running timers and provide
  a route to the running timers in the app. Its expanded presentation must show
  running Path names within platform size limits. Each visible Path row must retain
  its name and native elapsed timer after widget serialization, for one timer
  as well as multiple timers. A remaining-timer summary must not hide those rows.
- Acceptance must cover two simultaneous timers, stopping one and then the last,
  offline restoration, denied presentation permission, and sign-out while
  retaining server timers. Signed-device evidence remains required.

### Home tile presentation APIs

- The mobile Home view may use React Native `AccessibilityInfo` and
  `findNodeHandle` for reduced-motion scrolling and focus transfer to an existing
  tile. The shared native button may use SwiftUI `contentShape` and `shapes`
  modifiers to preserve a complete minimum touch target. These presentation-only
  imports must not permit client transport outside generated adapters.

## TestFlight delivery evidence

- An upload receipt must not be described as availability to testers.
- Release diagnostics must distinguish Apple's build processing state from
  internal/external beta distribution state and existing tester-group assignment.
- Diagnostics must use the existing App Store Connect credential inside the
  protected release environment and must not log credentials or tester details.

## Store release notes

- Every delivered mobile release must include concise user-facing changes in its
  store release notes: TestFlight build notes for the current iOS beta channel
  and release notes on the Google Play internal release.
- Notes must derive from the exact stable release tag, covering first-parent
  changes since the previous stable tag. Reviewed `Release notes:` bullets in
  feature, fix, or performance commits should describe user-visible behavior;
  existing `TestFlight notes:` bullets and commit subjects provide fallback.
- Both stores must share these highlights, deduplicate them, and respect their
  text limits. Notes must not include credentials or private operational details.
- Apple publication must verify the bundle, version, build and saved notes. Play
  must attach notes to the version code returned by the bundle upload in the
  same release edit. Failed notes publication must fail delivery visibly.
- Store credentials must remain in their existing protected environments.
  Notes must not promote a build or change its intended distribution channel.
- Acceptance: two feature commits with the same highlight produce one bullet;
  maintenance-only releases receive a short reliability message; an Apple build
  mismatch fails before writing; Play sends notes on the exact uploaded release.

The shared mobile transient-feedback adapter may import only `FullWindowOverlay` from the existing pinned `react-native-screens` package, and React Native accessibility announcements. This permits presentation above native navigation, not broader native or transport capabilities. The client boundary must reject namespace/re-export and unreviewed package imports.


### Android screen-listener lifetime recovery

Native startup and reload must not invoke a freed screen-removal listener or a
listener owned by a destroyed React host. The pinned `react-native-screens`
4.23.0 Android dependency receives the reviewed upstream #4413 backport from
commit `204d36b45f2a718c6f86e00cfc70765cbde06565`; only its four C++ listener files
may change, with the `cpp/legacy` paths mapped to the pinned version's `cpp` paths.
The patch must be applied reproducibly through the package-manager lockfile.

Verification must include native compilation and repeated cold launches with
retained offline data, followed by ordinary navigation and reconnection. Existing
timer and pending-activity state must remain intact. The change does not alter
product navigation, offline authorization, or dependency versions.
