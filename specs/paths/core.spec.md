# Paths: Core Product Behavior

Status: Approved for implementation

## Purpose

Define the central product boundaries and vocabulary for paths. This spec is
authoritative for the relationship between a path and its participating users.

## Terminology

- **Path:** A trackable, time-based skill, practice, or pursuit in which one or
  more users may participate.
- **Participant progress:** The time and progress attributed to one participant
  within a path.

Path role terminology is defined in [Path Roles](roles.spec.md#terminology).

## Product boundaries

- The user-facing term for a tracked pursuit must be **Path**.
- The product must track time invested in skills, practices, and pursuits.
- Every path must be time-based.
- Generic binary "did or did not" habits are outside the product scope.

## Participation

- A path must support one or more participants.
- A user must be able to track a path without adding another participant.
- Adding another participant must extend the existing path; it must not create or
  link a second path.
- Time tracked against a path must be attributed to the participant who tracked
  it.
- Each participant's time and progress must remain distinct within the path.
- Participants' progress within the same path must be available for comparison.
- One participant must not be able to record time or progress on behalf of
  another participant unless a later spec explicitly defines that behavior.
- The main participant comparison must expose aggregate progress indicators.
- Authorized Path members must also be able to inspect a current participant's
  individual sessions through a secondary participant detail experience; those
  sessions must not be listed in the main comparison.
- Session visibility and interaction behavior are defined in
  [Path Details](../experience/path-details.spec.md).
- Voluntary departure behavior is defined in
  [Path Membership](membership.spec.md#leaving-a-path).

### Example

Alice creates a guitar-practice path and Bob participates in it. Alice's tracked
time is attributed to Alice and Bob's tracked time is attributed to Bob. They can
compare their progress, but both are participating in the same guitar-practice
path.
