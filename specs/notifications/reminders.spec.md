# Goal-Aware Practice Reminders

Status: Approved for implementation

## Purpose

Define optional reminders that help a participant remain on track while there is
still enough time to take useful action.

## Product intent

- Practice reminders must help a participant make timely progress toward the
  path's interval goal.
- Reminders must be based on the individual participant's progress, interval,
  and configured time zone.
- Reminders must be optional and controllable by the receiving user.

## Participant controls

- Goal-aware reminders must default to enabled for each participant on a Path
  that has an interval goal.
- Each participant must be able to disable or re-enable reminders independently
  for each Path.
- A creator or administrator must not be able to change another participant's
  reminder preference.
- A participant's per-Path setting must further narrow the global practice-
  reminder notification channel and must not override that channel when it is
  disabled.
- Changing one Path's reminder setting must not change reminder behavior for
  another Path.
- Goal-aware reminder planning must honor the optional profile-wide recurring
  unavailable period defined in
  [User Preferences](../accounts/preferences.spec.md).
- Device operating-system notification controls may still suppress or schedule
  delivery independently of the application.

## Delivery

- Each goal-aware reminder and no-longer-achievable notice must be delivered as
  both an in-application notification and an operating-system push notification.
- The two surfaces must represent one underlying notification event rather than
  separate reminders or notices.
- Push delivery must remain subject to operating-system permission and the
  profile-wide recurring unavailable period; suppressing push must not suppress
  an otherwise enabled in-application notification.

## Actionability

- Reminder timing must consider both the practice time still needed and the time
  remaining in the participant's current interval.
- The goal-aware reminder must target the last meaningfully actionable portion
  of the interval: the participant must still have enough time to complete the
  remaining practice, plus a safety buffer.
- The safety buffer must make completion realistic without sending the reminder
  so early that it loses its last-chance urgency.
- Reminder scheduling must use a fixed 30-minute safety buffer for every
  participant and interval.
- The safety buffer must not learn from prior sessions or vary with the interval
  duration, remaining practice time, Path, or participant.
- A standard practice reminder must not be sent so late that completing the
  remaining goal is no longer reasonably actionable.
- A participant who has already reached the current interval goal must not
  receive a reminder to complete that goal.
- Participants in the same path may receive reminders at different moments
  because their progress and configured time zones may differ.

### Counterexample

If a participant still needs 40 minutes of practice and only 10 minutes remain in
the interval, the application must not send an ordinary reminder suggesting that
the participant can still complete the goal in that interval.

## Unavailable-period planning

- When no recurring unavailable period is enabled, reminder planning must use
  the actual end of the participant's current goal interval as its deadline.
- When the next unavailable period begins before the current goal interval ends,
  reminder planning must use that unavailable period's start as the effective
  actionable deadline.
- The last-chance reminder must be scheduled early enough before the effective
  actionable deadline to fit both the participant's remaining practice time and
  the fixed safety buffer.
- An unavailable period must not change the real goal interval, its progress, or
  the participant's ability to record activity during that period.

### Example

A participant's daily interval ends at midnight, their unavailable period begins
at 10:00 PM, and they still need 40 minutes. Reminder planning must treat 10:00
PM as the effective actionable deadline and send the last-chance reminder at
8:50 PM: 40 minutes of remaining practice plus the fixed 30-minute buffer before
10:00 PM. The participant may still record activity and reach the goal until
midnight.

## Frequency

- Each participant must receive at most one goal-aware reminder for a given Path
  during each of that participant's goal intervals.
- Delivery through both the in-app and push surfaces must represent the same
  reminder and must not count as separate reminders.
- Starting a new goal interval must make the participant eligible for that
  interval's reminder, subject to their notification settings and the
  actionability rules.

## Bundling

- Goal-aware reminders scheduled no more than five minutes apart for the same
  participant must be treated as due together for bundling.
- A bundle must be delivered at its earliest included reminder time; the
  application must not hold it until the five-minute window ends.
- At that earliest time, the bundle must include other eligible reminders
  scheduled within the following five minutes, delivering those reminders up to
  five minutes earlier than their individual scheduled times.
- The five-minute window must be anchored to the earliest scheduled reminder in
  a bundle; a later reminder outside five minutes of that earliest reminder must
  begin or join a different bundle even if it falls within five minutes of the
  bundle's latest reminder.
- When goal-aware reminders for multiple Paths become due together for the same
  participant, the application must combine them into one bundled notification
  rather than deliver one notification per Path.
- The bundle must identify every included Path clearly enough for the
  participant to understand which goals need attention.
- The bundle must appear as one in-application notification and one push
  notification, subject to the ordinary delivery rules.
- Each included Path must count as having delivered its one goal-aware reminder
  for that interval even though the participant received one bundle.
- Opening the bundle must take the participant to an in-application view from
  which every included Path can be reached.
- A reminder that is disabled, no longer actionable, already delivered, or
  suppressed by an active timer must not be included merely because another
  Path has an eligible reminder.

### Example

Reminders scheduled for 8:00 PM and 8:05 PM must share a bundle. A reminder
scheduled for 8:06 PM must not join that bundle, even though it is only one
minute later than the 8:05 PM reminder. The bundle must be delivered at 8:00 PM.

## Active timers

- While the participant has an active timer on the relevant Path, its live
  elapsed time must count as provisional progress when evaluating reminder
  timing and goal achievability.
- While that timer remains active, the application must deliver neither a
  goal-aware reminder nor a no-longer-achievable notice for that Path.
- A reminder or notice suppressed by the active timer must create neither an
  in-application notification nor a push notification and must not count as
  delivered for the interval.
- When that timer stops before the effective actionable deadline, the
  application must immediately recalculate the participant's remaining goal
  progress, reminder timing, and goal achievability using the recorded activity.
- If the goal remains incomplete, no reminder has been delivered in the
  interval, and the recalculated reminder remains actionable under the ordinary
  remaining-practice and safety-buffer rules, the application must deliver the
  reminder immediately.
- If those conditions are not met, stopping the timer must not generate the
  skipped reminder.
- An active timer on a different Path must not suppress the relevant Path's
  reminder.

## No-longer-achievable notice

- If an incomplete interval goal becomes impossible to complete using all of
  the actual time remaining in the interval, the participant must receive a
  no-longer-achievable notice.
- The no-longer-achievable notice must be distinct from the actionable
  goal-aware reminder and must not count against the one-reminder-per-interval
  limit.
- A participant must receive at most one no-longer-achievable notice for a given
  Path during each interval.
- This notice must respect the participant's global practice-reminder channel
  and per-Path reminder preference.
