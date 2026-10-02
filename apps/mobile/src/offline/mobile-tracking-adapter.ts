import type { TimerState } from '@hourpaths/api-client';
import { OfflineTracking, TrackingReplayWorker, effectivePathCapabilities, type TrackingSchedule, type TrackingSync } from '@hourpaths/client-core';
import type { MobileHomeProfile } from '../session-destination';

/** Maps native presentation values at the edge; the durable application service
 * remains independent of generated API contracts and React Native. */
export function mobileTrackingAdapter(dependencies: {
  owner: string;
  tracking: OfflineTracking;
  sync: TrackingSync;
  schedule: TrackingSchedule;
  current(): boolean;
  changed(): void | Promise<void>;
}) {
  let disposed = false;
  const assertCurrent = () => {
    if (disposed || !dependencies.current()) throw new Error('tracking_session_superseded');
  };
  const worker = new TrackingReplayWorker(async () => {
    assertCurrent();
    await dependencies.tracking.replay(dependencies.sync);
  }, dependencies.schedule, async () => {
    assertCurrent();
    await dependencies.changed();
  });
  async function timer(pathId: string): Promise<TimerState & { pending: boolean }> {
    assertCurrent();
    const value = await dependencies.tracking.tracking(pathId);
    assertCurrent();
    if (!value) throw new Error('tracking_snapshot_unavailable');
    return {
      accumulatedSeconds: value.savedTotalSeconds,
      running: value.activeSession !== null,
      pending: value.pending,
      ...(value.activeSession ? { timer: {
        id: value.activeSession.id, pathId,
        startedAt: value.activeSession.originalStartedAt, occurrenceTimeZone: value.activeSession.timeZone,
      } } : {}),
      ...(value.period ? { intervalProgress: {
        accumulatedSeconds: value.period.savedSeconds, targetSeconds: value.period.targetSeconds,
        startedAt: new Date(value.period.startsAt).toISOString(), endedAt: new Date(value.period.endsAt).toISOString(),
      } } : {}),
    };
  }
  async function changed(pathId: string) {
    const value = await timer(pathId);
    await dependencies.changed();
    assertCurrent();
    void worker.wake();
    return value;
  }
  return {
    async beginHydration(): Promise<number> {
      assertCurrent();
      return (await dependencies.tracking.snapshot()).revision;
    },
    async retain(profile: MobileHomeProfile, timeZone: string, revision: number): Promise<boolean> {
      assertCurrent();
      if (profile.id !== dependencies.owner) throw new Error('tracking_owner_mismatch');
      const paths = profile.paths.filter(path => !path.archivedAt && effectivePathCapabilities(path).trackTime);
      const entries = paths.map(path => {
        const value = profile.timers[path.id];
        if (!value || value.running !== !!value.timer || value.timer && value.timer.pathId !== path.id) {
          throw new Error('tracking_snapshot_invalid');
        }
        const period = value.intervalProgress;
        if (period && (!period.startedAt || !period.endedAt)) throw new Error('tracking_period_invalid');
        return {
          pathId: path.id,
          summary: { savedTotalSeconds: value.accumulatedSeconds, period: period ? {
            savedSeconds: period.accumulatedSeconds, targetSeconds: period.targetSeconds,
            startsAt: Date.parse(period.startedAt!), endsAt: Date.parse(period.endedAt!),
          } : null },
          timer: value.timer ? { id: value.timer.id, pathId: path.id, startedAt: value.timer.startedAt, timeZone: value.timer.occurrenceTimeZone } : null,
        };
      });
      const retained = await dependencies.tracking.retainHome(paths.map(path => ({ id: path.id, name: path.name, timeZone, goal: path.intervalGoal ? { targetSeconds: path.intervalGoal.targetSeconds, recurrence: path.intervalGoal.recurrence, alignment: path.intervalGoal.alignment } : null })), entries, revision);
      assertCurrent();
      return retained;
    },
    timer,
    async start(pathId: string) {
      assertCurrent();
      await timer(pathId);
      await dependencies.tracking.start(pathId);
      assertCurrent();
      return changed(pathId);
    },
    async stop(pathId: string, timerId: string) {
      assertCurrent();
      const active = (await dependencies.tracking.snapshot()).timers.find(value => value.id === timerId && value.pathId === pathId);
      if (!active) throw new Error('tracking_timer_unavailable');
      await dependencies.tracking.stop(timerId);
      assertCurrent();
      return changed(pathId);
    },
    wake(): void { if (!disposed && dependencies.current()) { worker.setPaused(false); void worker.wake(); } },
    pause(): void { worker.setPaused(true); },
    dispose(): void { disposed = true; worker.dispose(); dependencies.tracking.dispose(); },
  };
}
