# User Preferences

Status: Approved for implementation

## Purpose

Define user-level defaults that make creating goals convenient without changing
the meaning of goals that already exist.

## First day of the week

- Each user profile must have a first-day-of-the-week preference.
- At account creation, the preference must initially use the convention for the
  device's current locale.
- The user must be able to change the preference.
- The preference must provide the default weekly alignment when the user creates
  a weekly interval goal.
- The user must be able to choose a different weekly alignment for an individual
  goal during creation.
- Saving a weekly goal must copy the chosen alignment into that goal.
- Changing the profile preference must affect defaults for subsequent goal
  creation only.
- Changing the profile preference must not alter existing goals, including goals
  that are shared with or visible to other users.

## Time zone

- Each user profile must have a configured time zone.
- At account creation, the configured time zone must initially use the device's
  current IANA time zone without adding an onboarding question.
- The user must be able to change their configured time zone in settings.
- A user's configured time zone must change only through an intentional user
  action.
- A change to the device's time zone or the user's physical location must not
  automatically change the configured time zone.
- Traveling across time zones must not, by itself, shift the user's goal interval
  boundaries or alter the time remaining in an interval.
- Goal progress for a participant must use that participant's configured time
  zone.
- Viewing another participant's progress must not recalculate that participant's
  interval using the observing user's time zone.
- An intentional time-zone change must take effect immediately.
- Applying the change must immediately recalculate the user's current interval
  boundaries and remaining time in the new time zone.
- Each intentional time-zone change must be retained with the instant at which
  it became effective.
- The application must preserve the user's effective-dated time-zone history
  rather than overwrite the prior setting as though it had always applied.
- A time-zone change must apply to new timers, manual entries, and other new
  activity from its effective instant; it must not move prior activity to
  different historical calendar dates.
- A timer already running when the time zone changes must retain the time zone
  captured when that timer started until it is stopped.
- Time zones must be represented by IANA time-zone identifiers, such as
  `America/New_York`, rather than by a fixed UTC offset alone.
- Before the change is finalized, the user must be warned that it will change the
  amount of time remaining in their current interval.
- The time-zone change must require explicit confirmation after the warning is
  shown.
- A confirmed time-zone change must identify the exact configured time zone the
  user reviewed. If that configured value changed before the update is applied,
  the update must fail as a conflict instead of overwriting the newer choice.
- Retrying the same confirmed change must return its original result without
  appending another history entry. Reusing its idempotency identity for a
  different change must fail as a conflict.
- Confirming the currently configured time zone must succeed as an unchanged
  result and must not append a duplicate history entry.
- Canceling or dismissing the confirmation must leave the configured time zone
  and current interval unchanged.

## Native preference-settings presentation

- Time Zone must open from the compact Settings hierarchy as a titled native-
  stack destination with platform back and edge-swipe behavior.
- The current value, keyboard-safe native search, available IANA choices, exact
  selected state, warning, confirmation, saving, failure, and retry must remain
  distinct and localized without depending on a custom content card, color,
  translucency, or motion alone.
- Rows and required actions must reflow without clipping at maximum Dynamic Type
  and remain operable with the keyboard shown and with VoiceOver, Reduce Motion,
  Reduce Transparency, and Increase Contrast.
- A cold or direct Time Zone route must mount visible loading or recovery and
  must not render blank while session context or the authoritative preference
  resolves.
- Preference state and a reviewed change must remain owned by the activated
  account/profile session lineage and exact intent. Same-account credential
  rotation may preserve owned work; account replacement or sign-out must clear
  the prior value and cancel late load, confirmation, save, failure, or retry
  completion.

## Locale

- The initial product must not store an application-specific locale preference
  on the user profile.
- Locale-sensitive language, dates, times, numbers, and durations must follow
  the current device locale.
- A later device-locale change may update locale-sensitive presentation, but it
  must not change the saved first-day-of-the-week preference or the user's
  configured time zone.
- The application must not ask the user to choose a locale during onboarding.

## Goal-creation defaults

- The initial user profile must not store a default goal recurrence, interval
  target duration, or overall target duration.
- Creating a new Path must leave its optional interval goal and overall target
  unset until the user deliberately configures them.
- Creating or editing one Path's goals must not silently establish defaults for
  another Path.
- The first-day-of-the-week preference may continue to supply weekly alignment
  as already specified, but it must not cause a weekly goal to be selected by
  default.

## Recurring unavailable period

- Each user profile must support at most one recurring daily unavailable period.
- The unavailable period must default to disabled.
- The user must be able to enable, change, and disable it in profile settings.
- When enabled, it must contain a local start time and local end time and must
  recur every day.
- The period may cross midnight, such as 10:00 PM through 8:00 AM.
- Its local times must be interpreted using the user's intentionally configured
  IANA time zone, not the device's current time zone.
- The unavailable period must not alter Path goal intervals, recorded activity,
  or the time during which a participant may track activity.
- While the unavailable period is active, it must suppress operating-system push
  delivery for every application notification category.
- The unavailable period must not suppress creation or display of in-application
  notifications.
- Its effect on goal-aware reminder planning is defined in
  [Goal-Aware Practice Reminders](../notifications/reminders.spec.md).
- Its cross-category push-delivery effect is defined in
  [Notifications](../notifications/notifications.spec.md).
