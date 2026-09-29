import type { TrackingSnapshot } from './path';

export function currentProgress(snapshot: TrackingSnapshot, now: number) {
  const session = snapshot.activeSession;
  const sessionSeconds = session ? Math.max(0, Math.floor((now - session.startedAt) / 1000)) : 0;
  const period = snapshot.period;
  const needsPeriodRefresh = period !== null && now >= period.endsAt;
  const overlapSeconds = session && period
    ? Math.max(0, Math.floor((Math.min(now, period.endsAt) - Math.max(session.startedAt, period.startsAt)) / 1000))
    : 0;
  return {
    totalSeconds: snapshot.savedTotalSeconds + sessionSeconds,
    sessionSeconds,
    periodSeconds: period && !needsPeriodRefresh ? period.savedSeconds + overlapSeconds : null,
    needsPeriodRefresh,
  };
}
