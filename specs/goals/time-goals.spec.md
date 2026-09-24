# Time Goals

Status: Approved for implementation

## Purpose

Define the optional time goals a path may use to measure each participant's
recurring consistency and long-term progress.

## Terminology

- **Accumulated time:** The total tracked time attributed to a path.
- **Interval goal:** A target duration to track during each recurring calendar
  interval, such as a day, week, or month.
- **Current interval:** The active occurrence of an interval goal's recurrence.
- **Overall target:** A target duration for the accumulated time on a path, such
  as 10,000 hours.
- **Calendar-aligned interval:** An interval whose boundaries follow calendar
  units rather than a fixed elapsed duration.

## Optional goals

- A path must be able to accumulate time indefinitely without any goal.
- A path may have one interval goal.
- A path may have one overall target.
- The interval goal and overall target must be independently optional.
- A path may have neither goal, either goal, or both goals.
- Goals must belong to the path, not to an individual participant.
- Every participant in a path must use the same interval goal and overall target
  configuration.
- A participant must not be able to configure a different personal goal within
  the same path.
- A user who becomes a participant must receive the path's existing goals.
- Removing or omitting a goal must not remove or stop the path's accumulated
  time.
- During Path creation, interval-goal and overall-target configuration must use
  separate, independently operable optional sections. Enabling, disabling, or
  editing one section must not enable, disable, reset, or otherwise change the
  other section's draft.
- Disabling an optional goal in an unsaved creation draft may hide that goal's
  configuration, but a failed creation attempt must preserve the complete local
  draft so retry does not require the user to reconstruct either goal.

## Interval goal behavior

- An interval goal must define both a target duration and a recurring interval.
- Goal durations and accumulated time must support precision to one second.
- The product must support exactly five interval recurrences: hourly, daily,
  weekly, monthly, and yearly.
- The product must not support arbitrary or custom recurrences.
- All interval goals must use calendar-aligned intervals.
- A monthly interval must follow calendar-month boundaries; it must not be
  modeled as a fixed number of elapsed seconds or days.
- Progress toward an interval goal must be calculated from time tracked within
  the current interval by each participant.
- Interval-goal progress must remain separate for each participant.
- Progress in one interval must not be presented as progress in a later interval.
- Time beyond an interval's target must remain in that interval and be reported
  as progress beyond the target; it must not roll into another interval.
- When an interval goal exists, Home must show progress for the current interval.

### Examples

- Track 30 seconds each hour.
- Track 10 minutes each day.
- Track 10 hours each week.
- Track 40 hours each month.

## Interval alignment

- Each recurrence must have a default calendar alignment.
- The user configuring the interval goal may override the default alignment.
- An hourly goal must start at a selected minute within the hour and default to
  minute zero.
- A daily goal must start at a selected hour within the day and default to hour
  zero.
- A weekly goal must start on a selected day of the week.
- A monthly goal must start on a selected day of the month and default to the
  first day.
- For each monthly boundary, the effective start day must be the lesser of the
  configured start day and the number of days in that calendar month.
- A yearly goal must start on a selected month and day and default to January 1.
- A yearly goal configured to start on February 29 must use February 28 as its
  boundary in a non-leap year.
- A goal must store its selected alignment as part of the goal.
- Calendar-alignment preferences may supply defaults during goal creation but
  must not be a live source of alignment behavior after creation.
- Changing a calendar-alignment preference must not alter any existing goal.
- Every user viewing or comparing a goal must see progress calculated with the
  alignment stored on that goal.
- Changing alignment must alter interval boundaries, not the amount of time
  required by the goal.

## Daylight-saving and offset transitions

- Calendar interval boundaries must be resolved using the applicable IANA time
  zone and its time-zone rules.
- Elapsed activity assigned between resolved boundaries must continue to use
  actual elapsed time between UTC instants rather than an assumed number of
  hours in a local day.
- When a configured local boundary occurs twice during a backward clock
  transition, the earlier occurrence must be used.
- When a configured local boundary does not exist during a forward clock
  transition, the boundary must move forward by the transition gap.
- As a result, an hourly or daily interval containing an offset transition may
  contain more or less elapsed time than an ordinary interval without changing
  its configured calendar alignment or target duration.

## Participant time zones

- User time-zone configuration is defined in
  [User Preferences](../accounts/preferences.spec.md#time-zone).
- The recurrence and calendar alignment stored on a path's interval goal must be
  the same for every participant.
- Each participant's interval boundaries must be evaluated in that participant's
  configured time zone.
- Participants may therefore be in different current intervals, or have
  different amounts of time remaining, at the same moment.
- A participant's progress must not be recalculated using another participant's
  or observing user's time zone.

## Sessions crossing interval boundaries

- When a running timer crosses an interval boundary, its elapsed time must be
  allocated across the intervals it overlaps.
- The portion elapsed before a boundary must count toward the interval ending at
  that boundary.
- The portion elapsed after a boundary must count toward the interval beginning
  at that boundary.
- A timer that crosses multiple boundaries must contribute the corresponding
  elapsed portion to each interval.
- No elapsed time may be omitted or counted in more than one interval.
- Splitting interval-goal progress must not split or duplicate the timer's
  contribution to accumulated time or overall-target progress.

### Example

A daily interval ends at midnight. A timer runs from 11:50 PM until 12:20 AM.
Ten minutes count toward the ending day, twenty minutes count toward the new day,
and thirty minutes count once toward accumulated time.

## Goals as projections over recorded activity

- Recorded activity is defined in
  [Recorded Activity](../tracking/recorded-activity.spec.md).
- Interval goals and overall targets must be applied as calculations over
  recorded activity; they must not be the source of recorded time.
- Changing a goal's target duration, recurrence, or alignment must take effect
  immediately.
- After a goal changes, current and historical progress must be recalculated from
  recorded activity using the new goal configuration.
- Only the current goal configuration must be used when presenting current or
  historical progress.
- Users must not be shown historical progress evaluated under a previous goal
  configuration.
- The product is not required to expose previous goal configurations to users.
- Recalculation must not alter the occurrence time or duration of any recorded
  activity.
- Changing a weekly goal to a daily goal must regroup historical activity into
  daily intervals using the new alignment.
- Lowering a daily target from 60 minutes to 30 minutes when 45 minutes were
  recorded in a day must report 45 of 30 minutes for that day.
- Restoring an earlier configuration must reproduce the corresponding progress
  view from the unchanged recorded activity.

## Confirming goal changes

- Authorization to manage goals is defined in
  [Path Roles](../paths/roles.spec.md#goal-management-authorization).
- Before a goal change is applied, the user must be clearly warned that the
  change will recalculate current and historical progress for every participant
  in the path.
- The warning must distinguish between unchanged recorded activity and the goal
  progress views that will change.
- Applying the change must require an explicit confirmation after the warning is
  shown.
- Canceling or dismissing the confirmation must leave the goal configuration and
  all displayed progress unchanged.
- The confirmation must present the old and new goal configurations together for
  direct comparison.
- A confirmed goal-change request must identify both the exact old goal
  configuration that was reviewed and the proposed new goal configuration.
- The service must compare that reviewed old configuration with the current
  persisted configuration while holding the Path's mutation lock.
- If another confirmed change has replaced the reviewed configuration before
  this request acquires the lock, the later request must fail with a conflict,
  must not overwrite the intervening change, and must not record a successful
  mutation or reserve its idempotency key.
- Retrying the exact request after its first successful application must return
  the authoritative stored result rather than conflict merely because the
  current configuration is now the proposed configuration.
- The reviewed old configuration must be part of the request's canonical
  idempotency identity; reusing an idempotency key with a different reviewed old
  configuration must fail with an idempotency conflict.
- The comparison must use concise, plain language suitable for a nontechnical
  user.
- The comparison must emphasize the settings that are changing and must not add
  technical detail that does not help the user understand the effect.
- On mobile, the goal editor must present target durations in selectable human
  units while preserving the exact whole-second goal value submitted at the
  client boundary.
- On mobile, interval recurrence and calendar-alignment choices must use native
  selection semantics, and optional goal sections must expose their entire
  labeled row as an operable switch target.
- Editing a goal draft and reviewing a proposed change must be distinct
  presentation states. Entering review must not apply the draft, and canceling
  review must return to the editable draft without changing the stored goal.
- The mobile editor, review warning, direct old-and-new comparison, confirmation,
  cancellation, validation, retry, and success feedback must remain reachable
  with the software keyboard visible and at accessibility text sizes.
- When it can be shown accurately and concisely, the confirmation should include
  a plain-language example using the user's recorded activity to illustrate how
  displayed progress will change.
- Any example must make clear that the recorded activity itself will not change.

## Overall target behavior

- Progress toward an overall target must be calculated from the path's
  accumulated time for each participant.
- Overall-target progress must remain separate for each participant.
- Participants must be able to compare their individual progress toward the same
  overall target.
- An overall target must remain independent of interval-goal progress.
- Reaching or missing an interval goal must not change progress toward the
  overall target.
- Reaching the overall target must not stop the path from accumulating more time.
- Reaching the overall target must not cap, reset, or replace the participant's
  displayed progress.
- The achieved target must remain visibly completed while progress continues to
  increase beyond it, until an authorized user changes or removes the target.
- For example, after reaching a 1,000-hour target, another 25 tracked hours must
  produce progress equivalent to 1,025 of 1,000 hours rather than remaining at
  1,000 hours or beginning again at zero.

### Example

A guitar-practice path may have a weekly goal of 10 hours and an overall target
of 10,000 hours. Alice and Bob each see a weekly target of 10 hours and an
overall target of 10,000 hours. Alice's displays report Alice's time, Bob's
displays report Bob's time, and their progress can be compared.

## Reporting

- Interval-goal progress and overall-target progress must be reported separately.
- Each participant must see the path's goals and their own progress toward them.
- A path with no goals must report accumulated time without implying a missing or
  failed goal.

## Achievement event lifecycle

- For an interval goal, each participant must create at most one achievement
  event per interval occurrence when their progress first reaches or exceeds
  the goal threshold.
- A new interval occurrence must make the participant eligible for a new
  interval-goal achievement event.
- Continuing beyond the threshold within the same interval must not create
  duplicate achievement events.
- For an overall target, each participant must create one achievement event when
  their accumulated progress first reaches or exceeds that target.
- Continuing beyond an achieved overall target must not create another event for
  the same supported achievement.
- Creating a goal or lowering its threshold below progress already recorded must
  immediately display that goal as completed under the current configuration.
- A goal-configuration change by itself must not create an achievement feed
  event, even when the new threshold is already met.
- If the participant is below the configured threshold and later recorded
  activity first reaches it, that genuine activity-driven crossing must remain
  eligible for the ordinary achievement event.
- A goal-achievement feed event must remain derived from the path's current
  recorded activity rather than become an immutable historical claim.
- If editing or deleting recorded activity means an achievement is no longer
  supported, the achievement event must be removed.
- Removing an invalidated achievement event must also remove every reaction and
  comment attached to that event.
- Achievement invalidation and removal must not notify other users.
- If later recorded activity genuinely achieves the goal again, that later
  achievement must create a new event rather than restore the removed event or
  its previous engagement.
