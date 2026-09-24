# Account Authentication

Status: Approved for implementation

## Purpose

Define how users create and access accounts without application-managed
passwords.

## Social sign-in only

- Google and Apple must be the supported identity providers for the initial
  product on both Android and iOS.
- Both clients must present both providers so that an account created with one
  provider remains accessible from either supported platform.
- Additional identity providers may be added later but are not required
  initially.
- The application must support account creation and sign-in through approved
  external social identity providers.
- The application must not offer registration with an application-managed
  password.
- The application must not offer sign-in with an application-managed password.
- The application must not implement a local forgotten-password or password-reset
  workflow.
- Password and credential recovery must remain the responsibility of the selected
  identity provider.

## Native account entry

- Before an authenticated application destination is established, the native
  client must present account entry outside the authenticated primary-tab shell;
  signed-out, provider-preparation, onboarding, and duplicate-account recovery
  states must not expose Home, Following, or another signed-in destination.
- The signed-out experience must present a clearly visible, text-labeled primary
  sign-in action whose visible label remains stable while provider discovery or
  sign-in is in progress.
- Provider preparation and sign-in progress must be communicated once through a
  localized status with native progress semantics. Progress must not be
  communicated by replacing the primary action's label with text that can be
  clipped or hidden.
- While a sign-in attempt is active, the primary action must prevent duplicate
  submission. Provider cancellation must return to an operable signed-out state,
  and a provider or exchange failure must preserve a visible retry path.
- Successful account entry must transition directly to the authoritative server-
  selected Home, onboarding, or duplicate-account-recovery destination without
  briefly exposing a signed-out or unrelated primary surface.

## Single-account device model

- The initial product must support only one locally retained user account per
  device installation at a time.
- The product must not provide an account switcher, parallel signed-in sessions,
  or simultaneous local datasets for multiple users.
- Local activity, pending synchronization changes, and running timers must
  remain scoped to the single retained account.
- A timer kept running after sign-out must be available only when the same
  account signs in again; another account must not be able to inspect, stop,
  claim, or receive its elapsed time.

## Native account settings presentation

- Account settings must open as a titled native-stack destination from the
  compact Settings hierarchy and must use the platform back control and edge-
  swipe behavior.
- The destination must identify the currently activated account without
  exposing provider credentials or retaining another account's identity after
  account replacement or sign-out.
- Sign out and every running-timer resolution must use standard native
  confirmation actions, keep the reviewed account and timer count explicit,
  communicate admitted progress, and preserve a visible localized failure and
  retry path.
- A cold or direct Settings or Account route must mount visible localized
  loading or recovery before authoritative session resolution and must never
  render blank because Home's in-memory presentation context is absent.
- Same-account credential rotation must preserve the route and its owned sign-
  out work. Account replacement or sign-out must cancel prior-account
  presentation and late confirmations or completions must not act in, navigate,
  or expose identity to the replacement account.

## Replacing the retained account

- Signing in successfully with a different unlinked provider identity must
  replace the single account retained by that installation; the product must
  not require
  the prior account to sign in again merely to permit the replacement.
- Before clearing the prior local dataset, the application must automatically
  attempt to synchronize its pending changes when connectivity and its existing
  authorization allow.
- If the prior account has no unsynchronized local changes after that attempt,
  its local cache may be cleared automatically without an additional data-loss
  warning.
- A server-acknowledged running timer must remain attached to the prior account
  on the server and must not be stopped or deleted by the local account change.
- If unsynchronized local timers, entries, edits, deletions, or timer-stop
  actions remain, the application must warn clearly that continuing with the
  new account will permanently discard those specific pending changes.
- The warning must require explicit confirmation to continue or allow the user
  to cancel the account change.
- Confirming must discard the prior account's unsynchronized local changes,
  clear its remaining local dataset, and continue into the newly authenticated
  account without requiring the prior account to sign in.
- The warning must explain when discarding an unsynchronized timer-stop action
  will leave the corresponding server-backed timer running on the prior
  account.
- Replacing the retained account must not delete the prior application account
  or any data already synchronized to the server.

## Identity

- Social authentication must use OIDC.
- Every application account must have a stable internal user identity that is
  independent of any external identity provider.
- Paths, memberships, follows, profile data, recorded activity, notifications,
  and other product data must belong to the internal application user rather
  than directly to a Google, Apple, or other provider identity.
- Provider identities must be keyed by immutable issuer and subject identifiers;
  email must remain profile data rather than the identity key.
- Each immutable issuer-and-subject pair must be associated with at most one
  internal application user.
- The identity model must allow one internal application user to hold multiple
  external provider identities.
- An account must attach exactly one Google or Apple identity when it is
  created.
- The initial product must expose a user-facing identity-linking workflow in
  account settings so a signed-in user can attach the other supported provider.
- The application must never automatically link or merge application accounts
  merely because provider identities report the same email address.
- A successful first sign-in must allow the user to complete creation of an
  application account.
- A successful later sign-in with the same linked provider identity must access
  the same application account.

## Identity linking

- Adding another identity provider must not require replacing the stable
  internal application user or migrating its product data to a provider key.
- A user-initiated link must require both an authenticated application
  session for the existing user and successful authentication of the external
  identity being added.
- If the external identity is already associated with another application user,
  ordinary linking must fail without changing either account.
- Sign-in, email matching, and ordinary identity linking must never combine two
  existing application users or their data.
- If linking succeeds, either linked provider identity must subsequently sign
  in to the same internal application account.
- Account settings must show every linked provider without exposing its email
  to another user.
- A user must be able to unlink a provider identity only while at least one
  other provider identity will remain linked.
- The application must not allow unlinking the account's final sign-in identity.
- Linking and unlinking must not change Paths, memberships, activity, social
  relationships, or profile visibility.

## Duplicate-account prevention and recovery

- Before creating a new application account, if a new provider's verified email
  corresponds to an existing application account, the product must offer the
  person an opportunity to sign in to that existing account and link the new
  provider instead.
- Email correspondence must be treated only as a recovery hint. It must never
  prove ownership, create a link automatically, or disclose another account's
  private data.
- Completing the offered link must require successful authentication of both
  the existing account and the provider identity being added.
- Declining or canceling the offer must allow ordinary new-account onboarding to
  continue without linking the accounts.
- On mobile, this recovery state must present returning to sign-in for the
  existing account before the explicit choice to create a separate account.
- The two choices must remain text-labeled and operable without relying on
  icons, and must remain reachable with supported text-size reflow.
- While the separate-account choice is being applied, the mobile experience
  must prevent duplicate submission and announce progress; a failure must keep
  both choices available with localized retryable feedback.
- A nonmatching email, including an Apple private-relay address, must follow the
  ordinary account-creation flow and must not be assumed to represent a
  different or existing person.
- The initial product must not provide a full merge of two completed application
  accounts or transfer data between them.
- If separate completed accounts already exist, the recovery guidance must tell
  the user to choose the account they want to retain, permanently delete the
  unwanted account, and then link the released provider identity to the retained
  account.
- That guidance must explain that account deletion does not move or restore data
  and that all ordinary deletion warnings, ownership effects, and completion
  rules apply.
- A provider identity from an ordinarily deleted account must become available
  for linking after active deletion completes, subject to any applicable
  security or enforcement restriction.

## Offline access for an established session

- A user who previously signed in successfully and has an established local
  application session must be able to enter the application when the identity
  provider or the network is temporarily unavailable.
- Failure to refresh authentication solely because the device is offline must
  not block access to locally available time tracking.
- Offline access must provide the tracking and local-history capabilities
  defined in [Offline Behavior](../sync/offline.spec.md).
- A first-time user must not be able to create an account or complete initial
  sign-in without reaching the identity provider.
- Features that require current server data or authorization may remain
  unavailable until connectivity and authentication are restored.
- Locally recorded changes must remain pending until authentication can refresh
  and synchronization can proceed.

## Temporary provider failure and confirmed revocation

- A temporary inability to reach the identity provider or refresh authentication must not be
  treated as confirmed revocation and must continue to use the established
  offline-session behavior above.
- When the application receives reliable confirmation that access through the
  currently authenticated provider was revoked, it must enter a `Sign in
  required` state for the retained account.
- Confirmed revocation must suspend synchronization, social features, and other
  server-dependent or administrative actions until authentication is restored.
- Confirmed revocation must prevent the user from starting a new timer.
- A timer already running when revocation is confirmed must remain visible and
  stoppable; stopping it must preserve its elapsed activity locally for later
  synchronization.
- Unsynchronized activity and other pending local changes must remain retained
  and must not be discarded merely because access was revoked.
- Revocation must not delete the application account, server data, local data,
  or Path relationships.
- Signing in again with the same immutable provider issuer-and-subject identity
  must restore access and allow pending synchronization to resume.
- Signing in with a different unlinked provider identity must not claim,
  inspect, or synchronize the retained account's timers or local data.

## Signing out with running timers

- If the user attempts to sign out while one or more timers are running, the
  application must show a prompt before sign-out takes effect.
- The prompt must clearly identify that timers are still running and that the
  user must choose how to resolve them before signing out.
- The application must not silently stop or silently leave timers running as a
  side effect of sign-out.
- Dismissing or canceling the prompt must cancel sign-out and leave every timer
  running.
- Sign-out must not complete until the user explicitly selects one of the
  supported timer-resolution actions.
- The prompt must offer **Stop and save, then sign out**.
- Choosing **Stop and save, then sign out** must stop every running timer, save
  each elapsed session to its path, and complete sign-out only after those local
  saves succeed.
- The prompt must offer **Keep timers running and sign out**.
- Choosing **Keep timers running and sign out** must complete sign-out without
  stopping the timers.
- Timers kept running after sign-out must remain attributed to the account and
  paths that started them; sign-out must not transfer their time to another
  account.
- The signed-out application screen must not show the retained timers' Path
  identities, elapsed durations, or stop controls.
- Completing sign-out while timers remain running must remove their iOS Live
  Activities, Dynamic Island presentations, Android persistent notifications,
  and other operating-system timer surfaces.
- The same account must sign in again before those timers can be inspected,
  stopped, or saved through the application.
- After that account signs in, every retained timer must reappear with its
  original start time and current elapsed duration.
- Operating-system timer surfaces must be restored for those running timers
  after the same account signs back in, subject to current platform permission.
- The prompt must offer **Cancel**, which must leave the user signed in and every
  timer running.

## Related behavior

Profile setup after first sign-in is defined in
[Account Onboarding](onboarding.spec.md).
