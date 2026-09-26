# Recorded Activity

Status: Approved for implementation

## Purpose

Define recorded activity as the goal-independent source of truth for time spent
on a path.

## Terminology

- **Recorded activity:** A record that a participant spent a duration of time on
  a path, with enough occurrence timing to place that time into calendar
  intervals.

## Source of truth

- Time spent on a path must be recorded independently of interval goals and
  overall targets.
- Recorded activity must be attributed to both the path and the participant who
  performed it.
- Recorded activity must retain when the activity occurred and how long it
  lasted.
- Creating, changing, or removing a goal must not create, move, divide, merge,
  duplicate, or delete recorded activity.
- Accumulated time and goal progress must be derived from recorded activity.

## Canonical activity information

- Every recorded activity entry must have a stable identity.
- Every entry must identify its Path and owning participant.
- Every entry must retain its UTC start and end instants and occurrence IANA
  time zone.
- Every entry must retain its creation time and most recent modification time.
- When an entry has been edited, it must retain the revision history required
  to reproduce each synchronized prior version.
- An entry may retain the optional private note and its private revision history
  defined below.
- Information needed only to synchronize a mutation idempotently must not become
  user-facing activity metadata.

## Canonical time representation

- A recorded activity's start and end must be stored as UTC instants.
- Elapsed duration must be derived from the UTC end instant minus the UTC start
  instant with precision to one second. It must not be persisted as an
  independently editable canonical value.
- A duration edit must update an entry's end instant rather than storing a
  separate duration that could disagree with its start and end.
- Recorded activity must retain the IANA time zone that applied to the
  participant when the activity occurred.
- Changing the participant's configured time zone later must not rewrite the
  activity's UTC instants or original historical calendar attribution.
- A session that crosses a local calendar boundary may contribute elapsed
  portions to multiple calendar periods without splitting, duplicating, or
  mutating the underlying activity entry.
- A completed recorded activity duration must be a positive whole number of
  seconds; an entry shorter than one second or with zero or negative duration
  must not be created or saved by an edit.
- A completed entry's end instant must not be later than the current instant
  when the entry is created or edited.
- The initial product must not impose an arbitrary maximum activity duration
  when the start and end are otherwise valid.

## Duration entry presentation

- Mobile creation and editing must present duration as hours and minutes rather
  than require conversion to a total number of seconds. Nonzero seconds must
  remain visible and editable; otherwise second precision may be disclosed on
  demand. This same duration control should be used for goal entry.
- Editing an existing duration must preserve its exact value until the user
  changes it. For example, 14,021 seconds must open as 3 hours, 53 minutes, and
  41 seconds; changing the minutes to 54 must produce 14,081 seconds.
- Blank fields may contribute zero when another field has a value. An entirely
  blank duration must remain blank. Invalid or unsafe numeric input must remain
  invalid, never silently become a shorter saved duration.

## Recording methods

- A participant must be able to record activity by starting and stopping a
  timer.
- A participant must be able to enter activity manually.
- Timer-created and manually created activity must contribute to accumulated
  time and goal progress using the same calculation rules.
- A recorded activity's original recording method must not be displayed as a
  persistent user-facing classification after creation.
- A recorded activity entry may include an optional short note.
- An activity note must contain at most 2,000 characters after Unicode NFC
  normalization.
- An empty or whitespace-only note must be treated as absent.
- Notes may contain line breaks, ordinary Unicode text, and emoji but must not
  contain control characters other than ordinary line breaks.
- An over-length note must be rejected rather than silently truncated.
- The initial product must not add a separate title, tag, or category to a
  recorded activity entry.
- A note must never be required to start, stop, manually add, or save activity.
- Starting or stopping a timer must not open a note prompt or add another
  blocking step to the tracking flow.
- A participant must be able to add or edit their note afterward from the
  activity-entry detail experience.
- Manual entry may expose the note as a secondary optional field, but that field
  must not compete visually with date, time, duration, or save controls.
- An activity note must be private to the participant who owns the recorded
  activity.
- Creators, administrators, other participants, and supporters must not be able
  to view another participant's activity note.
- Activity notes must not appear in feed events, shared progress comparisons,
  notifications, or statistics.
- A note's current value and revision history must be excluded when another
  authorized user inspects the activity entry or its edit history.
- A note-only edit must not update the public feed event, add an `Edited` label
  to that event, or notify another user.

## Manual entry occurrence

- Manual entry must collect a calendar date, start time, and duration.
- The date and start time must be interpreted in the participant's configured
  time zone and converted to the canonical recorded-activity representation.
- When a manually entered local start time occurs twice during a backward clock
  transition, it must resolve to the earlier occurrence.
- When a manually entered local start time does not exist during a forward clock
  transition, it must move forward by the transition gap.
- The form must default to the participant's current local date.
- Until the user explicitly changes the date or start time, entering a duration
  must position the activity so that it ends at the current time.
- The user must be able to override the default date and start time before
  saving.
- Saving must create one recorded activity entry with an end instant derived
  from its start and duration.
- On mobile, manual entry and owner-authorized editing must use a compact native
  form sheet with persistent Cancel and Save-or-Retry actions. Date, start time,
  duration, and optional note must remain reachable with the software keyboard
  and maximum Dynamic Type.
- Validation or service failure must identify the problem in text, preserve the
  complete activity draft, and keep retry available without duplicating a
  successful save. Submission progress and outcome must use native status
  semantics and remain understandable with VoiceOver and reduced motion.

## Overlapping activity

- Recorded activity entries may overlap in elapsed time, whether they belong to
  the same Path or different Paths.
- Each overlapping entry must contribute its full duration independently to its
  own Path, progress, history, and statistics.
- The application must not silently merge, shorten, or discard an entry merely
  because it overlaps another entry.
- A participant may create a manual entry that overlaps a running timer,
  including a timer on the same Path.
- Permitting overlapping completed activity must not weaken the rule that a
  participant may have at most one active timer on any one Path.

## Running timer lifecycle

- A running timer must be durable account state rather than only an in-memory
  client countdown.
- When connectivity is available, starting a timer must synchronize an active
  timer record containing its participant, Path, UTC start instant, and the
  participant's effective IANA time zone.
- An active timer must not have an end instant. Stopping it must assign its UTC
  end instant and thereby determine the completed activity's duration.
- A timer must retain that starting IANA time zone for its entire run.
- Intentionally changing the participant's configured time zone while the timer
  is running must not change the timer's occurrence time zone or divide the
  eventual recorded activity between time zones.
- Stopping the timer must create recorded activity using the IANA time zone
  captured when the timer started.
- A timer started after the configured time-zone change must use the new time
  zone.
- An active timer must not contribute a completed entry to history, statistics,
  goals, or the social feed until it is stopped and recorded as activity.
- A timer started offline must remain a clearly pending local timer until the
  server acknowledges it; the lack of acknowledgment must not prevent it from
  measuring elapsed time locally.
- The pending local active-timer record must be durable across application
  termination and device restart rather than existing only in memory or an
  evictable cache.
- Once an active timer is acknowledged by the server, clearing a device's local
  cache or signing out must not stop or delete that server-backed timer.
- A server-acknowledged timer must appear with its original start time and live
  elapsed duration on every online device signed into the same account.
- The participant must be able to stop that timer from any such device.
- A successful stop on one device must complete the timer only once, create one
  recorded activity entry, and automatically remove the running state and its
  operating-system surfaces from the account's other online devices.
- If a timer is stopped before one whole second has elapsed, it must end without
  creating recorded activity, progress, statistics, achievements, or a feed
  event.
- The application must explain that no activity was saved because the timer ran
  for less than one second, using a quiet non-blocking presentation.
- Another device learning that the timer was stopped elsewhere must not create a
  duplicate activity entry or continue presenting the timer as running.
- A running timer must continue until the participant explicitly stops it or a
  separately specified lifecycle rule ends it.
- Closing or terminating the application must not stop a running timer.
- Restarting the device must not stop a running timer.
- After the application or device restarts, the timer must be restored using
  its original start time so that elapsed time is not lost or restarted.
- Every restored timer must remain independently stoppable when multiple timers
  are running.
- Removing a participant from a path must stop their timers for that path and
  delete the elapsed time rather than save it; timers on other paths must remain
  unaffected.
- Deleting a path must stop every timer on that path and discard its elapsed
  time; timers on other paths must remain unaffected.
- Permanently deleting an account must stop all of that user's timers and
  discard their elapsed time without saving final sessions.
- Attempting to sign out with running timers must use the explicit resolution
  prompt defined in
  [Account Authentication](../accounts/authentication.spec.md#signing-out-with-running-timers).

## Unusually long timer notification

- Once a participant has at least three eligible completed sessions on a path,
  the application must calculate that participant's average session duration
  for that path.
- Every current positive-duration completed session owned by that participant
  on the Path must be eligible, whether it was originally recorded by a timer or
  entered manually.
- The average must be the arithmetic mean of the eligible sessions' current
  durations.
- Editing an eligible session's duration must recalculate the average using the
  edited duration, and deleting a session must remove it from the calculation.
- The initial calculation must not trim, cap, or otherwise exclude an eligible
  session merely because its duration is an extreme outlier.
- When a running timer reaches 1.5 times that average duration, the application
  must notify the participant that the timer is still running.
- Each running timer must produce at most one unusually-long-timer notification,
  even if it continues running far beyond the threshold.
- Another participant's sessions must not affect a user's long-timer threshold.
- A path with fewer than three eligible completed sessions must not produce an
  average-based long-timer notification.

## Simultaneous timers

- A participant must be able to run timers for multiple paths simultaneously.
- A participant must have at most one active timer on any one Path.
- Attempting to start another timer on a Path where that participant already has
  one running must not create a second timer or reset the existing timer's start
  time.
- The application must instead surface the existing running timer and its stop
  control.
- Starting a timer for one path must not stop, pause, or alter a timer running
  for another path.
- Each running timer must continue to attribute elapsed time to its own path and
  participant.
- A participant must be able to stop each simultaneous timer independently.
- Home representation and ordering for simultaneous timers are defined in
  [Home](../experience/home.spec.md#active-timer-position).
- Operating-system representation is defined in
  [Client Platforms](../platform/clients.spec.md#active-timer-visibility).

## Editing recorded activity

- Every recorded activity entry must be editable after it is created, including
  historical entries.
- Only the participant to whom an activity entry is attributed may edit that
  entry.
- Path creator or administrator status must not grant authority to edit another
  participant's recorded activity.
- A denied edit attempt must leave the activity and all derived data unchanged.
- An entry created by a timer must remain editable.
- An entry created manually must remain editable.
- Editing must allow the activity's occurrence timing and duration to be
  corrected.
- After an edit, accumulated time and current and historical goal progress must
  be recalculated from the updated recorded activity.
- Editing one entry must not alter another entry.
- Each edit must be retained in an edit history for that recorded activity
  entry.
- Authorized users viewing the entry must be able to inspect its edit history.
- An edit must update any feed event derived from the entry in place and must
  not create another feed event.
- Editing recorded activity must not notify other users.

## Deleting recorded activity

- A recorded activity entry must be deletable after it is created, whether it
  originated from a timer or manual entry.
- Only the participant to whom an activity entry is attributed may directly
  delete that entry.
- Path creator or administrator status must not grant authority to directly
  delete another participant's individual activity entry.
- This ownership rule does not prevent separately specified destructive
  lifecycle operations, such as participant removal or Path deletion, from
  deleting activity as part of the larger confirmed operation.
- A denied deletion attempt must leave the activity and all derived data
  unchanged.
- Deleting an entry must remove its contribution from accumulated time, goal
  progress, history, and statistics.
- Deleting an entry must also delete its derived feed event and all reactions
  and comments attached to that event.
- A successful deletion must remove the entry and every already-loaded derived
  representation from the current client and apply the server's authoritative
  accumulated-time, completed-session-count, goal-progress, and unread-
  notification-count result without requiring an application restart or full
  sign-in cycle.
- The successful deletion result must identify every exact feed event removed
  with the entry, including unsupported goal achievements, so clients can
  remove only those events and their related notifications while preserving
  unrelated loaded data.
- While deletion is pending, the client must prevent navigation from presenting
  the entry as safely retained when the server may still commit the request.
- A failed deletion must leave the visible entry and derived client state
  unchanged, must explain plainly that the activity was not deleted, and may
  offer retry only when the failure is retryable.
- Successful deletion must provide an accessible status announcement.
- Deleting an entry and its derived social data must not notify other users.
- Deleting one entry must not alter another recorded activity entry.

### Example

If Alice practices from 5:00 PM until 6:00 PM on Monday, the product records one
hour of Alice's activity on the path at that time. Whether that hour satisfies a
daily, weekly, monthly, yearly, or no interval goal is calculated separately.
