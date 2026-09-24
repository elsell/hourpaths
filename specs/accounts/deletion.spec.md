# Account Deletion

Status: Approved for implementation

## Purpose

Define a user's ability to permanently delete their account and remove their
personal data and path relationships.

## User capability

- A user must be able to permanently delete their account.
- Completing account deletion must make the account unavailable for future use.
- Account deletion must not be presented as a reversible deactivation.
- Before final confirmation, the user must receive a prominent warning that
  account deletion is permanent, begins immediately, and has no recovery or
  cancellation window.
- The warning must summarize that the user's profile, preferences, activity,
  progress, social content and relationships, pending changes, and active-timer
  time will be permanently removed.
- The user must explicitly confirm deletion after seeing the complete warning;
  canceling or dismissing it must leave the account unchanged.
- A valid signed-in application session must be sufficient to reach and confirm
  account deletion.
- The initial product must not require recent provider reauthentication, a
  provider-password challenge, biometric verification, or a device-passcode
  challenge before deletion.
- Once confirmed, deletion must begin immediately and the product must not offer
  an undo, grace period, delayed-deletion cancellation, or account recovery
  workflow.
- In addition to in-application deletion, the product must provide a public web
  route through which an authenticated user can request deletion of their
  account and associated data.
- The web route must apply the same identity verification, warning, ownership,
  timer, and deletion effects as the in-application workflow.

## Personal data removal

- Account deletion must stop every timer running for the user.
- Elapsed time from timers stopped by account deletion must be discarded and
  must not be saved as final recorded activity.
- Account deletion must remove the user's profile and preferences.
- Ordinary account deletion must remove the account's active provider-identity
  associations so those identities can later be linked to another account after
  deletion completes.
- Account deletion must remove the user's recorded activity from every path.
- Account deletion must remove the user's reactions, comments, follow
  relationships, and pending social requests.
- Account deletion must remove pending path and ownership-transfer invitations
  sent to or awaiting action from the user.
- Totals, goal progress, statistics, and feed content derived from deleted data
  must no longer include that data.
- Account deletion must remove the user's locally retained account data,
  unsynchronized changes, authentication session, notification state, and device
  notification registration.
- Account deletion must clear every Live Activity, persistent notification, or
  other operating-system surface representing the user's timers.
- Account deletion must not be presented as complete while the application still
  retains an active timer or pending activity for the user.

## Completion and retention

- Confirmation must immediately disable the account, revoke active sessions,
  and remove its data from user-facing product behavior.
- Removal from active databases, indexes, caches, device registrations, and
  ordinary application replicas must complete as one deletion workflow and no
  later than 24 hours after confirmation.
- The application may retain deleted data only in encrypted rolling backups for
  at most 30 days, during which the data must remain beyond use and must not be
  queried, restored into active use, or processed for another purpose.
- Any restoration from a backup must reapply all deletion records before the
  restored system serves users or background work.
- Application logs must exclude user content and private activity notes.
- Operational or security logs that can identify the deleted user must expire
  within 30 days unless the relevant record is part of an active security or
  moderation investigation.
- When an account is subject to active moderation or enforcement, the product
  may retain only the minimum report evidence and a pseudonymous provider-
  identity reference needed to complete that work.
- That exceptional moderation or enforcement record must expire no later than
  90 days after the case closes unless a legal obligation requires otherwise.
- Ordinary profile, Path, activity, social, notification, and preference data
  must not be retained under the moderation exception.
- The deletion warning and published privacy information must explain the
  backup schedule and the limited moderation, security, or legally required
  exceptions plainly.

## Path relationships

- Account deletion must remove the user as a participant or supporter from every
  path they do not own.
- Account deletion must revoke every administrator role held by the user.
- Removing the user from a shared path must not delete another participant's
  recorded activity.

## Creator-owned paths

- Owning a path must not block account deletion.
- Account deletion must delete every path for which the user is still the
  creator, including shared paths.
- Before confirmation, the user must be clearly warned that all paths they still
  own will be deleted.
- The warning must make clear that shared paths used by other participants are
  included.
- The warning must make clear that deleting each owned path permanently deletes
  every participant's recorded activity within that path.
- The warning must advise the user to transfer ownership of any shared path they
  want to preserve before deleting the account.
- The application must not transfer ownership automatically.
- The user must explicitly confirm deletion after seeing the warning.
