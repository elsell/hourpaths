# Path Roles

Status: Approved for implementation

## Purpose

Define the roles users may hold in relation to a path and establish the initial
authorization rule for managing shared goal configuration.

## Terminology

- **Creator:** The participant who created the path and uniquely retains the
  creator-only lifecycle, visibility, role-management, ownership, and deletion
  authority defined below.
- **Administrator:** A participant authorized to rename the Path, manage its
  goals, and manage ordinary participant and supporter access without receiving
  creator-only authority.
- **Participant:** A user authorized to track their own time against the path.
- **Supporter:** A user authorized to view designated path information without
  tracking time against the path.

## Goal-management authorization

- The path creator must be allowed to create, change, or remove the path's
  interval goal and overall target.
- A path administrator must be allowed to create, change, or remove the path's
  interval goal and overall target.
- A participant who is neither the creator nor an administrator must not be
  allowed to create, change, or remove the path's goals.
- A supporter must not be allowed to create, change, or remove the path's goals.
- A denied goal-management attempt must leave the goal configuration unchanged.

## Creator and administrator relationship

- The creator must always be a participant in the path.
- An administrator must also be a participant in the path.
- The creator and administrators must both be allowed to rename an active Path,
  edit its goals, and invite, remove, or switch ordinary participants and
  supporters subject to the ordinary membership rules.
- Only the creator must be allowed to change Path visibility, archive or
  unarchive the Path, grant or revoke administrator status, initiate ownership
  transfer, or permanently delete the Path.
- Administrators must not be allowed to perform those creator-only actions.
- Personal nudge, reminder, Home ordering, and Home pinning preferences must
  remain controlled by each individual user; neither the creator nor an
  administrator may change another user's personal preferences.
- The creator must not be changed to a supporter while retaining creator status.
- An administrator must not be changed to a supporter while retaining
  administrator status.

## Administrator role changes

- Only the creator must be allowed to grant administrator status to a
  participant.
- An administrator must not be allowed to grant administrator status to another
  participant.
- Only the creator must be allowed to revoke another user's administrator
  status.
- An administrator must not be allowed to revoke another administrator's status.
- An administrator must be allowed to voluntarily relinquish their own
  administrator status.
- Voluntarily relinquishing administrator status must require explicit
  confirmation.
- After stepping down, the user must remain a participant in the path.
- Canceling or dismissing the confirmation must leave the user's administrator
  status unchanged.

## Ownership transfer

- A path must have exactly one creator.
- Only the current creator must be allowed to initiate an ownership transfer.
- The creator must be able to transfer ownership to an existing participant in
  the path.
- An administrator is eligible to receive ownership because an administrator
  is also a participant.
- A supporter must become a participant before they can receive ownership.
- A Path must have at most one pending ownership-transfer request at a time.
- Initiating a transfer must create a pending ownership-transfer request for the
  selected participant.
- The selected participant must explicitly accept the pending request before the
  transfer takes effect.
- Until acceptance, the current creator must retain creator status and no role
  changes from the proposed transfer may take effect.
- The selected participant must be able to decline the request.
- Declining the request must leave all roles unchanged.
- The creator must be able to cancel a pending ownership-transfer request before
  it is accepted.
- Canceling the request must leave all roles unchanged.
- A canceled request must no longer be available for acceptance.
- A pending ownership-transfer request must expire automatically after the
  configured ownership-transfer lifetime.
- Each ownership-transfer request must store the reviewed timestamp and
  absolute expiration timestamp supplied by its signed reservation.
- The stored expiration must be calculated from the review time and the
  ownership-transfer lifetime configured at that moment.
- The request must use its stored expiration timestamp as the source of truth;
  it must not store a lifetime duration and recalculate expiration later.
- Once created, a request's expiration must not change because deployment
  configuration changes.
- An expired request must no longer be available for acceptance.
- Expiration must leave all roles unchanged.
- Initiating or completing an ownership transfer for an archived Path must be
  unavailable. A request that was pending when the Path was archived may be
  completed after the Path is unarchived only while its stored expiration is
  still in the future.
- Acceptance must revalidate that the selected recipient is still a current
  participant. If that membership changed, the request must be unavailable and
  all roles must remain unchanged.
- After cancellation, rejection, or expiration, the creator must be able to
  initiate a new ownership-transfer request.
- When ownership transfer completes, the recipient must become the new creator.
- When ownership transfer completes, the former creator must become an
  administrator and remain a participant.
- The former creator must retain the management permissions shared by
  administrators while losing the creator-only abilities to delete the path and
  transfer ownership.
- Ownership transfer must not alter any participant's recorded activity or goal
  progress.
- Pending ownership-transfer requests must remain reachable by their authorized
  creator and recipient independently of notification read or deletion state.

## Confirming an ownership-transfer request

- The creator must first request a server-authored review for the selected
  candidate before confirming an ownership-transfer request.
- The review response must use the server's canonical recipient identity and
  must include the review timestamp, the exact absolute expiration timestamp
  calculated from the configured lifetime at review time, and a signed
  reservation token binding the creator, Path, recipient, and both timestamps.
- Requesting or displaying a review must not create a transfer request, create
  an ownership-transfer notification, or change any role.
- Before creating an ownership-transfer request, the creator must be shown a
  clear summary of what will happen if the recipient accepts.
- The summary must identify the intended recipient.
- The summary must explain that the recipient will become the creator.
- The summary must explain that the current creator will become an administrator
  and remain a participant.
- The summary must explain that no role changes occur unless the recipient
  accepts.
- The summary must state when the pending request will expire.
- The request lifetime must be presented in a human-friendly unit appropriate to
  its configured length, using minutes, hours, days, or years.
- Relative lifetime formatting must not round away any portion of the configured
  duration; it must combine units when needed, such as "1 hour 30 minutes."
- The summary must show both the relative lifetime and the exact expiration date
  and time.
- The exact expiration must be rendered in the viewing user's configured time
  zone.
- The language must be concise and understandable to a nontechnical user.
- Creating the request must require explicit confirmation after the summary is
  shown by submitting the signed reservation token.
- Confirmation must revalidate that the token belongs to the authenticated
  creator and requested Path and that its canonical recipient remains eligible.
- The created request must persist the review timestamp and exact expiration
  timestamp from the reservation so the displayed expiration and the enforced
  expiration cannot diverge.
- Pending and mutation-result representations must identify the authenticated
  user's canonical counterpart and that counterpart's role in the transfer.
- Canceling or dismissing the confirmation must not create a transfer request or
  change any role.
