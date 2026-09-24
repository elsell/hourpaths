# First Sign-In Onboarding

Status: Approved for implementation

## Purpose

Define the short profile-confirmation workflow that follows a user's first
successful provider/OIDC sign-in.

## First sign-in

- Before presenting setup for a new account, the application must perform the
  verified-email duplicate-account prevention flow defined in
  [Account Authentication](authentication.spec.md#duplicate-account-prevention-and-recovery).
- If the person authenticates an existing account and links the new provider,
  the application must open that existing account and must not create or onboard
  a second internal user.
- If no correspondence is found or the person continues with account creation,
  onboarding must proceed without implying that an email address uniquely
  identifies an application user.
- After first sign-in, the application must present a profile setup step before
  the user reaches Home.
- The setup step must be pre-populated with applicable profile data supplied by
  the selected provider through OIDC.
- The setup step must display the email address supplied by the provider, when
  one is supplied, so the user can confirm which provider identity is being
  used.
- The email must be presented as private account-reference information rather
  than a public profile field.
- The OIDC `name` claim must pre-populate the editable display name when
  supplied.
- The OIDC `picture` claim must pre-populate the optional profile picture
  when supplied.
- An OIDC `email` claim, including an Apple private-relay address, must remain a
  private account-reference email.
- No provider claim may directly set the application's username. The username
  must remain an app-specific suggestion that the user reviews under the
  application's format and uniqueness rules.
- If the optional `name` or `picture` claim is absent, onboarding must remain
  usable and allow the user to complete or omit the corresponding editable
  profile information according to its ordinary requirements.
- The user must be able to review the imported data before confirming it.
- The user must be able to correct editable profile information rather than being
  forced to accept inaccurate provider data.
- Required profile fields must be complete and valid before onboarding can be
  confirmed.
- Before onboarding can be confirmed, the user must complete the age and policy
  acceptance behavior defined in
  [User Policies, Age, and Support](../safety/user-policies.spec.md).
- The application must suggest an available username derived from the provider
  display name when possible.
- A suggestion must satisfy the same format and case-insensitive uniqueness
  rules as a manually entered username.
- The user must be able to replace the suggestion before completing onboarding.
- The user must explicitly review and confirm the username; onboarding must not
  silently finalize the suggested value merely because it is available.
- If a suggested or entered username becomes unavailable before confirmation,
  the application must leave onboarding in place, explain the conflict, and
  allow the user to choose another username.
- Successful confirmation must complete application-account creation and take
  the user into the application.

## Native presentation and completion

- Native onboarding must present one compact, scrollable profile-confirmation
  screen outside the authenticated primary-tab shell.
- The screen must use a clear task order: a concise introduction, the private
  provider account reference, editable application-profile information, policy
  and age review, and one primary completion action. Visual grouping must not
  combine private account-reference information with public profile fields.
- A provider email must be visibly identified as private, read-only account
  information and must remain visually subordinate to the editable profile
  fields.
- Editable fields, policy resources, acknowledgements, and the completion action
  must remain text-labeled; required meaning or operation must not depend on an
  icon alone.
- The native screen must provide an explicit username-review control even when
  the username was pre-populated. Editing the username must clear that review;
  retryable activation failures and policy refreshes must preserve it for the
  same incomplete account, while sign-out or session replacement must not carry
  it into another account's onboarding draft.
- Blank display names and invalid usernames must receive field-specific
  localized guidance exposed as accessibility error feedback. During
  onboarding, the client must apply the activation boundary's nonblank display
  name rule and the exact shared username-format rules both when enabling
  completion and again at submission. It must not enforce the unresolved
  100-character display-name limit until the product defines whether that limit
  counts Unicode scalar values or user-perceived grapheme clusters.
- With the software keyboard visible and at every supported Dynamic Type size,
  the screen must reflow or scroll so each field, policy action,
  acknowledgement, validation message, and the completion action remains
  readable, reachable, and operable within the safe area.
- Until confirmation requirements are satisfied, the screen must explain the
  remaining requirement in text rather than relying only on a disabled
  completion action.
- While confirmation is in progress, the client must prevent duplicate
  submission, keep the completion action's visible label stable, and communicate
  progress separately through localized text and native status semantics.
- The activation request and its presentation state must be owned by the current
  session operation. Sign-out or credential replacement must synchronously
  invalidate that ownership so a late response cannot affect the next account.
  Adoption of the credential produced by that same activation must retain the
  owner, busy presentation, and immutable onboarding draft until Home is
  published or the adoption failure is handled; only a genuinely superseding
  session operation may unlock the prior draft early.
- If the activated Home credential is retained after a retryable Home-load
  failure, onboarding must remain visibly locked and identify loading, offline,
  or server-error recovery state as applicable. It must offer an explicit retry
  and sign-out path until Home publishes or the credential is disposed, and it
  must never submit or apply stale onboarding edits against that Home credential.
- A validation or activation failure must preserve the user's editable values,
  identify an affected field when applicable, and leave a visible retry path.
- Successful confirmation must replace onboarding directly with the
  authoritative activated Home destination without briefly exposing signed-out
  content or authenticated primary tabs beneath onboarding.

## Leaving before confirmation

- Exiting or abandoning onboarding before confirmation must not complete
  application-account creation.
- An incomplete user must not have a discoverable profile, reserve a username,
  establish social relationships, or own or join a Path.
- The next successful provider sign-in must return the user to onboarding and
  pre-populate it again from the available provider data.
- The initial product is not required to restore unconfirmed edits made during
  an abandoned onboarding attempt.

## Returning sign-in

- A returning user with completed onboarding must not be required to reconfirm
  unchanged provider profile data on every sign-in.
- A returning user must be able to edit their application profile separately
  from sign-in.
