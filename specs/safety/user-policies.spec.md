# User Policies, Age, and Support

Status: Approved for implementation

## Purpose

Define the policy acknowledgments and support information required for a social
product containing user-generated content.

## Published policies and contact

- The product must publish current Terms of Service, Privacy Policy, Community
  Guidelines, and support contact information.
- These resources must be reachable during onboarding, from account settings,
  and through the public support presence used by the application stores.
- The Privacy Policy must describe collected data, its uses and recipients,
  account deletion, backup retention, exceptional moderation or legal
  retention, and how to contact the operator.
- The Community Guidelines must describe disallowed content and conduct,
  reporting, blocking, enforcement, and appeals in plain language.
- Support contact information must provide a monitored route for users to reach
  a person or support process.

## Acceptance

- Before completing account creation, a user must explicitly accept the current
  Terms of Service and Community Guidelines and acknowledge the Privacy Policy.
- The application must retain the accepted policy versions and acceptance
  instant.
- A material policy change must require renewed acceptance before the user can
  resume server-dependent or social actions, while preserving access needed to
  review the change, export or delete the account, and stop running timers.

## Renewed acceptance for existing accounts

### Actors and preconditions

This flow concerns an authenticated, already activated account whose retained
policy acceptance does not cover a material policy change published by the
operator. Onboarding retains its own acceptance flow. The published policy
set remains the authoritative source of review links and version identifiers.

### Review and resumption

- Mobile and Studio must present the current policy links and the required
  explicit acceptance or acknowledgement before resuming restricted actions.
- The server must enforce this requirement for ordinary authenticated
  server-dependent and social actions; hiding client controls alone is not
  enforcement. The requirement must apply across domain entry points.
- A review and its submission must belong to the authenticated account. A
  previous account's review, delayed response, or saved form must not authorize
  the current account or populate its policy screen.
- A successful submission must retain the exact accepted versions and the
  acceptance instant atomically with its successful audit record. It must not
  erase prior acceptance evidence.
- If the published policies change after review, submission must not silently
  accept policies the user has not reviewed. The client must refresh the review
  and explain that renewed confirmation is needed.
- Retrying an acknowledged submission must not duplicate acceptance/audit
  effects or overwrite newer evidence. An uncertain network outcome must remain
  retryable without converting it into acceptance of a different policy set.
- A policy-required response observed during ordinary client use must open the
  current account's review. A response from an old credential or a different API
  origin must not open or populate a replacement account's review.
- The policy-required state must not be treated as an expired or revoked
  session. It must not erase valid credentials, retained local history, pending
  offline changes, or a running timer.
- Authentication, ownership, and destructive-action confirmation still apply
  to the permitted review, export, account-deletion, and timer-stop actions.
  The exemption must not grant ordinary tracking or social mutations.
- Account deletion may verify whether a known device notification belongs to
  the current recipient using an ownership-only response. The check must retain
  ordinary recipient authentication, visibility and audit rules, expose no
  notification content or target identifiers, and conceal missing and foreign
  notifications identically. Policy review must not block this cleanup check.

### Running timers during review

- While policy review is required, clients must offer a paginated list of the
  authenticated account's currently running timers and controls to stop them.
- This list must expose only the timer identifier, Path identifier and name, and
  start instant needed to identify and stop a timer. It must not expose another
  participant's timers or provide a general Path/social browsing exemption.
- Each listed Path must pass the existing tracking permission check. An
  authorization dependency failure must fail closed. Stopping uses the existing
  owner-scoped, idempotent stop operation and its normal audit/data rules.
- Locally retained timers and queued changes must remain retained during review;
  a policy-required response must not be interpreted as membership loss.
- A device-retained timer must use the existing durable stop operation. Its
  queued stop must remain available for synchronization after review, including
  when the network is unavailable. Clients must identify a pending device stop
  and must not offer its stale server counterpart as a second running timer.
  An uncertain server-stop response must retry the same reviewed timer and
  idempotency key, never a newer timer on the same Path.

### Account export boundary

- Export reads must derive the account identifier from the authenticated session.
  A caller-supplied account identifier must not select the exported account.
- Export must use explicit data projections. Credentials, session tokens, signing
  material, internal authorization records, other participants' private notes,
  and other accounts' private profile fields must not enter the export.
- Path-related export data must retain current Path authorization and membership
  rules, including archived Paths that the account can still access. A policy
  exception must not restore access to a Path the account left or lost access to.
- Large collections must be read in bounded pages with account-bound cursors.
  A cursor for another account or collection must not be accepted.
- Export reads and denials must use the normal audit and principal rate limiter.
  An authorization or audit dependency failure must fail closed.
- Clients must distinguish device-retained pending work from synchronized
  server activity. Export must not mutate, clear, or replay the local queue.
  Local records for inaccessible Paths must be excluded with a count, without
  exposing the inaccessible Path's name or identifiers.
- A failed page or account change must prevent a partial download from being
  presented as complete. File adapters must recheck account ownership after
  system save prompts and remove abandoned temporary share copies.
- The initial client download uses a versioned JSON document with collection
  start/end instants, account/profile data, accessible Paths and goals, recorded
  activity, and a separate device-pending section.
- Download packaging is a client concern. The API/application data boundary must
  not depend on whether the chosen download format is JSON or a CSV archive.

### Failure and acceptance scenarios

1. An existing account has accepted the prior Terms version. After a material
   Terms change is published, its current session remains valid but an ordinary
   server mutation is denied with a policy-review requirement. Both clients
   display the current review instead of offering sign-in again.
2. The account reviews and confirms the current policies. The versions,
   timestamp, and audit commit together; a subsequent request resumes ordinary
   authorized use without requiring another sign-in.
3. Publication changes between review and confirmation. Submission rejects the
   stale review and the client displays the new versions for explicit review.
4. The save succeeds but its response is lost. Retrying the same submission
   returns the committed result without repeating its side effects.
5. A second account submits the first account's review evidence. The server
   rejects it without changing either account's acceptance.
6. While renewed acceptance is required, the account can review the policies,
   export its permitted data, explicitly delete its account, and stop an
   existing running timer. It cannot use these exceptions to start a new timer
   or perform unrelated server-dependent or social actions.
7. A policy service/network failure is distinguishable from valid acceptance;
   the client retains its session and work and offers recovery. Switching
   accounts while a request is pending discards the old account's result.


8. On mobile, review opened while Following or Stats is active must still allow
   account deletion and sign-out. Opening deletion must reveal its confirmation;
   canceling must return to policy review without deleting data.
9. After acceptance refreshes the current session projection, a later policy
   publication must retain working sign-out and deletion actions. The review
   must bind to the current credential instance and discard replaced instances.

## Minimum age

- The initial product must be available only to users who attest that they are
  at least 16 years old.
- The application must not collect an exact birthdate solely to enforce this
  initial age boundary.
- A user who does not attest that they are at least 16 must not complete account
  creation or receive a discoverable application profile.
- The product must not be marketed or represented as directed to children.
