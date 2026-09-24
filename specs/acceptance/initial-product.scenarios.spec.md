# Initial Product Acceptance Scenarios

Status: Derived from approved specifications

## Purpose

Translate the approved normative behavior into end-to-end scenario families for
implementation and test planning. These scenarios supplement rather than
replace the authoritative domain specifications; every normative `must` and
`must not` remains independently testable.

## Accounts and identity

### ACCT-01: Create an account with either provider

Given a person has no application account, when they authenticate successfully
with Google or Apple, attest that they are at least 16, accept the required
policies, and confirm a valid profile, then one internal application user is
created with that provider identity and Home opens empty.

### ACCT-02: Link the other provider

Given a signed-in user has one provider identity, when they authenticate the
other provider through account settings, then both immutable provider identities
sign in to the same internal user without moving or duplicating product data.

### ACCT-03: Reject an identity already linked elsewhere

Given the external identity selected for linking belongs to another application
user, when linking is attempted, then neither account changes and no merge or
email-based association occurs.

### ACCT-03A: Prevent an accidental duplicate account

Given a new provider reports a verified email corresponding to an existing
application account, when account creation begins, then the person is offered a
fully authenticated route to the existing account and may link the provider
only after authenticating both identities; no link occurs from email alone.

### ACCT-03B: Recover from two completed accounts

Given the person already completed separate accounts, when they try to link an
identity owned by the other account, then no merge occurs and the product
explains that they must retain one account, delete the other under the ordinary
destructive rules, and link the released identity after deletion completes.

### ACCT-04: Preserve offline access

Given a previously authenticated local session and unavailable network or
provider, when the user opens the application, then locally available tracking
continues while server-dependent features remain unavailable.

### ACCT-05: Replace a retained account safely

Given another account has unsynchronized local changes, when a different
unlinked identity signs in, then synchronization is attempted and any remaining
data loss requires explicit confirmation before replacement.

### ACCT-06: Resolve sign-out with active timers

Given one or more running timers, when sign-out is requested, then the user must
choose stop-and-save, keep-running, or cancel, and each choice produces exactly
the behavior described by the authentication specification.

## Paths, membership, and authorization

### PATH-01: Create a name-only Path

Given a valid signed-in user, when they create a Path with only a valid name,
then they become its creator and first participant and can immediately track
time without a goal.

### PATH-02: Create optional goals

Given Path creation, when the user supplies either, both, or neither optional
goal, then the saved Path reflects exactly that choice and no missing goal is
presented as failure.

### PATH-03: Accept an invitation

Given a pending participant or supporter invitation, when the recipient accepts
it, then only the offered role is granted and the inviter receives the specified
informational notification.

### PATH-04: Warn a private-profile participant

Given a private-profile user is invited to participate in a more broadly visible
Path, when they attempt to accept, then the Path-governed visibility warning is
shown before acceptance and canceling leaves the invitation pending.

### PATH-05: Enforce management boundaries

Given each Path role in turn, when every management action in the permission
matrix is attempted, then authorized actions succeed, denied actions fail
closed, and no denied attempt changes Path or user data.

PATH-05 is aggregate acceptance across independently demonstrable management
slices rather than one oversized implementation change. Each slice must ship
one coherent user workflow through the applicable API, authorization, web, and
mobile boundaries while retaining unchanged-on-denial coverage. The dedicated
PATH-06 through PATH-10 scenarios contribute their corresponding ownership,
lifecycle, and membership cells; PATH-05 closes only when the complete matrix
has evidence. The first slice, PATH-05A, renames an active Path: creators and
administrators may save a valid name, participants and supporters fail closed,
and canceling or a denied attempt leaves the Path unchanged.

PATH-05B lists and cancels ordinary pending invitations: a creator or current
administrator can see every pending invitation for an active Path and cancel
one regardless of which manager sent it; participants and supporters fail
closed; the recipient can no longer accept the canceled invitation; its
actionable notification and pending push work disappear; and unrelated
membership, invitations, relationships, and notifications remain unchanged.

PATH-05C changes an active Path's visibility: only the creator can apply a
reviewed private, followers, or public transition permitted by the creator's
profile privacy; administrators and other roles fail closed. Broadening exposes
eligible history immediately and informs affected private-profile participants,
while narrowing removes general-audience access immediately. Membership,
recorded activity, progress, profiles, unrelated Paths, and denied or canceled
attempts remain unchanged.

### PATH-06: Transfer ownership

Given the creator selects an eligible participant, when the recipient accepts a
nonexpired transfer, then the recipient becomes creator, the former creator
becomes administrator, and recorded activity and membership remain unchanged.

### PATH-07: Archive and unarchive

Given an active Path with running timers, when the creator archives it, then
server-known timers stop and save, the Path becomes view-only and leaves active
Home; when unarchived, its prior configuration returns without restarting a
timer.

### PATH-08: Delete a shared Path

Given a shared Path with participant activity, when the creator confirms the
complete destructive warning, then the Path and every participant's associated
data are removed, running elapsed time is discarded, and affected users receive
standalone deletion notices.

### PATH-09: Leave while retaining activity

Given a noncreator participant chooses the default ordinary leave while a timer
is running, when leaving completes, then the timer stops and saves, access ends,
and all of that participant's retained activity is hidden from everyone until
the user rejoins.

Given a noncreator participant explicitly confirms leave and delete while a
timer is running, when leaving completes, then the timer stops without saving,
access ends, and all of that participant's Path activity and derivatives are
permanently deleted and do not return after rejoining. Canceling either choice
changes nothing, and a supporter may leave without being shown an inapplicable
activity choice.

### PATH-10: Remove a participant

Given an authorized manager confirms participant removal, when removal
completes, then membership, activity, progress, social derivatives, pending
offline work, and Path timers for that participant are removed as warned.

## Tracking and goals

### TIME-01: Start and stop a timer

Given a trackable active Path with no running timer for the participant, when the
timer starts and later stops after at least one second, then exactly one activity
entry is created with UTC start and end instants and an occurrence time zone,
its duration is derived rather than stored independently, and all derived
progress is updated.

### TIME-02: Reject a subsecond session

Given a timer has run for less than one second, when it is stopped, then it ends
without recorded activity or social derivatives and a quiet explanation is
shown.

### TIME-03: Enforce one active timer per Path

Given the participant already has an active timer on a Path, when another start
is attempted on that Path, then no second timer is created and the existing
timer is surfaced.

### TIME-04: Permit simultaneous and overlapping activity

Given timers on different Paths or manually overlapping entries, when they are
saved, then each valid entry contributes its full elapsed time independently and
is not silently merged or shortened.

### TIME-05: Create and edit manual activity

Given a valid local date, time, and duration ending no later than now, when a
manual entry is saved and later edited by its owner, then history and all
derived calculations use the current entry while every synchronized revision
remains inspectable.

### TIME-06: Delete owned activity

Given a completed activity with feed engagement, when its owner deletes it,
then its time, progress, statistics, event, comments, reactions, and related
notifications are removed without notifying other users.

### TIME-07: Split an activity at interval boundaries

Given activity crosses one or more goal boundaries, when progress is calculated,
then actual elapsed portions contribute once to their respective intervals
while accumulated time still counts the full activity once.

### TIME-08: Recalculate after a goal change

Given historical activity and an authorized confirmed goal change, when the new
configuration takes effect, then current and historical progress are projected
from unchanged activity under only the new configuration.

Before that mutation, Manage Path presents the old and proposed configurations
together and warns that progress views will change while recorded activity will
not. Canceling leaves both configuration and displayed progress unchanged;
confirming immediately applies the new current projection on Home and Path
detail.

### TIME-09: Resolve daylight-saving boundaries

Given an ambiguous or nonexistent local boundary, when it is converted to an
instant, then backward transitions use the earlier occurrence, forward gaps
move forward by the gap, and elapsed duration continues to use UTC instants.

### TIME-10: Retain a running timer's starting time zone

Given a timer is running when the participant changes configured time zone,
when the timer later stops, then its entire activity retains the time zone
captured at start while a subsequently started timer uses the new time zone.

## Offline synchronization

### SYNC-01: Track through termination and restart

Given an offline running timer, when the application terminates or the device
restarts, then reopening restores the same timer from its original start and
stopping it creates durable pending activity.

### SYNC-02: Retry idempotently

Given a mutation response is lost, when the same operation identity is retried,
then its effect occurs once and no activity, timer, edit, deletion, or event is
duplicated.

### SYNC-03: Resolve conflicting active timers

Given two active timers for the same participant and Path synchronize, when the
conflict is resolved, then the later UTC start remains active, the older elapsed
time is discarded, and every online surface converges with an explanation.

### SYNC-04: Reject work after membership removal

Given pending Path activity and server-confirmed membership removal, when sync
occurs, then that activity is permanently rejected, removed from retry, and
explained without affecting another Path's work.

### SYNC-05: Split work at archival

Given an offline activity spans the server archival instant, when it
synchronizes, then the pre-archive portion saves, the later portion is
discarded, and the user sees both amounts.

### SYNC-06: Resolve concurrent edits and deletion

Given concurrent edits, when they synchronize, then the deterministic most
recent edit becomes current and all versions remain in history; given deletion
versus edit, deletion wins and the edit cannot recreate the entry.

### SYNC-07: Distinguish transient and permanent failure

Given each failure class in the offline specification, when synchronization
fails, then transient work remains retryable while permanent work leaves the
queue with one durable explanation.

### SYNC-08: Handle an offline device-clock reversal

Given an offline timer whose device-provided end instant is later than its start,
when it synchronizes, then those instants are retained and duration is derived
from them; given an end instant equal to or earlier than start, then no activity
is saved and the local session remains available for explicit user correction.

## Visibility and social behavior

### SOC-01: Apply profile and Path visibility

Given each profile and Path visibility combination, when another user discovers
or opens the Path, then the Path rule governs Path activity while unrelated
profile and Path data remain protected.

### SOC-01A: Find people safely

Given an authenticated user on Following, when they enter an eligible profile
query and open a result, then ranked active profiles appear in compact native
rows and the dedicated profile shows only the approved public identity
projection while blocked and private data remain undisclosed.

### SOC-01B: Follow people and resolve requests

Given public, private, blocked, deleted, and self profile targets, when a user
follows, cancels, approves, rejects, or unfollows through Following, then the
authoritative one-way relationship and counts converge exactly once, only the
specified notifications are created, pending access remains closed, and
follow-derived Path access changes without disturbing independent Path roles.

### SOC-02: Expand and reduce visibility

Given existing eligible Path events, when visibility expands, then history
becomes available and affected private-profile participants are notified; when
visibility contracts, the removed audience loses access immediately.

### SOC-03: Build a chronological feed

Given events from followed users and shared-Path participants, when the feed is
loaded, then each eligible event appears once in publication-time order and no
arbitrary-user event appears.

### SOC-03A: View chronological practice activity

Given saved timer sessions and manual entries from followed users or fellow
tracking participants on the same Path, when Following is loaded, then each
currently authorized practice event appears exactly once in server-publication
order, private notes and unrelated activity remain absent, and refresh, edits,
deletion, and cursor pagination preserve the same stable event identity and
ordering.

### SOC-03B: Celebrate goal achievements in the feed

Given a followed or shared-Path participant first crosses a currently
configured interval goal or overall target through saved activity, when
Following is loaded, then the achievement appears exactly once in the shared
chronological timeline with its safe participant, Path, target, and achievement
kind, while replays, configuration-only completion, inaccessible state, and
activity-invalidated achievements remain absent.

### SOC-03C: See followed people who are actively tracking

Given established followed users with one or more currently visible running
timers, when Following is loaded, then each followed user appears once above
completed events with every authorized active Path and live elapsed duration,
while stopped, blocked, inaccessible, pending, and unrelated timers remain
absent.

### SOC-04: Interact positively

Given an accessible eligible event whose owner permits interaction, when a user
reacts, comments, or hearts a comment, then only the approved positive controls
are accepted and the corresponding delayed notification behavior applies.

### SOC-05: Disable and restore interactions

Given an event owner disables comments or reactions, when the setting changes,
then existing interactions hide without deletion, related notifications are
removed, new interactions are denied, and re-enabling restores only undeleted
interactions.

### SOC-06: Block another user

Given two users block one another with and without a remaining shared Path, when
their surfaces are loaded, then ordinary identity and interactions are hidden
while only the minimum shared-Path collaboration and management behavior
remains.

### SOC-07: Send a nudge

Given an authorized sender, eligible recipient, selected preset, and available
rate-limit slot, when a nudge is sent, then it targets that recipient and Path
once and creates the permitted notification; an ineligible attempt changes
nothing.

## Notifications

### NOTE-01: Deliver by channel and context

Given each exact event in the notification matrix, when it occurs, then only the
specified recipients receive one notification through an enabled channel and
specific preferences may narrow but never broaden a disabled channel.

### NOTE-02: Respect unavailable periods and permissions

Given an active unavailable period or denied OS permission, when an event
occurs, then push is suppressed as specified while eligible in-application
state remains and product workflows continue.

### NOTE-03: Remove stale notification targets

Given referenced content is deleted or access is revoked, when notification
state updates, then target-opening in-app notices disappear, explanatory access
notices remain, and an old push opens only the opaque unavailable result.

### NOTE-03A: Remove a stale Path nudge after manager removal

Given a participant has a Path nudge and an unrelated notification, when a
creator or administrator removes the participant from that Path, then the nudge
and any not-yet-handed-off push delivery disappear, unread state converges, the
informational removal notice remains, the unrelated notification is preserved,
and the old exact nudge notification opens only the opaque unavailable result.

### NOTE-03B: Remove a stale Path nudge after voluntary leave

Given a participant has a private-Path nudge and an unrelated notification,
when that participant voluntarily leaves the Path, then the nudge and any
not-yet-handed-off push delivery disappear, the participant's unread state
converges, the unrelated notification remains unchanged, the creator retains
the informational member-left notice, and the old exact nudge notification
opens only the opaque unavailable result.

### NOTE-03C: Remove a hidden retained-activity interaction notice

Given a participant has a private-Path activity interaction notification and
an unrelated notification, when the participant voluntarily leaves while
retaining the activity, then the interaction notification and its
not-yet-handed-off push disappear, unread state converges, the unrelated and
creator member-left notices remain, and the old exact notification opens only
the opaque unavailable result. Rejoining may reveal the retained activity again
but must not resurrect the removed notification or push work.

### NOTE-03D: Remove an inaccessible interaction notice after visibility contraction

Given a nonmember has a target-opening interaction notification for a public or
followers Path, plus an unrelated notification and an accessible informational
visibility-change notice, when the creator narrows the Path so that nonmember
loses access, then the interaction notification and its not-yet-handed-off push
disappear, delivery-token material is scrubbed, unread state converges, and the
unrelated and visibility-change notices remain unchanged. The old exact
notification opens only the opaque unavailable result, and later re-expansion
must not resurrect the removed notification or push work.

### NOTE-04: Keep read state synchronized

Given multiple signed-in devices, when a notification is opened, deleted, or
marked read, then read state, ordering, and supported application badges
converge without mutating the underlying product event.

### NOTE-05: Avoid redundant foreground interruption

Given the user is already viewing the relevant content, when its notification
event arrives, then content and unread state update quietly without a redundant
intrusive banner.

## Deletion and moderation

### SAFE-01: Delete an account

Given a user confirms the complete deletion warning, when deletion begins, then
the account is disabled immediately, active product data disappears within 24
hours, backups remain beyond use for no more than 30 days, and only an allowed
minimal active-case exception can survive temporarily.

### SAFE-02: Report and review content

Given accessible reportable content, when a user submits a reason and optional
valid explanation, then one restricted moderation case is created, the reporter
receives only acknowledgment, and authorized review can produce an auditable
proportionate outcome.

### SAFE-03: Appeal enforcement

Given an enforcement notice less than 30 days old, when the affected user
submits their one appeal, then the original evidence and decision remain,
another reviewer is used when practicable, and the final reason is communicated
without identifying the reporter.

### SAFE-04: Reject clearly disallowed public text

Given a new or edited public comment or profile description fails the automated
safety check, when submission is attempted, then nothing is published and the
user receives a neutral editable error; passing content remains reportable.

## Experience and accessibility

### EXP-01: Preserve primary navigation state

Given independent state in Home, Following, and Stats, when the user switches
among them, then each surface preserves its navigation, filters, and scroll
position for the application session.

### EXP-02: Keep Home tracking-first

Given active, pinned, shared, solo, and supporter Paths, when Home is displayed,
then filters and ordering follow personal settings, running timers temporarily
lead, and no social feed content appears.

### EXP-02A: Put running Paths at the top of Home

Given one or more trackable Paths with authoritative running timers, when Home
is displayed or a timer successfully starts or stops, then every running Path
appears exactly once in a compact Active section ordered by newest start time,
stopped Paths preserve their ordinary relative order, and invalid, failed,
stale, supporter-only, and social state cannot enter that section.

### EXP-03: Present accessible progress

Given progress, privacy, synchronization, warning, chart, and activity-grid
states, when assistive technology or a non-color presentation is used, then
every state has a meaningful text or semantic equivalent and remains operable.

### EXP-04: Page an unbounded collection

Given a collection exceeding one page, when additional pages load or retry,
then stable cursor ordering prevents duplicates, prior items remain visible on
failure, and the end state is distinguishable from an error.
