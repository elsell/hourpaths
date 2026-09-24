# Accessibility

Status: Approved for implementation

## Purpose

Define the cross-platform accessibility baseline for every user-facing product
surface.

## Standards baseline

- User-facing experiences must conform to WCAG 2.2 Level AA wherever its
  success criteria apply.
- Native iOS and Android experiences must also follow their platform
  accessibility conventions and use platform accessibility semantics.
- A platform convention may refine how an accessible behavior is presented but
  must not be used to weaken the WCAG 2.2 Level AA baseline.

## Perceivable state

- Color must not be the only means of communicating progress, goal completion,
  privacy, warnings, errors, synchronization state, timer state, or selection.
- The charcoal-and-yellow visual theme must maintain the required text,
  component, focus, and graphical contrast.
- Native materials and custom content must remain legible with Increase
  Contrast and Reduce Transparency enabled. A translucent or glass effect must
  not be the only boundary, grouping cue, or state indicator.
- Text and meaningful controls must remain usable with supported operating-
  system text scaling and display-size settings.
- User-readable native text must not cap a supported operating-system text-size
  preference below the size selected by the user. Decorative glyphs that are
  hidden from assistive technology may remain fixed-size when scaling would
  distort their non-text purpose.
- Information must remain understandable without requiring a specific device
  orientation unless the content itself makes that orientation essential.

## Semantics and operation

- Every interactive control must expose an accessible name, role, current
  state, and available action through the platform accessibility system.
- The complete visible area of a control must be operable, and every required
  primary action must provide at least a 44-by-44-point touch target.
- Required primary actions must remain reachable within the safe area on every
  supported phone viewport and when supported text scaling causes content to
  reflow; the screen must scroll when the content cannot otherwise fit.
- Content presented beneath a native navigation bar must use the navigation
  container's automatic scroll insets and must not add a second top safe-area
  inset. It must continue to protect the left, right, and bottom safe-area
  edges.
- A native form sheet that can present the software keyboard must adjust its
  scrollable content for the keyboard and must allow the user to reach and
  operate every required action without first dismissing the keyboard.
- A screen that is preparing a required action must communicate that progress
  in text and through platform status semantics instead of presenting an
  unexplained disabled control.
- The initial application loading state must pair its localized status text
  with a native indeterminate activity indicator and progress semantics.
- Timer start and stop controls, progress indicators, privacy indicators,
  navigation, notification state, and destructive confirmations must remain
  operable with supported assistive technologies.
- Focus order and reading order must follow the logical task sequence rather
  than incidental visual placement.
- Required actions must not depend solely on gestures that lack an accessible
  alternative.

## Charts and activity grids

- Every chart, progress visualization, and activity-grid cell must have an
  accessible label that communicates its relevant value and context.
- A native progress control's numeric accessibility value must remain within
  its declared minimum and maximum. When progress exceeds a target, its
  localized accessibility text must still communicate the uncapped value.
- Charts and the GitHub-style activity grid must provide an accessible textual
  summary of the information conveyed visually.
- Intensity, color, or shape differences in a chart or grid must not be the only
  way to distinguish values or states.
- Accessible chart and grid content must use the same underlying values as the
  visual presentation.

## Errors, warnings, and status

- Validation errors must identify the affected input and explain the problem in
  text.
- Status changes such as timer start, timer stop, synchronization failure, and
  successful destructive confirmation must be exposed to assistive technologies
  without relying solely on a transient visual cue.
- Warning and confirmation experiences must preserve accessible focus and must
  not obscure the action or consequence being confirmed.

## Motion and progressive disclosure

- User-interface transitions must honor the operating system's reduced-motion
  preference. Removing or reducing animation must not hide a state change,
  delay required content, or remove feedback needed to complete a task.
- Expanding or collapsing optional form content must preserve a logical focus
  position and expose the resulting expanded or collapsed state through native
  accessibility semantics.
- Progress, validation, failure, retry availability, and success must remain
  understandable when motion is disabled and must not be communicated by
  animation alone.

## References

- [WCAG 2.2](https://www.w3.org/TR/WCAG22/)
- [Apple Human Interface Guidelines](https://developer.apple.com/design/human-interface-guidelines)
- [Android accessibility principles](https://developer.android.com/guide/topics/ui/accessibility/principles)
