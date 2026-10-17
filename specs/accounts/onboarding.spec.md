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
- The user must be able to review imported identity text before confirming it.
  The optional photo task must show the automatically imported picture and
  provide removal or replacement without another account confirmation.
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

## Guided presentation and completion

- Web and native onboarding must use a short, polished step-by-step flow rather
  than one form containing every field and acknowledgement. Native onboarding
  must remain outside the authenticated primary-tab shell.
- The flow must separate profile identity, privacy/preferences, age/policy review,
  and optional photo into focused tasks. The optional photo task follows
  successful account confirmation and precedes Home, reusing the active-account
  image-processing and upload boundary. It must show the current step and
  provide a clear primary action and a way to return to earlier steps without
  losing edits. Steps must share presentation primitives and spacing with the
  rest of their client rather than introduce one-off controls.
- Profile identity must begin with a concise introduction and editable display
  name and username. Private provider account information must remain visually
  separate and subordinate to public profile fields.
- The photo prompt must appear during initial setup, be clearly optional and
  provide a visible Skip action. Skipping must not prevent account creation;
  upload failure must preserve a way to skip or retry without reactivating the
  account. Back navigation after activation must not return to account creation.
  A provider-supplied picture must be imported automatically through the safe
  image-processing boundary. The photo step must show the imported picture and
  allow removal or replacement. Import failure must not block setup or Skip.
- Profile visibility must initially select the owner-approved social/public
  option, explain its meaning and allow the person to choose private before
  final confirmation. Device-derived time zone and week start remain editable
  under the existing preference rules.
- Required acknowledgements must be confined to the final review task and must
  remain explicit. Merely visiting a step or opening a policy must not accept it.
  Only final confirmation may activate the account. Back navigation must not
  mutate the server or clear same-account edits.
- A provider email must be visibly identified as private, read-only account
  information and must remain visually subordinate to the editable profile
  fields.
- Editable fields, policy resources, acknowledgements, and the completion action
  must remain text-labeled; required meaning or operation must not depend on an
  icon alone.
- Continuing from the profile-identity step must explicitly confirm the visible
  username, including a pre-populated suggestion, without a separate review
  toggle. Editing the username must clear that review;
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
- Successful confirmation must adopt the authoritative activated account before
  presenting optional photo setup. Completing or skipping that task must open
  Home without briefly exposing signed-out content or primary tabs beneath
  setup. It must not repeat account activation.

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

## Guided-flow acceptance

1. A new user reviews identity and the initially social profile setting, then
   explicitly completes age/policy review before the single activation request.
   The user can add a photo or skip directly to Home without another activation.
2. Back and forward navigation preserve edits. An invalid name or username
   prevents advancing with field-specific guidance; a server username conflict
   returns to identity without clearing other completed input.
3. Policy changes invalidate stale acknowledgements and return to review while
   preserving identity edits. Sign-out or account replacement discards the prior
   account's step state and draft.
4. Keyboard, enlarged text and screen-reader navigation keep every action
   reachable; optional photo failure does not trap the person in setup.
5. Activation, Home loading and retry retain the existing session-operation
   ownership and mutation guards; changing presentation must not bypass them.

The web photo task uses the authenticated `/studio/setup/photo` destination.
The browser navigation adapter must admit only that fixed internal path; it must
not accept a caller-supplied redirect URL. Existing-account restoration must
continue to Home without replaying photo setup.

## Provider-photo decision — October 17

The owner approved automatic import of a provider-supplied picture, with removal
and replacement available during setup. Explicit Use this photo consent is not
required. Existing image validation and re-encoding requirements still apply.
