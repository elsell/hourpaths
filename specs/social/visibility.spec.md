# Profile and Path Visibility

Status: Approved for implementation

## Purpose

Define how profile privacy supplies defaults while allowing each path to control
its own visibility.

## Profile privacy and path defaults

- Always-public profile fields are defined in
  [User Profile](../accounts/profile.spec.md#always-public-information).
- Every profile must have a public or private privacy setting.
- Profile privacy must influence the default visibility offered when creating a
  path.
- A public profile must default a new path to public visibility.
- A private profile must default a new path to followers visibility.
- During path creation, the user must be able to replace the default with any
  visibility permitted by their profile privacy.
- Every path must store its own visibility setting.
- A path's visibility must be evaluated independently after it is created; the
  profile setting must not replace the path's explicit setting when access is
  checked.
- Profile privacy must set the maximum public visibility a path may have.

## Per-path privacy

- Every path must use one of three visibility levels: private, followers, or
  public.
- Only the Path creator must be allowed to change the Path's visibility after
  creation.
- Administrators, ordinary participants, and supporters must not be allowed to
  change Path visibility.
- An administrator's authority to manage explicit participant and supporter
  access must not grant authority to change the Path's general audience.
- Private visibility must restrict access to users granted access through a path
  role.
- Followers visibility must additionally allow the path creator's followers to
  access the path.
- Public visibility must allow users beyond the path creator's followers to
  access the path.
- A Path's visibility and explicit roles must govern access to participant
  identity, progress, and activity within that Path even when an individual
  participant's profile is more restrictive.
- A participant's profile privacy must not hide their Path activity from a user
  who is authorized to see it through the Path.
- Path-based access to a participant's identity or activity must not, by
  itself, grant access to that participant's profile or unrelated Paths.
- A user with a public profile must be able to create or change a path to be
  private.
- A private path must not be shown, discoverable, or otherwise disclosed to a
  follower solely because that user follows the profile.
- A user with a private profile must not be able to make an individual path
  publicly visible.
- The public visibility option must be unavailable while the profile is private.
- A private profile may use private or followers visibility for a path.
- A path may be more restrictive than its profile's privacy setting but must not
  be more publicly visible.
- Changing a profile from private to public must not alter the visibility of any
  existing path.
- After that profile change, existing private paths must remain private and
  existing followers paths must remain followers-only until explicitly changed.
- Public visibility must become available for new paths and explicit path edits
  after the profile becomes public.

## Changing a profile from public to private

- Changing a profile from public to private must retain all existing followers
  as approved followers.
- Changing a profile from public to private must change every existing public
  path to followers visibility.
- Existing private and followers paths must retain their visibility.
- Before the privacy change is confirmed, the profile owner must be clearly
  informed that all public paths will become followers-only.
- Applying the profile change and path visibility changes must be one atomic
  operation.
- Canceling or dismissing the change must leave profile and path visibility
  unchanged.
- Followers must not receive a notification merely because the profile became
  private or its public paths became followers-only.
- If the profile later becomes public again, affected paths must remain
  followers-only until explicitly changed.
- Path roles may grant access that ordinary follower status does not grant.
- The effective visibility of a path must be clearly shown to users who can
  access and manage it.

## Expanding path visibility

- When a path visibility change admits a broader audience, all existing eligible
  feed history from the path must become available to that newly admitted
  audience.
- Historical sharing on visibility expansion must happen automatically and must
  not require a separate **Share all** or **Start now** choice.
- Existing and future events must remain subject to all other current
  authorization and blocking rules.
- Expanding visibility must not change path membership, recorded activity, goal
  progress, or the visibility of another path.
- Reducing visibility must immediately hide Path details, activity history,
  feed events, and active timers from an audience that loses access, even while
  an authorization relationship update is awaiting retry.
- When reduced visibility removes a nonmember's effective Path access, any of
  that user's target-opening Path interaction notifications and pending push
  work must be retired under the notification lifecycle rules. Unrelated and
  still-accessible informational notices must remain, and later visibility
  expansion must not resurrect retired notification or push work.
- A visibility mutation must reconcile its Path authorization changes in
  durable resource order, using one strictly advancing mutation instant, before
  reporting success. A dependency failure must fail the request and leave the
  ordered change retryable; an identical replay must finish reconciliation
  before it may report success.
- After a creator broadens a Path's visibility, every current participant or
  administrator whose private profile would ordinarily be more restrictive
  than the Path's new audience must receive an informational notification.
- The notification must identify the Path, state its new visibility, and
  explain that the participant's identity, progress, and activity within that
  Path are now available to the broader Path audience.
- The notification must make clear that the participant's profile and unrelated
  Paths remain governed by their existing privacy settings.
- The notification must report the completed visibility change rather than ask
  the participant to approve or reject it.

## Changing an existing Path's visibility

- The active Path's effective visibility must be shown in its existing
  management surface to the creator.
- The creator must be offered only visibility levels permitted by the creator's
  current profile privacy. Other Path roles must not be shown a visibility
  control.
- A change to a broader audience must require explicit confirmation that names
  the Path, the current and proposed visibility, and explains that eligible
  historical identity, progress, and activity from this Path will become
  visible to that broader audience.
- The confirmation must explain that membership, recorded activity, progress,
  the creator's profile, and unrelated Paths will not change.
- Canceling or dismissing the confirmation must send no request and leave the
  Path unchanged.
- A confirmed change must include the visibility the creator reviewed. If the
  durable visibility changed before the request is applied, the operation must
  fail as a conflict and the client must reload the authoritative Path before
  another attempt.
- Repeating an identical confirmed request with the same idempotency identity
  must replay the original result. Reusing that identity for another Path or
  visibility transition must fail as a conflict.
- A successful response must return the authoritative Path projection. Web and
  native clients must replace their useful local Path state with that result.
- Narrowing visibility does not require the broader-audience warning, but it
  must still be an explicit submitted change and must take effect immediately.
- Selecting the already-effective visibility must send no request.
- A failed request must preserve the last useful Path state, explain that the
  change did not complete, and allow retry without duplicating side effects.
- Archived Paths and attempts by administrators, participants, supporters,
  unrelated users, or users who cannot discover the Path must fail without
  changing or disclosing protected Path state.

## Private-profile participants joining broader Paths

- Before a user with a private profile accepts participant access to a Path
  whose general audience is broader than the audience permitted by their
  profile, the application must warn them that the Path's access rules will
  govern activity they record there.
- The warning must identify the Path's current visibility and explain plainly
  that people who cannot open the user's private profile may still be able to
  see the user's identity, progress, and activity within that Path.
- The warning must explain that joining does not make the user's profile or
  unrelated Paths more visible.
- The user must see the warning before confirming acceptance; canceling or
  dismissing it must leave the invitation pending.
- If the user is rejoining and has retained activity on the Path, the warning
  must also explain that the retained activity will become visible again under
  the Path's current access rules.

## Inaccessible Paths

- A Path must not appear on Home, Following, profile, or search surfaces for a
  user who lacks permission to discover it.
- If an unauthorized user attempts to open a Path through a direct link,
  bookmark, stale notification, stale local reference, invalid identifier, or
  guessed identifier, the application must show a generic `Path unavailable`
  result.
- The result must not disclose the Path name, creator, visibility, membership,
  activity, or whether the Path exists.
- A nonexistent Path and an existing but unauthorized Path must be
  indistinguishable in user-facing error content.
- When a device learns that access was revoked, it must remove the Path from
  ordinary visible surfaces; attempting to open any remaining stale reference
  must use the same opaque result.
- An opaque access failure must not imply that requesting access is possible or
  reveal another user to contact.
