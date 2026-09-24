# API Collection and Abuse-Control Behavior

Status: Approved for implementation

## Purpose

Define consistent externally observable behavior for unbounded collections and
protecting public or user-driven actions from resource abuse.

## Pagination

- Every collection that can grow without a small fixed product bound must use
  cursor-based pagination at its service boundary.
- This includes feeds, comments, notifications, followers, following, profile
  search, Path membership, invitations, activity history, and moderation work.
- A collection request must default to 25 items and must not accept a requested
  page size greater than 100.
- Every page must use the collection's authoritative stable ordering and a
  deterministic tie breaker.
- A cursor must be opaque to clients and must not grant access to an item the
  requesting user is no longer authorized to see.
- Loading another page must not duplicate an item already returned in the same
  traversal.
- A failed subsequent-page request must preserve the items already shown and
  provide a retry action.
- Reaching the end of a collection must be distinguishable from a temporary
  loading failure.

## Abuse controls

- Public and user-driven operations must be protected by configurable limits
  appropriate to the actor, source, action, and affected resource.
- At minimum, profile search, follows, invitations, comments, reactions,
  reports, and image uploads must have feature-level abuse controls in addition
  to any deployment-wide request limit.
- Limits must be enforceable per authenticated user and, where appropriate, per
  network source.
- A limited request must not partially apply its intended mutation.
- The user must receive a concise retryable response without internal security
  details.
- When a reliable retry time is available, the response must communicate it to
  the client.
- Operational limits must remain configurable and must not become public
  product promises unless another specification intentionally defines a
  user-facing quota.
- Product-semantic limits, such as nudge frequency, must remain independently
  enforced even when general abuse limits would allow more requests.
