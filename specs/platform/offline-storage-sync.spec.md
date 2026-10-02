# Durable tracking architecture

Status: Implementation design for approved offline behavior

## Authority and scope

[Offline Behavior](../sync/offline.spec.md) owns product rules; [authentication](../accounts/authentication.spec.md)
owns account access, revocation and sign-out. This document defines technical
boundaries for implementing those rules without changing their scope.
The first delivery covers offline timers, retained participating Paths/history,
restart, synchronization and timer failure/conflict handling in mobile and
Studio. Manual creation and recorded-entry editing use the same durable queue
in the next delivery. Neither delivery alone closes checklist area 2.

## Local ownership and atomic persistence

A shared client-core application service must own tracking commands and queue
replay behind injected clock, identity, durable-store and synchronization ports.
It must not import React, Svelte, SQLite, IndexedDB, HTTP or generated DTOs.
Mobile uses a transactional SQLite adapter; web uses IndexedDB transactions.
Credential storage remains separate from tracking storage.

Every record must be partitioned by the immutable application account ID.
Adapters must require this owner for reads and writes. Account replacement or
sign-out must cancel the current orchestrator generation before presentation
can adopt a replacement account. An acknowledgement arriving for an old
account may settle its durable operation but must never publish into the new
account's UI. No signed-out surface may expose retained Path identities.

A start command must atomically persist the timer, original UTC start, effective
IANA zone and stable operation ID before presenting it as saved locally.
A stop command must atomically persist its original end instant and dependent
operation before removing the running presentation. Failure to persist must
leave the previous durable state intact and show a localized retryable error.
Pending work, unresolved corrections and rejection notices must not be evicted
with history. A schema migration must retain them transactionally.

The retained Path metadata must include its configured goal alignment and the
participant's configured IANA zone. When an authoritative period expires,
clients must resolve the next calendar period locally using the same clamping,
daylight-saving gap and repeated-hour rules as the API. Saved progress from the
previous period must not carry forward. Retained own activity and pending
sessions contribute only their overlap with the newly resolved period; the
running timer remains one occurrence with one lifetime contribution. Showing
new period bounds must not wait for an unavailable server response.

Retained data includes all active participating Paths, permissions needed to
render tracking controls, own running timers, and the most recent 90 days of
own recorded activity. Refresh must merge acknowledged server data with pending
local changes rather than replace pending changes. Studio must hydrate timer
summaries and personal appearances for every participating Path before
publishing a complete Home cache, independently of filtering or rendered rows.
A failed or superseded batch must preserve the previous complete Home snapshot. Incomplete hydration must
be explicit; pagination must not silently truncate retained history.
A history refresh must finish every authorized source page before committing its
90-day snapshot. It must retain original interval ends and Path display names,
and fence account changes and concurrent local commands. A failed refresh must
leave the prior snapshot intact. Offline timeline pages must identify retained
history and incomplete initial hydration; pending sessions must be shown without
linking to a server entry that does not exist yet. Native Path history must use
the same account ledger as Home timers. Its background refresh must enumerate
active and archived Paths and every own-activity page before committing. Cached
rows must be read-only until server details are available, and pending rows
must be visibly distinguished from acknowledged activity.

## Server synchronization boundary

A dedicated authenticated activity synchronization command must accept original
device instants and occurrence zone, an immutable client timer ID and a stable
operation ID. Generated REST DTOs must map into application inputs. The caller
identity comes only from the application session. Existing online timer routes
retain their contract; replay must not substitute server time for device time.

During a rolling deployment, an old API instance may not recognize the replay
route. A route-level permanent rejection must identify the synchronization
operation in its machine-readable error. A generic proxy or unknown-route 404
must remain transient; it must not discard queued activity.

The server must decide current authorization through the existing permission
port and persist changes, idempotency result and audit atomically. An operation
ID is bound to its canonical content hash. Duplicate delivery returns the
settled outcome; different content with the same ID is rejected. Per-account,
per-Path serialization must cover online and offline timer mutations.

Timer conflicts order by original UTC start instant, then lexicographically by
immutable timer ID for equal instants. A discarded timer must retain a terminal
sync outcome so later retries cannot resurrect it. A stop must refer to its
own timer identity and must never stop a different winning timer.
Online starts, stops and lifecycle stops must use the same durable timer
identity registry as offline commands. A device may replay an adopted timer
using its server-visible ID; that alias must remain scoped to the participant,
Path, original start and occurrence zone. Adopting a timer must not allocate a
second active timer or allow a changed occurrence to reuse its identity.
Online stops and lifecycle stops must settle the same offline timer identity.
Replay must also recognize pre-upgrade online timer identities from their
persisted mutation records. A completed, subsecond, or deleted pre-upgrade
session must not be recreated by a delayed start or stop.
If archival has already recorded an acknowledged timer, a subsequently delivered
offline stop from before archival must correct that same recorded entry and
preserve its revision history, rather than add a second entry. Removing an
entry must leave its terminal timer identity incapable of recreating it.
Archive handling must use the persisted archive instant and preserve only the
allowed pre-archive interval. Restoring a Path must not reset a terminal timer
identity or replace an already recorded archive cutoff for that session. Membership removal and deleted/inaccessible Paths
must use the approved permanent-rejection behavior without leaking inaccessible
identities. Confirmed membership loss or deletion must durably disable new
local starts and settle pending timers for that Path, without affecting another
Path. Cached Home must hide those tracking controls after restart. Only a
participating-Path refresh begun after the rejection may restore them; an older
in-flight response must not reverse the decision. Invalid device-clock intervals must remain correction-required
locally; they must not become zero-duration server entries. Explicit correction
must carry the original timer identity separately from the reviewed occurrence
start and end. It must atomically settle that timer and record the corrected
occurrence, without starting a new live timer or stopping an unrelated timer.
The reviewed interval must satisfy ordinary time, membership and archival
validation. A previously discarded, completed or deleted timer remains terminal;
correction must not recreate it. Correction and its stable operation identity
must commit locally before the correction-required state is removed. Retries
must retain both the original identity and the same reviewed interval. A
validation rejection of a reviewed interval must stop automatic retry while
returning that session to correction-required state with the reviewed values
retained. A server timer refresh must not restart its local running display.

## Replay and session lifecycle

Replay must be automatic on connectivity and foreground restoration, serialized
causally for each entity, and use the same operation IDs for every retry. A
manual retry may wake this worker but must not create another operation.
Transient failures retain pending work with bounded backoff. Permanent outcomes
settle only their operation and retain a dismissible account-owned explanation.

An established session may restore local tracking without OIDC discovery.
A locally valid native session with a previously verified, securely persisted
account binding must open retained Home before network refresh, including near
credential expiry. A new interactive session must verify its account before
reading that account's retained Home. An expired credential must not use this
shortcut. Native Home metadata must remain separate from credentials and the
non-evictable timer ledger. Personal Path appearances must be retained in an
account-partitioned cache and restored before remote reads complete. Older
read responses must not overwrite a newer saved appearance revision.
Confirmed revocation pauses replay and new starts while keeping an existing
timer stoppable and preserving its pending result. The existing session storage
must atomically replace rejected credentials with a credential-free retained
account reference. Restoring this reference permits only local Stop and sign-in
recovery; it must not restore server-dependent or administrative surfaces.
Explicit sign-out clears this reference. Successful account replacement must
hide the previous retained account before exposing the replacement account. Sign-out timer resolution
must commit local stops before credential removal, or retain running timers
hidden until the same account returns. Network failure must not force a local
save to wait for server acknowledgement.

## Presentation and verification

Mobile and Studio must use their shared controls for offline/pending state,
correction, rejection, conflict and manual retry. All copy belongs to the shared
locale catalog. The nonblocking offline banner and per-entry pending indicators
must follow the product specification, including dismissal reset on reconnect.
Both clients must show the saved and discarded durations for an archival
rejection. A rejection may display its retained Path name only when the replay
outcome permits identifying that Path; inaccessible resources remain unnamed.

Acceptance must demonstrate process restart with original timer instants,
acknowledgement loss and duplicate replay, same-account restoration and different
account isolation, two-device timer conflicts and losing retries, archival
splitting, lost membership, device-clock correction, and unrelated queue
preservation. Run adversarial REST/persistence checks for the new boundary and
real platform persistence tests. Existing behavior tests should be reused;
source-shape assertions are not evidence of durable operation.
