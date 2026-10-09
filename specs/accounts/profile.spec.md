# User Profile

Status: Approved for implementation

## Purpose

Define the basic profile information used to recognize a user without exposing
their paths or activity.

## Profile fields

- A profile must have a unique username.
- A profile must have a name.
- A profile must support a profile picture.
- A profile may have a description.
- The description must be optional.

## Profile editing

- After onboarding, a user must be able to edit their name, profile picture,
  description, and username within the application.
- Profile edits must update the user's application profile without requiring a
  corresponding change to a linked provider account.
- A later provider sign-in must not overwrite profile values the user has saved in
  the application.
- Changes to required or unique fields must satisfy the same validation rules as
  initial profile setup before they are saved.

## Editing authorization and outcomes

- Profile editing is an authenticated account-owner operation. The saved account
  must be selected from the active application session, never from a submitted
  owner identifier. A provisional, expired, revoked, or replacement account
  session must not modify another account's profile.
- Mobile and Studio account settings must expose editing of the application name,
  username, and optional description. Picture upload and removal remain required
  by the picture requirements below; delivery of text editing does not complete
  those requirements.
- A successful save must persist the submitted text fields together and make them
  available to subsequent own-profile and authorized public-profile reads.
- Validation failure, an unavailable username, or persistence failure must leave
  the saved profile unchanged. The editor must retain the submitted values so the
  user can correct or retry them.
- A save based on an older profile revision must not silently overwrite a newer
  edit. The client must retain the draft and offer recovery from the conflict.
- Retrying the same save after a lost response must not repeat the mutation or
  its audit event. Reusing a retry key for different input must be rejected.
- Profile changes must preserve the immutable account ID, provider identities,
  provider emails, memberships, recorded activity, and existing privacy rules.
- A later provider sign-in must retain the edited application profile, as required
  above. Public-text safety requirements remain governed by
  [Moderation](../safety/moderation.spec.md).

### Editing acceptance scenarios

1. An account owner edits their name, username, and description in either client;
   reopening the editor and viewing that profile in the other client shows the
   saved values on the same account.
2. A username differing only in case from another account's username is rejected;
   none of the submitted fields is saved and the editor retains the draft.
3. Clearing the optional description saves no description; an empty name or an
   invalid username cannot be saved.
4. An unauthenticated request, an onboarding-only session, or a forged owner
   identifier cannot alter a saved profile. Authorization dependency failures
   must fail closed.
5. Signing out during a save and entering another account cannot apply the old
   account's result or draft to the replacement account.

## Always-public information

- A user's username must be visible to everyone who can discover or access the
  profile, regardless of whether the profile is public or private.
- A user's name must be visible to everyone who can discover or access the
  profile, regardless of whether the profile is public or private.
- A user's profile picture must be visible under the same conditions.
- If supplied, a user's profile description must be visible under the same
  conditions.
- Always-public profile information must not include paths, goals, recorded
  activity, progress, or achievements.
- A user's provider account email must not be public, searchable, or exposed to
  followers, participants, supporters, or Path managers.
- Follower and following counts must be visible to everyone who can discover or
  access the profile, including before a private profile approves a follow
  request.
- The identities in the follower and following lists must be visible only to
  the profile owner and users who currently follow that profile.
- Participation or supporter membership in a shared Path must not by itself
  grant access to either identity list.
- Losing the follow relationship must immediately remove access to those lists.
- Blocking and account-deletion rules must continue to remove identities and
  adjust counts where applicable.
- Profile search and private-profile discovery behavior are defined in
  [Following](../social/following.spec.md#profile-discovery).

## Missing profile picture

- A profile picture must remain optional.
- When a user has no profile picture, every profile surface must show the
  product's neutral blank-avatar icon.
- The product must not generate an initials avatar or require the user to retain
  a picture originally supplied by an identity provider.

## Profile picture uploads

- A user must be able to upload JPEG, PNG, WebP, or HEIC image content no larger
  than 10 megabytes.
- The application must validate the actual image content rather than trusting a
  filename extension or declared content type.
- Accepted images must be re-encoded into an application-controlled safe image
  format and must have embedded metadata removed before storage or display.
- The user must be able to crop or position the image for a square profile
  presentation.
- Invalid, unsupported, or oversized image content must be rejected without
  changing the current profile picture.

## Private account identity reference

- Account settings must show each linked identity provider and any account email
  supplied by that provider.
- Provider emails must be visible only to the account owner and must not be
  editable as application profile fields.
- The application must not use email as the durable identity key; the immutable
  OIDC issuer-and-subject identity must remain authoritative.

## Username uniqueness

- Usernames must be unique case-insensitively.
- Two usernames that differ only by letter case must be treated as the same
  username for creation, editing, search, and profile lookup.
- The product may preserve the user's accepted capitalization for display, but
  display casing must not create a second identity or change uniqueness.

## Username format

- A username must contain between 3 and 64 characters, inclusive.
- A username may contain only ASCII letters, numbers, underscores, and periods.
- A username must not contain spaces or other characters.
- The same format rules must apply during onboarding and every later username
  change.

## Name and description format

- A display name must contain between 1 and 100 characters after surrounding
  whitespace is trimmed.
- A profile description must contain at most 500 characters.
- An empty or whitespace-only description must be treated as no description.
- The same limits must apply during onboarding and every later profile edit.
