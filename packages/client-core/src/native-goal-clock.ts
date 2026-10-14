type Period = { accumulatedSeconds: number; targetSeconds: number; startedAt?: string; endedAt?: string };

/** Native count-up bounds include saved time and only this period's overlap.
 * A bounded clock freezes at the explicit period end until reconciliation. */
export function nativeGoalClock(period: Period | undefined, sessionStart: number, now: number) {
  if (!period) return undefined;
  const start = Date.parse(period.startedAt ?? ''), end = Date.parse(period.endedAt ?? '');
  if (![start, end, sessionStart, now, period.accumulatedSeconds, period.targetSeconds].every(Number.isFinite)
    || end <= start || now < start || now >= end || sessionStart > now
    || period.accumulatedSeconds < 0 || period.targetSeconds <= 0) return undefined;
  const lower = Math.max(start, sessionStart) - period.accumulatedSeconds * 1000;
  const targetEnd = lower + period.targetSeconds * 1000;
  return {
    lower, upper: end,
    fraction: Math.min(1, Math.max(0, (now - lower) / (period.targetSeconds * 1000))),
    // Do not pretend an unreachable goal is complete at its period boundary.
    progressUpper: targetEnd <= end ? targetEnd : undefined,
  };
}
