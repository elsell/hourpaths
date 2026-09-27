import type { PathAppearance } from '@hourpaths/client-core';

export type LiveActivityPage = {
  timerId: string;
  personId: string;
  displayName: string;
  username: string;
  profilePictureURL?: string;
  pathId: string;
  pathName: string;
  startedAt: string;
  appearance: PathAppearance;
  overallGoal?: { recordedSeconds: number; targetSeconds: number; label: string };
  goal?: {
    recordedSeconds: number;
    targetSeconds: number;
    intervalStart?: string;
    intervalEnd?: string;
    label: string;
  };
};

/** null means the viewer has reached the end or its selected activity vanished. */
export function adjacentLiveActivity(pages: readonly Pick<LiveActivityPage, 'timerId'>[], selected: string, direction: -1 | 1): string | null {
  const index = pages.findIndex(page => page.timerId === selected);
  if (index < 0 || index + direction >= pages.length) return null;
  return pages[Math.max(0, index + direction)]?.timerId ?? null;
}

export function projectLiveActivity(page: Pick<LiveActivityPage, 'startedAt' | 'goal'>, now: number) {
  const started = Date.parse(page.startedAt);
  const sessionSeconds = Number.isFinite(started) ? Math.max(0, Math.floor((now - started) / 1000)) : 0;
  const goal = page.goal;
  if (!goal) return { sessionSeconds, goalSeconds: undefined, goalExpired: false };
  const lower = goal.intervalStart ? Date.parse(goal.intervalStart) : started;
  const upper = goal.intervalEnd ? Date.parse(goal.intervalEnd) : Number.POSITIVE_INFINITY;
  const overlap = Number.isFinite(started) && Number.isFinite(lower) && !Number.isNaN(upper)
    ? Math.max(0, Math.floor((Math.min(now, upper) - Math.max(started, lower)) / 1000)) : 0;
  return { sessionSeconds, goalSeconds: Math.max(0, goal.recordedSeconds) + overlap, goalExpired: now >= upper };
}
