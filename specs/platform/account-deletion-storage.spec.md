# Account deletion storage and recovery

The [account deletion specification](../accounts/deletion.spec.md) owns product
behavior. This document defines its persistence and recovery boundaries.

- The client must generate and retain an unpredictable deletion receipt secret
  before sending confirmation. The secret must contain at least 128 bits of
  cryptographic randomness and must be scoped to the reviewed account.
- The authenticated confirmation must atomically store only a SHA-256 digest of
  that secret with the account deletion record, remove product data and sessions,
  enqueue authorization cleanup, and append immutable audit evidence.
- A receipt request must prove possession of the exact secret for the exact
  account. It may return only successful deletion status, never profile data or
  a new session. A missing or mismatched receipt must fail identically.
- Ordinary expired or revoked credentials must remain invalid. Receipt lookup
  is a separate, narrow capability and must not bypass ordinary authentication
  on the deletion command or on any other endpoint.
- Receipt reads must use the principal limiter and append audit evidence after
  successful capability verification. Final verification and its audit write must
  commit atomically while locking the deletion record against retention cleanup.
  The secret must never appear in logs,
  audit data, URL parameters, or deletion records. Receipt lookup must reject
  records older than 30 days.
- Client cleanup must target the captured deleted account, even if another
  account signs in while a response is pending. Until completion is proven,
  clients must retain their pending-deletion state and must not replay its work.
- Local storage must keep an opaque account write barrier after deleting the
  profile, history, queues, caches and receipt secret. The barrier may contain
  only the account ID; it must prevent late responses or older open clients from
  repopulating the deleted account's data. It must not affect another account.
- Restored backups must apply the externally retained deletion records before
  serving traffic or workers. A local record inside the restored backup alone
  is insufficient evidence of restoration safety.

## Native notification cleanup

- Before the server deletion request, the native client must durably capture the
  identifiers of the account's presented and scheduled notifications. Retries
  must retain those identifiers until local cleanup and credential removal both
  complete, including when credential removal fails after data removal.
- Cleanup must target captured identifiers; it must not dismiss a replacement
  account's notifications wholesale. Restart recovery must check deletion intent
  before restoring cached Home or starting sync.

## Deletion-specific audit expiry

- A separately credentialed, bounded retention operation must remove audit details
  identifying deleted accounts independently of general audit-history retention.
  It must start at 29 days after deletion, leaving one day for retries before the
  30-day limit. The runtime API must retain no audit update or delete privilege.
- Each batch must append an immutable summary containing only its completion
  time, batch limit and count. The retention credential must not read audit
  details or directly mutate audit or deletion records.
- The operation must preserve audit details for accounts not eligible for deletion
  expiry. Receipt access after cleanup may create new audit evidence; subsequent
  batches must include it while the receipt remains available.

## Offline restore replay

- Restore replay must consume an operator-controlled deletion manifest exported
  from the authoritative deletion records outside the backup being restored.
  It must validate the complete manifest before changing the restored database.
- A deletion manifest must declare its minimum supported backup creation time.
  Replay must reject backups older than that boundary, even if they are otherwise
  within 30 days. Before retiring external records, cleanup must durably advance
  that boundary so a backup that could contain a retired account cannot be used.
  A missing boundary must fail closed; an empty record list is not proof that an
  arbitrary older backup is safe.
- Replay must preserve the original deletion time and receipt digest, remove the
  same personal data as ordinary deletion, and queue authorization cleanup.
  Replaying the same manifest must be idempotent. A backup containing an older
  provisional state of the deleted account must also be cleaned.
- Replay must run with API and background workers stopped. Replaying database
  rows alone does not authorize reopening traffic: the restore workflow must
  also verify deletion-manifest completeness, authorization convergence, backup
  retention and encryption, and the absence of deleted accounts.

## External write-ahead deletion records

- After authenticating the reviewed account and explicit irreversible confirmation,
  the application must durably retain the accepted deletion record outside the
  primary database before removing data. Failure to persist that record must
  prevent database removal.
- The record must contain only the account ID, original acceptance time, audit
  event ID and receipt digest. It must be encrypted at rest with an authenticated
  cipher. Its encryption key must be separately managed as a secret.
- Repeated admission for the same account and receipt digest must return the
  original record. A competing digest must not overwrite the accepted request.
- An accepted request survives a later database transaction failure and remains
  an irreversible pending deletion. Recovery must reapply these accepted records,
  including requests whose final database response was lost. A rolled-back
  database transaction must still leave its immediate database state unchanged.
- Restoring an older backup must use this external record set; a periodic export
  alone must not be treated as proof that all accepted deletions are represented.

- Server startup and a recurring bounded recovery pass must retry accepted
  deletions without requiring the deleting client to remain open. A failed
  account must not prevent other valid accepted requests from being processed.
  Retries must preserve the original acceptance record and remain idempotent.
- An accepted external record must prevent subsequent session authentication,
  issuance, rotation, and onboarding activation for that internal account ID,
  even while database removal is pending. An unreadable journal must fail closed
  as a service failure, not misclassify an unaffected user's credential as expired.
- Retention commands must not log SQL parameters, connection strings, credentials,
  account identifiers, or unrestricted database errors. Operational failure output
  must use a generic failure event and nonzero exit status.
- Native notification cleanup identifiers must be durable before a recoverable
  deletion intent is saved. Failure to capture them must prevent admission; a
  restart after admission must not require the old credential to capture them.
- Starting sign-in directly from the public or canonical deletion entry must
  return to the authenticated deletion review. Starting an unrelated sign-in must
  clear any abandoned deletion-entry redirect; arbitrary return URLs are forbidden.
- A native deletion snapshot must include only notifications whose recipient
  ownership is established for the deleting account. It must preserve another
  account's notices and fail closed when ownership lookup is unavailable.
- Push metadata must carry the opaque internal recipient account ID for local
  deletion cleanup. It must not authorize any server operation. Native clients
  must suppress foreground delivery and dismiss retained notices for locally
  fenced recipients on receipt and app activation, even without a signed-in
  session. Legacy notices without recipient metadata require authenticated
  recipient resolution before cleanup.
- The runtime database adapter must not emit raw SQL or interpolated query
  parameters through ORM logging. Application diagnostics must use the existing
  typed observability boundary rather than duplicate personal records in logs.
- Deleted-account retention must also remove completed permission-delivery
  records containing the deleted account as actor, owner, user subject, or user
  resource. Cleanup must share the bounded retention batch and immutable summary.
  It must preserve unfinished revocations and unrelated accounts' delivery records.
