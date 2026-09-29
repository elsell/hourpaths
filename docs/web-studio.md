# Studio web rollout

The Studio replacement is available at `/studio` after signing in through the
current application. Paths includes live timers and totals,
creation, renaming, reviewed goal changes, personal appearance, pinning, manual
order, and connected personal history. Preferences use the same account records
as mobile.

Following is available at `/studio/following`, with active Path rings, a full-screen
live viewer, connected chronological activity, comments, multiple emoji reactions,
and participant rosters. `/studio/people` provides discovery and follow requests;
`/studio/profile/<username>` shows identity, relationship, authorized Path counts,
live Paths, and activity. These routes use the same authoritative social API as
mobile. Public profile images load anonymously without referrers.

Stats is available at `/studio/stats`. Date presets, adjacent periods, and
multiple Path selection use server-calculated personal recorded time. The chart
opens at recent values, the calendar defaults to a seven-row contribution grid,
and distributions include accessible recorded durations. Refresh preserves manual
chart exploration.

The current application remains available through **Current app** while
Settings and the remaining account and Path lifecycle screens are migrated. Do not remove that entry point until the approved cutover criteria
in [the Studio specification](../specs/experience/web-studio.spec.md) pass.

Local development uses the existing web command and runtime configuration:
`pnpm --dir apps/web dev`. Open `/studio` in the same browser session used to sign
in. A missing or expired session returns the sign-in entry; transient request
failures retain a valid credential and expose retry controls.

The approved images are in `docs/design/web-studio/`. The timeline correction is
normative: its vertical line continues across day boundaries. Following must not
include the people-you-follow sidebar shown in its early image.
