import type { SessionPath, TimerState } from '@hourpaths/api-client';
import { effectivePathCapabilities } from '@hourpaths/client-core';

export type HomePathSections<P extends SessionPath = SessionPath> = {
  active: P[];
  ordinary: P[];
};

function validTimerInstant(value: string): number | null {
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.\d+)?(?:Z|[+-](\d{2}):(\d{2}))$/.exec(value);
  if (!match) return null;
  const [, yearText, monthText, dayText, hourText, minuteText, secondText, offsetHourText, offsetMinuteText] = match;
  const year = Number(yearText);
  const month = Number(monthText);
  const day = Number(dayText);
  const hour = Number(hourText);
  const minute = Number(minuteText);
  const second = Number(secondText);
  const offsetHour = Number(offsetHourText ?? 0);
  const offsetMinute = Number(offsetMinuteText ?? 0);
  const leapYear = year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0);
  const daysInMonth = [31, leapYear ? 29 : 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31][month - 1] ?? 0;
  if (year === 0 || day < 1 || day > daysInMonth || hour > 23 || minute > 59 || second > 59 ||
      offsetHour > 23 || offsetMinute > 59) return null;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

export function homePathSections<P extends SessionPath>(
  paths: readonly P[],
  timers: Readonly<Record<string, TimerState | undefined>>,
): HomePathSections<P> {
  const active: Array<{ path: P; sourceIndex: number; startedAt: number }> = [];
  const ordinary: P[] = [];

  paths.forEach((path, sourceIndex) => {
    const state = timers[path.id];
    const startedAt = state?.timer ? validTimerInstant(state.timer.startedAt) : null;
    const isActive = effectivePathCapabilities(path).trackTime &&
      state?.running === true &&
      state.timer?.pathId === path.id &&
      startedAt !== null;

    if (isActive) {
      active.push({ path, sourceIndex, startedAt });
    } else {
      ordinary.push(path);
    }
  });

  active.sort((left, right) => right.startedAt - left.startedAt || left.sourceIndex - right.sourceIndex);
  return {
    active: active.map(({ path }) => path),
    ordinary,
  };
}
