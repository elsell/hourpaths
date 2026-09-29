# Studio web rollout

The Studio replacement is available at `/studio` after signing in through the
current application. This first delivery covers Paths: live timers and totals,
creation, renaming, reviewed goal changes, personal appearance, pinning, manual
order, and connected personal history. Preferences use the same account records
as mobile.

The current application remains available through **Current app** while
Following, Stats, Settings, and the remaining account and Path lifecycle screens
are migrated. Do not remove that entry point until the approved cutover criteria
in [the Studio specification](../specs/experience/web-studio.spec.md) pass.

Local development uses the existing web command and runtime configuration:
`pnpm --dir apps/web dev`. Open `/studio` in the same browser session used to sign
in. A missing or expired session returns the sign-in entry; transient request
failures retain a valid credential and expose retry controls.

The approved images are in `docs/design/web-studio/`. The timeline correction is
normative: its vertical line continues across day boundaries. Following must not
include the people-you-follow sidebar shown in its early image.
