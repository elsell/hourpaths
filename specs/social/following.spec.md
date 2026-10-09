# Following

Status: Approved for implementation

## Purpose

Define the one-way social relationship used to discover and receive another
user's shared activity.

## Terminology

- **Follower:** A user who follows another user.
- **Following:** The user being followed.
- **Follow request:** A request awaiting approval from a user with a private
  profile.

## Relationship model

- Profile privacy is defined in
  [Profile and Path Visibility](visibility.spec.md#profile-privacy-and-path-defaults).
- Following must be one-way and must not require reciprocity.
- A user may follow another user without being followed back.
- Following another user must not automatically make that user a follower.

## Activity notification preference

- Each follower must be able to choose whether to receive notifications when a
  followed user starts an eligible timer.
- This preference must belong to the follower-following relationship so that a
  user can enable it for some followed profiles without enabling it for all.
- The preference must default to disabled when a follow relationship is
  established.
- The product should present this as an explicit opt-in comparable to subscribing
  to a followed profile's activity notifications.
- Changing the preference must not unfollow the profile or change access to its
  feed events.

## Profile discovery

- Users must be able to search for profiles in the initial product.
- Profile search must match usernames and display names.
- Search and pre-approval profile views may show the username, display name,
  profile picture, and description for both public and private profiles.
- A private profile's pre-approval view must not expose follower-only Paths,
  feed activity, or other content merely because the profile was found in
  search.
- Search results and profile access must respect the profile owner's privacy,
  blocking, and visibility rules.
- Discovering a profile through search must not itself establish a follow
  relationship or grant access to follower-only content.
- A search query must contain at least two non-whitespace characters before a
  server search is performed.
- Matching must be case-insensitive and Unicode-normalized for display names and
  case-insensitive under the username rules for usernames.
- Results must rank exact username matches first, followed by username-prefix,
  display-name-prefix, and then substring matches.
- Results within the same relevance class must use username as a stable
  deterministic tie breaker.
- The initial product must not require fuzzy or typo-tolerant matching.

### Find people acceptance slice (`SOC-01A`)

- The initial `Following` surface must let an authenticated user search for
  profiles and open a dedicated profile destination from a result.
- Entering fewer than two non-whitespace characters must not send a profile
  search request.
- Search results must use compact identity rows and the dedicated profile
  destination must show only the always-public profile projection: username,
  display name, optional safe profile picture and description, and accurate
  follower and following counts.
- Until profile-picture upload is implemented, a missing picture must use the
  product's neutral blank-avatar icon.
- Search and profile reads must exclude a blocked user in either direction and
  must never disclose email, provider identity, profile privacy, Paths, goals,
  activity, or internal account state.
- Loading, empty, retry, unavailable, refresh, and pagination behavior must be
  accessible and localized. Newer searches must not be replaced by stale
  responses from older queries.
- This slice must not create, approve, reject, cancel, or remove a follow
  relationship and must not present controls that imply those actions are
  available.

### Follow people acceptance slice (`SOC-01B`)

- From a dedicated profile, an authenticated user must be able to follow a
  public profile immediately or send a non-expiring request to a private
  profile without learning the target's privacy setting directly.
- The profile action must present the authoritative viewer-relative state as
  `Follow`, `Requested`, or `Following`; the user's own profile must not present
  a follow action.
- The sender must be able to cancel a pending request and unfollow an
  established relationship. The private-profile owner must be able to approve
  or reject each request from a dedicated Follow Requests destination that
  remains reachable from Following independently of notification history.
- Approval must establish exactly one one-way relationship. Rejection,
  cancellation, and unfollowing must take effect immediately, remain silent,
  and leave an independently granted Path role or opposite-direction follow
  unchanged.
- A public follow must notify the followed user, a private request must create
  one actionable notification for its recipient, and acceptance must notify
  the requester. Notification deletion must never delete or resolve the
  underlying request.
- Pending requests must grant no follower-only access. An established follow
  must grant only the access provided by the followed user's followers-visible
  Paths, and losing that relationship must revoke follow-derived access
  immediately while retaining independent Path-role access.
- Missing, inactive, deleted, or blocked profiles in either direction must be
  indistinguishable to the requester and must produce no relationship,
  notification, count, audit-success, or authorization side effect.
- Requests and mutations must be idempotent and race-safe. Concurrent approval,
  rejection, cancellation, privacy change, blocking, or account deletion must
  converge without duplicate relationships or notifications.
- Follow Requests must use stable opaque cursor pagination and compact safe
  identity rows. Loading, empty, refresh, pagination, retry, unavailable,
  mutation-progress, success, and failure states must be localized and
  accessible on native and web surfaces.
- Follower/following identity-list browsing, removing an existing follower,
  and per-relationship timer-start notification opt-in remain outside this
  slice.

## Native discovery and profile presentation

- Following must expose Find People and Follow Requests as independently
  recognizable standard native bar actions. Neither action may be hidden behind
  an unrelated content control or duplicated as custom in-content chrome.
- Find People must use the platform search presentation and compact,
  intrinsic-height identity rows. The search field, loading status, result
  identity, and disclosure action must reflow without clipping or horizontal
  scrolling at every supported Dynamic Type size.
- A discovered profile and Follow Requests must open as dedicated native-stack
  destinations with a localized current-view title and platform back control.
  Standard menus and confirmations must own secondary and destructive actions.
- Profile presentation must lead with the safe public identity projection,
  followed by accurate counts, the authoritative viewer-relative relationship
  state, and available actions. Heavy custom cards and custom glass must not
  wrap that hierarchy.
- Profile, search, and request loading, stale-useful, empty, unavailable, retry,
  refresh, pagination, and mutation-progress states must remain distinct and
  localized. A cold or direct route must mount a visible loading or recovery
  presentation rather than return blank while presentation context or its
  authoritative target resolves.
- Native clients must accept the generated API's optional top-level `$schema`
  link on follow, follow-request list, and follow-request review responses while
  continuing to reject unexpected envelope metadata and malformed schema links.
- Search and relationship results must remain owned by the activated
  account/profile lineage, query or target, and admitted operation. Credential
  rotation for that same account must preserve owned work without allowing an
  older credential failure to dispose the newer credential. Sign-out or account
  replacement must cancel presentation ownership, and a stale response or late
  completion must not replace newer state or populate a replacement account.
- A delayed native confirmation for unfollowing, blocking, or another
  relationship action must freeze its owning account/profile lineage, target,
  reviewed capability, action, and idempotency key. It must revalidate that
  frozen intent immediately before admission and send no request after sign-out,
  account replacement, target change, capability loss, or superseding intent.
- VoiceOver order must follow identity, public profile facts, relationship
  state, then available action. Reduce Motion, Reduce Transparency, and Increase
  Contrast must preserve the same hierarchy without depending on animation,
  translucency, or color alone.

## Public profiles

- Following a public profile must take effect without approval from the profile
  owner.

## Private profiles

- Attempting to follow a private profile must create a follow request rather than
  immediately establishing the follow relationship.
- The private profile owner must be able to approve or reject the request.
- Approval must establish the follow relationship.
- Rejection must not establish the follow relationship.
- A pending follow request must not expire automatically.
- It must remain pending until it is accepted, rejected, canceled by the sender,
  or invalidated because either account is deleted or one user blocks the other.

## Profile privacy changes

- Changing a profile from public to private must preserve every established
  follow relationship.
- Existing followers must be treated as already approved and must continue to
  receive follower-level access after the profile becomes private.
- Follow attempts made after the profile becomes private must use the ordinary
  follow-request and approval workflow.
- The privacy change must not require the profile owner to reapprove existing
  followers.

## Removing a follower

- A user must be able to remove an existing follower without blocking them.
- Removing a follower must end only that follower-to-profile follow
  relationship; it must not end a follow relationship in the opposite direction.
- Removing a follower must not notify the removed user.
- A removed follower must lose follower-only access and feed eligibility.
- Removing a follower must not prevent them from following again immediately
  when the profile is public or sending a new request when the profile is
  private.

## Profile connection lists and follower removal (`SOC-01C`)

- Mobile and Studio profile follower/following counts must open the corresponding
  paginated identity list for the profile owner or a current follower. Counts
  remain public; counts must not imply that an unauthorized identity list is
  available. The visibility rules in [User Profile](../accounts/profile.spec.md)
  are authoritative for these lists.
- Every page must revalidate current visibility. A cursor must be bound to the
  requesting account, profile, and list direction; it must not grant access after
  unfollowing, removal, blocking, or account deletion.
- Lists must use the shared compact identity rows and safe avatar, support opening
  a listed profile, and provide localized loading, empty, retry and pagination
  states. Account replacement must discard the previous account's list and work.
- Only the owner of the follower list may remove an inbound follower. The action
  must clearly identify the affected person and require explicit confirmation.
  Its behavior is governed by Removing a follower above; independent Path access
  and the opposite-direction relationship must remain unchanged.
- Removal must persist the relationship change, retry receipt and audit event
  atomically and converge authorization before returning success. A retry after
  a lost response must not remove a newly established follow relationship.
- Removal must not create a notification. Pending deliveries that depended on
  the removed follow subscription must no longer be eligible.

### Acceptance scenarios

1. An owner and a current follower can page through both lists. An unrelated user
   can read the counts but not the identities, even with shared Path membership.
2. A cursor from another account, profile or direction is rejected. Losing the
   follow relationship between pages removes access immediately. Blocked and
   deleted identities are excluded.
3. Removing an inbound follower updates counts and removes follower-derived
   access without a notification, without ending the reverse follow, and without
   removing an independent participant/supporter role.
4. The removed person can follow a public profile again or request a private
   profile again. Replaying the old removal cannot remove that new relationship.
5. Failure or account replacement cannot report false success, leak the former
   account's list, or discard the user's ability to safely retry.

## Canceling and unfollowing

- A user must be able to cancel a follow request they sent while it remains
  pending.
- Canceling must remove the pending request immediately and must not notify the
  recipient.
- A user must be able to unfollow an established profile at any time.
- Unfollowing must take effect immediately and must not notify the other user.
- Unfollowing must remove follow-based feed eligibility, follower-only access,
  and the relationship's timer-start notification preference.
- Canceling or unfollowing must not remove access independently granted through
  participation or supporter membership in a shared Path.

### Live active-path progress

Active-following timers must include recorded lifetime progress, configured overall
goal, and the current interval's recorded progress, target, recurrence and UTC
boundaries, calculated in the participant's time zone at the response instant.
Clients must add only the running session's overlap with those boundaries and
refresh at the interval end. Progress must not include private notes or another
user's personal appearance. Existing active-following eligibility and authorization
must apply to every returned path.

### Active-path avatar rings and navigation

- Each active person's highlighted avatar outline must contain one segment for
  each currently visible active Path. One active Path must render a continuous
  outline. The strip must not print a separate active-Path count below the name;
  accessible labels must still identify the person's active Paths.
- The full-screen viewer must change instantly between Paths belonging to the
  same person. Horizontal slide transitions may occur only when navigation
  crosses to another person, and must be disabled when Reduce Motion is enabled.
- The viewer's top progress segments must represent the selected person's Paths,
  not the total number of active people. Navigation must remain manual.

### Prominent profile relationship action

- Another user's profile must show a prominent full-width Follow, Requested,
  or Unfollow button beneath their identity and biography, preserving current
  request, confirmation, authorization, busy, and failure behavior.
- The current user's own profile must not show a follow control.

### Profile header and active Paths

- Mobile profile headers must place the avatar beside a compact row of Paths,
  Followers, and Following counts, with identity and description below and the
  existing prominent relationship action beneath them.
- The avatar must reuse the Following strip’s segmented outline, one segment
  per currently visible active Path. An inactive profile must have no outline.
- Tapping an active profile avatar must open that person’s Paths in the shared
  full-screen live viewer, preserving manual navigation and instant transitions
  between Paths belonging to the same person.
- Counts and active Paths must respect existing Path authorization and bilateral
  blocks; the header must not reveal inaccessible Paths.

- The Paths count must include distinct unarchived Paths in which the profile
  owner currently participates (creator, administrator, or participant), excluding
  supporter-only membership and Paths inaccessible to the viewer.
- Profile active Paths must include authorized running timers regardless of
  whether the viewer follows that person, including the viewer’s own profile.
- A profile must refresh live state on focus, foreground, and while its viewer
  is open. Stopped or newly inaccessible Paths must disappear on refresh.
- Failure to load the Path summary must not display a fabricated zero or enable
  a stale active outline; retry must be available through profile refresh.
