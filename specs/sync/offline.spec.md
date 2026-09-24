# Offline Behavior

Status: Approved for implementation

## Purpose

Define the time-tracking behavior that must remain available without an internet
connection and the limits of other offline surfaces.

## Core offline capability

- Lack of network connectivity must not prevent a participant from tracking
  time.
- While offline, a participant must be able to start and stop a timer for a
  locally available path.
- Starting an offline timer must durably store its active state on the device,
  including its Path, participant account, original start instant, and effective
  IANA time zone.
- Offline active-timer state must not be treated as an evictable cache.
- An offline timer must continue logically while the application is terminated
  or the device restarts.
- Reopening the application must restore the timer as running from its original
  start instant and show the elapsed duration that accrued while the application
  was not running.
- Apart from where its active state is stored before synchronization, an offline
  timer must provide the same start, elapsed-time, stop, and recorded-activity
  behavior as an online timer.
- Stopping the restored timer while still offline must create the same pending
  recorded activity that stopping it without an application restart would have
  created.
- While offline, a participant must be able to create a manual time entry for a
  locally available path.
- While offline, a participant must be able to edit a locally available recorded
  activity entry.
- These capabilities must apply to both single-participant and shared paths that
  are locally available on the device.
- Offline tracking actions must be retained until they can be synchronized.
- Every active Path in which the current user is a participant must be retained
  locally with enough Path and membership data to start, stop, manually add,
  and display pending activity while offline.
- The initial offline guarantee does not require retaining supporter-only or
  archived Paths locally because they cannot receive new tracked activity.

## Locally available history

- While offline, a user must be able to view recorded activity history already
  available on the device.
- The device must retain the most recent 90 days of the user's own activity
  history for offline access; it must not require a complete local copy of all
  historical activity.
- The 90-day window must be applied consistently rather than varying silently
  according to incidental cache eviction.
- The application must not imply that local history is complete when older or
  unsynchronized history is unavailable.
- Offline operation must not require the device to retain the user's complete
  historical dataset.

## Offline banner

- Whenever the application is offline, it must display a visible but
  non-blocking offline banner.
- The banner must explain that tracking remains available, changes will
  synchronize automatically, and locally shown history is limited to the most
  recent 90 days.
- The banner must be dismissible and dismissal must not pause synchronization,
  discard pending activity, or prevent the user from seeing an entry's
  unsynchronized status.
- Dismissing the banner must hide it for the remainder of the current continuous
  offline period.
- Once connectivity returns, that dismissal must reset; if the application
  later goes offline again, the banner must appear again.
- The banner must not cover or disable Home timer controls or manual entry.

## Unavailable or incomplete offline behavior

- The Social surface is not required to function offline.
- Activity or progress from other participants is not required to be available
  or current offline.
- Aggregate Stats/History is not required to function offline.
- Path administration and other non-tracking workflows are not required to
  function offline.

## Synchronization

- Offline changes must synchronize automatically when connectivity becomes
  available.
- A still-running offline timer must synchronize its active state using its
  original start instant rather than restarting from the reconnection time.
- Automatic synchronization must not require the user to initiate a manual
  sync action.
- A user must not need to recreate an offline time entry merely because it was
  recorded without connectivity.
- Synchronization must preserve the participant and path attribution of each
  offline entry.
- Every locally created synchronization mutation must receive a stable unique
  operation identity before its first delivery attempt.
- Automatic and manual retries of the same mutation must reuse that operation
  identity.
- Applying the same operation more than once must have the same product effect
  as applying it once; a retry must not duplicate an activity entry, timer,
  edit, deletion, or social event.
- Reusing one operation identity for materially different mutation content must
  be rejected rather than treated as a retry.
- Mutations that depend on the same entity must synchronize in causal order,
  including creation before edits and an earlier edit before a later edit.
- Deletion of an entity must be terminal for pending mutations to that entity;
  a later-arriving edit or retry must not recreate it.
- Mutations for independent entities may synchronize concurrently when doing so
  cannot violate their individual ordering or authorization rules.

## Offline clock behavior

- Before server acknowledgement, an offline timer's UTC start and stop instants
  must necessarily come from the device clock.
- A client may use a monotonic clock transiently to keep its live elapsed-time
  display stable while it is running, but that value must not become a separate
  canonical activity duration.
- When an offline timer synchronizes with an end instant later than its start
  instant, the server must accept those UTC instants subject to ordinary
  validation and derive duration from their difference.
- The initial product must not attempt to infer or repair other device-clock
  changes automatically.
- If an offline timer's end instant is equal to or earlier than its start
  instant, synchronization must not create recorded activity.
- That timer must remain available locally in a correction-required state rather
  than being discarded or retried indefinitely.
- The participant must be able to correct the occurrence timing and explicitly
  save it as recorded activity; the corrected entry must then follow all
  ordinary validation and synchronization rules.
- The correction experience must explain plainly that the device time changed
  and the session's timing must be reviewed, without exposing implementation
  details.

## Conflicting active timers

- When synchronization reveals more than one active timer for the same
  participant and Path, the timer with the most recent UTC start instant must
  remain active.
- Every older conflicting timer must end without creating recorded activity;
  its elapsed time must be discarded.
- This most-recent-start rule must apply regardless of whether the winning or
  losing timer was already acknowledged by the server or was first created
  offline.
- If conflicting timers have identical start instants, synchronization must use
  a stable deterministic tie breaker so every device selects the same winner.
- The device must plainly tell the user that a timer conflict was resolved,
  identify the Path, and explain that the older timer's elapsed time was not
  saved.
- Resolving a timer conflict must update every online device and operating-system
  timer surface to show only the winning timer.
- A retry of the winning start or a discarded losing start must not reverse the
  settled result or create another timer or activity entry.

## Failed synchronization

- A failed synchronization attempt must not discard or roll back a locally
  saved activity entry.
- The unsynchronized entry must remain queued for automatic retry.
- Automatic retries must continue when conditions allow without requiring the
  user to recreate the entry or initiate every attempt.
- The application must make an entry's unsynchronized state visible to the user.
- The unsynchronized indicator must be clear but visually subordinate to normal
  tracking controls and content; it must not become a distracting interruption.
- The user must have access to a manual retry action for pending changes.
- Manual retry must supplement automatic retry rather than disable or replace
  it.
- A failed manual retry must leave the entry saved and queued for another retry.

## Transient and permanent failures

- Network unavailability, timeouts, temporary server or dependency failures,
  throttling, and authentication that can be refreshed must be treated as
  transient failures and remain eligible for retry.
- Confirmed provider revocation must pause synchronization until the user signs
  in again and must not, by itself, discard pending activity.
- A pending mutation must be permanently rejected when its account, Path,
  membership, required role, or target entry no longer exists or no longer
  authorizes the operation.
- A pending mutation must also be permanently rejected when its Path lifecycle
  forbids it, its content type has been disabled or blocked, its payload
  permanently violates current validation, or its operation identity was reused
  with materially different content.
- The more specific split and retention rules for archival, deletion, role
  change, or conflict must take precedence when they preserve part of an
  otherwise rejected operation.
- Permanently rejecting one mutation must remove it from automatic retry and
  must not discard an unrelated pending mutation.
- The application must retain one durable in-application explanation of a
  permanent rejection until the user dismisses it.
- The explanation must identify the affected action and Path when the user
  remains authorized to know that identity, while avoiding internal error or
  security details.

## Synchronizing after membership removal

- If the server confirms that a user is no longer a participant in a path, an
  offline activity change for that path must be treated as permanently rejected
  rather than transiently failed.
- Activity time rejected because membership ended must be removed from the local
  pending queue and must not be retried.
- The rejected time must not be synchronized, retained as path activity, or
  offered for transfer to another path.
- The application must clearly tell the user that the activity was not saved
  because they are no longer a participant in the path.
- This permanent rejection message must distinguish the result from an entry
  that is merely waiting for another automatic retry.
- Rejection of one path's pending activity must not discard pending activity for
  another path.

## Synchronizing after Path archival

- An offline timer session synchronized after its Path was archived must be
  evaluated against the server-recorded instant at which archival took effect.
- A pending session that ended at or before the archival instant must be saved
  normally when its other authorization and validation requirements remain
  satisfied.
- A pending session that started before and ended after the archival instant
  must be split at that instant: the elapsed portion before archival must be
  saved as recorded activity and the portion after archival must be discarded.
- A pending session that started at or after the archival instant must be
  permanently rejected and removed from the retry queue.
- The application must clearly tell the participant whenever any elapsed time
  is discarded because the Path became archived, including how much time was
  saved and how much was discarded when a session was split.
- The discarded portion must not contribute to activity history, goals,
  statistics, achievements, or feed events.
- Processing one archived Path's pending session must not alter pending activity
  for another Path.

## Concurrent activity edits

- If the same recorded activity entry is edited on multiple devices before all
  edits synchronize, the most recent edit must become the entry's current
  version.
- Conflict resolution must occur automatically and must not interrupt the user
  with a merge or conflict-resolution prompt.
- Every synchronized version, including a version superseded by the winning
  edit, must remain available in the activity entry's edit history.
- Resolving one activity conflict must not alter another recorded activity
  entry.
- The synchronization design must define a deterministic ordering for deciding
  which edit is most recent, including a stable tie breaker.

## Edit conflicting with deletion

- If one device deletes a recorded activity entry while another device has a
  pending offline edit to that entry, deletion must win regardless of the
  relative edit time.
- The pending edit must not recreate, restore, or create a replacement for the
  deleted activity.
- Once the device learns of the deletion, it must remove the rejected edit from
  the pending retry queue and remove the deleted entry from local history.
- The application must tell the user plainly that their edit was not saved
  because the activity was deleted elsewhere.
- Rejecting that edit must not affect pending changes for another activity
  entry.
