type Interval = { accumulatedSeconds: number; targetSeconds: number; startedAt?: string; endedAt?: string };
type State = { accumulatedSeconds: number; running: boolean; timer?: { startedAt: string }; intervalProgress?: Interval };

export function timerPeriodExpired(state: State, now: number): boolean {
  const end = Date.parse(state.intervalProgress?.endedAt ?? '');
  return Number.isFinite(end) && now >= end;
}

// API totals contain saved activity only. Derive display values without mutating
// that base, so refreshes and Stop receipts cannot count a session twice.
export function liveTimerProgress<T extends State>(state: T, now: number): T {
  const start = state.running ? Date.parse(state.timer?.startedAt ?? '') : NaN;
  const elapsed = Number.isFinite(start) && Number.isFinite(now) ? Math.max(0, Math.floor((now - start) / 1000)) : 0;
  let interval = state.intervalProgress;
  if (timerPeriodExpired(state, now)) interval = undefined;
  else if (interval && Number.isFinite(start) && Number.isFinite(now)) {
    const periodStart = Date.parse(interval.startedAt ?? '');
    const periodEnd = Date.parse(interval.endedAt ?? '');
    // Older servers omit bounds; keep their authoritative saved value rather
    // than guessing the participant's timezone or goal alignment.
    if (Number.isFinite(periodStart) && Number.isFinite(periodEnd) && periodEnd > periodStart) {
      const overlap = Math.max(0, Math.floor((Math.min(now, periodEnd) - Math.max(start, periodStart)) / 1000));
      interval = { ...interval, accumulatedSeconds: interval.accumulatedSeconds + overlap };
    }
  }
  return { ...state, accumulatedSeconds: state.accumulatedSeconds + elapsed, intervalProgress: interval };
}

// Bound period refreshes per path, including during mixed-version deployment.
// Callers still own session and snapshot checks before applying the response.
export function createTimerPeriodRefresher() {
  const attempts = new Map<string, number>();
  return (states: Record<string, State>, now: number, refresh: (pathID: string, snapshot: State) => Promise<void>) => {
    for (const [pathID, snapshot] of Object.entries(states)) {
      if ((!timerPeriodExpired(snapshot, now) && !(snapshot.running && snapshot.intervalProgress && !snapshot.intervalProgress.endedAt)) || (attempts.get(pathID) ?? 0) > now) continue;
      attempts.set(pathID, Infinity);
      void refresh(pathID, snapshot).catch(() => undefined).finally(() => attempts.set(pathID, now + 30_000));
    }
  };
}
