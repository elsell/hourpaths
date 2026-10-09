# Mobile social presentation

Status: Approved for implementation, September 2026

## Purpose and scope

Following and user profiles present an accessible, person-first social activity
history. Authorization and publication order remain authoritative in
[Social feed](../social/feed.spec.md) and [Following](../social/following.spec.md).

## Feed and profiles

- Following must show compact story-style outlined avatars for currently active
  followed people above the completed chronological feed.
- Feed and profile activity must reuse the same post presentation: safe public
  identity and publication time, Path and duration or achievement, followed by
  only authorized reactions and comments. Private activity notes must not appear.
- Tapping an author must open their profile. Profiles must show compact identity,
  bio, follow relationship and counts above that person's newest-first visible
  activity history. Pagination must fetch the actual profile timeline, not filter
  the already loaded Following page.
- Shared palette, spacing, typography, avatar, and engagement components must
  keep alignment and interaction consistent across these destinations. Loading,
  empty, failed, refreshing, and pagination states must remain distinct.

## Live activity viewer

- Tapping an active avatar must open a full-screen viewer. Each page represents
  one currently active Path for that person, on a full-bleed palette background.
- Top position segments must represent the selected person's active Paths only,
  not the total number of people. Right tap advances to that person's next Path,
  then the next person. Left tap reverses this sequence. Before the first page
  it stays put; after the final page it closes.
- Navigation must be manual through left/right tap regions, with a horizontal
  page animation. There must be no visible previous/next buttons, bottom help
  text, or automatic timed advance. A close control must remain available.
- Assistive technology must expose named next/previous/close actions. Reduced
  motion must remove the sliding animation; large text must remain readable.
- Each page must identify the person and Path, display the live session elapsed
  time, and show configured goal progress. Projected progress includes only the
  running session's overlap with the current goal period and is labeled as
  including that session. Server-recorded totals remain authoritative.
- Period rollover, timer disappearance, stale refresh, changed access, and account
  replacement must not keep presenting fabricated live goal progress. Refresh
  or remove the affected page. No goal must be invented for a goal-less Path.
- Appearance is viewer-local: use the established deterministic Path palette
  default when the viewer has no personal saved appearance. Never expose another
  user's personal appearance record as public data.

## Acceptance

- Maya tracking Guitar and Reading shows two segments. Right tap on Guitar
  shows Reading, then Alex's first active Path with Alex's own segment count.
- A profile's second page contains only that profile's currently visible events,
  remains newest-first, and contains no duplicated events after refresh.
- A session spanning a weekly boundary adds only its current-week overlap to
  this week's recorded progress. At the next boundary a refreshed projection is
  required; the old goal must not keep increasing as if still current.

### Inactive tab recovery

A mounted but unfocused native tab must not navigate or enqueue route recovery
when its presentation is temporarily unavailable. Recovery must belong to the
focused route and cancel on blur. Returning from Following or Stats to Home and
opening Settings must remain on Settings during account-presentation refresh or
a device text-size change; an inactive tab must not steal that navigation.
