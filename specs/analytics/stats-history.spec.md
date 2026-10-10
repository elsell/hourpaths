# Stats and History

Status: Approved for implementation

## Purpose

Define the broad analytical surface for understanding a user's activity across
paths and over time.

## Scope

- Stats/History must provide aggregate views across the user's paths.
- Stats/History must open with the user's own recorded time aggregated across
  all of their paths.
- Stats/History must include only the current user's recorded activity and must
  not aggregate or display another participant's time from a shared Path.
- Shared participant comparisons must remain within the relevant Path detail
  experience rather than the global Stats surface.
- Stats/History must provide a multi-select Path filter.
- The default selection must be `All Paths`.
- The user must be able to select one Path or any combination of multiple Paths,
  and every aggregate, chart, calendar, and distribution on the surface must
  update to use only the selected Paths.
- Filtering to one Path must not turn Stats/History into the Path detail view;
  the surfaces must retain their distinct purpose and navigation.

## High-level analysis

- Stats/History must show the distribution of the user's recorded time across
  paths.
- Cross-Path comparisons and distributions in the initial product must compare
  tracked duration only.
- Global Stats must not compare or aggregate goal-completion percentages across
  Paths, because Paths may use different interval recurrences and targets.
- Selecting multiple Paths with different goal configurations must not prevent
  their recorded time from being aggregated or compared.
- Stats/History must include a calendar view of the user's total recorded time
  across all paths.
- The calendar view must support daily, weekly, monthly, and yearly
  aggregation.
- The Stats date-range control must offer `Day`, `Week`, `Month`, `Year`, and
  `All Time` presets.
- `Day`, `Week`, `Month`, and `Year` must initially select the current calendar
  period and must allow navigation to adjacent completed or current periods.
- `All Time` must include the current user's complete available recorded history
  for the selected Paths.
- The initial product must not provide an arbitrary custom start-and-end date
  range.
- Stats/History must show activity over time.
- Stats/History must support time-based measures such as minutes tracked over
  time.
- Stats/History must support interval summaries such as activity per day.
- Historical calculations must derive from recorded activity rather than stored
  goal-progress snapshots.

## Initial chart presentation

- The initial Stats presentation must use a donut chart to show distribution of
  tracked time across the selected Paths for the selected date range.
- The initial Stats presentation must use a bar chart to show tracked activity
  over time, using time buckets appropriate to the selected date range.
- The default bar-chart buckets must be hours for `Day`, days for `Week` and
  `Month`, months for `Year`, and years for `All Time`.
- When an `All Time` selection does not span enough history for yearly buckets
  to be useful, it must use the largest smaller calendar bucket that produces a
  meaningful multi-point view.
- A range with no activity must show a plain zero-activity state while retaining
  the selected filters and range controls; it must not fabricate chart values.

- The daily calendar must use a contribution grid: seven weekday rows, chronological
  week columns, and discrete color intensity for recorded duration. It must include
  zero-activity dates within the selected period, respect the configured week start,
  and expose each date and exact duration through accessible text and selection.
- On every client, the grid must render every calendar date from the selected
  range start through its end, including leading, intermediate and trailing
  zero-activity dates. An entirely empty range must retain its full grid.
  Activity entries must not determine or shorten the displayed date boundaries.
- The contribution grid must be the default calendar presentation for every date
  range; weekly, monthly, and yearly summaries remain selectable.
- Native range and calendar-grouping selectors must use paired platform
  foreground/background colors so selected labels retain readable contrast.
  A brand tint must not override only the selection background.
- Horizontally scrolling activity charts must initially show the most recent
  dates when a range or Path filter changes. Refreshes must preserve manual
  exploration of earlier dates.

## Historical calendar attribution

- Calendar statistics must attribute recorded activity using the participant's
  configured IANA time zone that was effective when the activity occurred.
- A later intentional time-zone change must not move earlier activity to a
  different displayed day, week, month, or year.
- New timers, manual entries, and other new activity must use the new time zone
  from the change's effective instant; a timer already running must retain its
  starting time zone as defined in
  [Recorded Activity](../tracking/recorded-activity.spec.md#running-timer-lifecycle).
- When one activity crosses a calendar boundary in its applicable time zone,
  each elapsed portion must contribute to the corresponding calendar period
  without duplicating time or splitting the source activity record.

## Delivery and failure behavior

- Stats must be available through the existing primary navigation in the mobile
  and web clients, using each client's shared controls, spacing, and typography.
- Loading, retry, and zero-activity states must retain the selected Path filter
  and date range. A failed refresh must not display data for a different filter.
- Every chart must provide the corresponding dates, Paths, and recorded durations
  as accessible text. Color alone must not identify a Path or a calendar value.
- Switching accounts or signing out must clear the previous account's Stats data
  and discard late responses from that account.
- Aggregation must include the complete eligible recorded history, not only a
  client's currently loaded page of Paths or activity. Deleted or inaccessible
  records must not contribute after refresh.

## Acceptance scenarios

- Given two Paths containing the user's recorded activity and another
  participant's sessions, All Paths shows only the user's time. Selecting either
  Path updates every summary, distribution, chart, and calendar consistently.
- Given a recorded session from 23:30 to 00:30 in its occurrence time zone, each
  adjacent date receives 30 minutes. Changing the current preference later must
  not move that recorded time into a different date.
- Given daylight-saving transitions, recorded elapsed time must be conserved
  across the historical calendar buckets, including repeated or skipped hours.
- Given no activity in the selected period, controls remain usable and the
  surface shows zero recorded time without invented chart values.
