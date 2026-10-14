import type { RunningTimerSurface } from '@hourpaths/client-core';

/** Keep restart controls only for still-accessible Paths in the same account lease. */
export function retainStoppedTimers(
  running: readonly RunningTimerSurface[], previous: readonly RunningTimerSurface[],
  accessiblePaths: readonly string[],
): RunningTimerSurface[] {
  const result = [...running];
  for (const row of previous) {
    if (typeof row.stoppedAt === 'number' && accessiblePaths.includes(row.pathId)
      && !result.some(timer => timer.pathId === row.pathId)) result.push(row);
  }
  return result;
}
