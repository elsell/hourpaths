import { projectTrackingState, type TrackingSnapshot, type TrackingSummary } from './offline-tracking';

export type NativeGoalPeriod = NonNullable<TrackingSummary['period']>;
const maximumProjectionWindow = 8 * 60 * 60 * 1000;

/** Local numeric projections, not remote commands. A native refresh can choose
 * the current period without restoring an old server timer or old-period total. */
export function nativeGoalPeriodSchedule(
  state: TrackingSnapshot, owner: string, pathId: string, now: number, expiresAt: number,
): NativeGoalPeriod[] {
  if (!owner || state.owner !== owner) throw new Error('tracking_owner_mismatch');
  if (!Number.isFinite(now) || !Number.isFinite(expiresAt)) throw new Error('tracking_native_clock_invalid');
  const end = Math.min(expiresAt, now + maximumProjectionWindow);
  const periods: NativeGoalPeriod[] = [];
  let cursor = now;
  while (cursor < end) {
    const period = projectTrackingState(state, pathId, cursor)?.period;
    // Older retained clients may have a summary but no calendar definition.
    // Such a summary cannot authorize extrapolation beyond its actual interval.
    if (!period || period.startsAt > cursor || period.endsAt <= cursor) break;
    if (![period.startsAt, period.endsAt, period.savedSeconds, period.targetSeconds].every(Number.isFinite)
      || period.savedSeconds < 0 || period.targetSeconds <= 0) throw new Error('tracking_goal_invalid');
    if (periods.length >= 32) throw new Error('tracking_native_period_limit');
    periods.push({ ...period });
    cursor = period.endsAt;
  }
  return periods;
}
