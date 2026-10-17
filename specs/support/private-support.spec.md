# Private support

Status: Approved for implementation (October 17, 2026).

## Purpose and actors

HourPaths must provide a private way for a requester to contact the operator
without a public support email. A requester submits a support request. An
authorized support operator reviews requests and replies. A request and its
messages form one support conversation, separate from social content and
moderation cases.

## Request and reply behavior

- The public support URL must provide an actual route to support, and existing
  mobile and Studio support links must reach it.
- A requester must be able to enter a subject and message, review them and submit
  the request. Inputs must be text labeled, accessible and localized through the
  shared locale catalog. Message content must be displayed as plain text.
- The form must explain that requests are private and must discourage sending
  passwords, sign-in codes or session tokens.
- Submission must persist before the client displays success. A retry after an
  uncertain response must not create a duplicate conversation.
- Failed submission must preserve the draft and provide a clear retry action.
- Success must provide a reference and a way to return to the conversation.
- The requester must be able to read operator replies and add a follow-up.
- The operator must have a usable inbox and reply workflow. A form that stores
  messages without a way for the operator to review them is not complete.
- The service must not imply email delivery or promise a response time.

## Privacy and boundaries

- Only the requester and authorized support operators may read a conversation.
  Followers, Path members and ordinary account holders must not gain access.
- Access must be enforced at the server; hiding a client link is insufficient.
- Missing and inaccessible conversations must not disclose whether a request
  exists. A request identifier alone must not authorize access.
- Request content must not appear in social feeds, public URLs, logs or audit
  metadata. Operator reads and replies must be audited without copying message
  content into the audit record.
- Successful mutations and their audit records must commit atomically. Requests
  and replies must be bounded and rate limited using the application's shared
  enforcement mechanisms.
- The initial support form must use a recovery credential, without linking a
  conversation to an account or inferring account ownership from message text.
  Support requests must not automatically become a moderation-retention exception.
- No public support copy may disclose the operator's backup strategy or make
  backup guarantees.

## Acceptance

1. Submit a request through a mobile support link and reopen its conversation on
   the web; only the requester can read it through the requester interface.
2. An authorized operator sees the request and replies; the requester can read
   the response and send a follow-up without email.
3. A lost submission response followed by retry creates one request.
4. Foreign, missing, malformed and expired credentials do not reveal content;
   authorization and storage failures fail closed.
5. Empty, oversized or invalid input produces useful correction and keeps the
   draft. Untrusted markup appears as text.
6. Keyboard navigation and large text retain readable fields, errors, submission
   status and reply controls.

## Signed-out access and platform administration

- People must be able to submit requests and read replies without signing in or
  providing an email address. Success must provide a private recovery link.
- Ticket identifiers must be canonical 26-character ULIDs, not sequential or
  auto-incrementing public numbers. Each new ticket must use fresh cryptographic
  randomness for its ULID; the timestamp/identifier is not an access credential.
  Knowing a ticket identifier alone must never allow reading or replying.
- The recovery secret must be generated with cryptographic randomness, stored
  only as a one-way digest on the server, and authorize only its conversation.
  It must not authorize account recovery, sign-in or any account mutation.
- The secret must travel in the URL fragment on a recovery link and in a
  dedicated request header when accessing the API; it must not appear in the
  URL path/query, server logs, notification payloads or operator-facing data.
- Recovery access lasts 30 days from submission; the form must show the expiry
  and explain how to start a new request afterward. A conversation accepts at
  most 100 messages, each at most 5,000 Unicode code points, with a subject of
  at most 160 code points. These bounds must produce localized feedback.
- The requester must be told to keep the link private and save it to read replies.
  Operators must not be able to retrieve the original recovery secret.
- Platform administrators must receive in-app notifications for newly submitted
  support requests and requester follow-ups. Notifications must link to the
  private admin inbox and contain no message text or recovery secret.
- Support inbox access and replies must require a persisted platform-admin grant
  for the stable account identity and a current server authorization decision.
  Path administrators and invitation administration alone must not confer this
  privilege. Revocation must remove access to both tickets and notifications.
- Platform administrators must also receive in-app notifications for newly
  submitted moderation reports and appeals. These notifications must contain no
  report evidence or appeal text and must open only an authorized private record.
  Support and moderation remain distinct domains; notification delivery must not
  grant permission to unrelated content or bypass moderation authorization.
- The owner authorized granting platform administration to the account located
  using the supplied email on October 17. Resolve that address to an unambiguous
  existing account, then bind the grant to its immutable identity. Email must
  never become the runtime authorization rule.

Related: [User policies and support](../safety/user-policies.spec.md),
[Account deletion](../accounts/deletion.spec.md).
