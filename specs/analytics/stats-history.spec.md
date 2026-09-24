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
