import { OfflineTracking, type TrackingStore } from '@hourpaths/client-core';
import type { RetainedTimers, RetainedTimersSnapshot } from '../ports/retained-timers';

/** Deliberately exposes no Start, replay, HTTP, or account-administration port. */
export function retainedTimers(owner: string, store: TrackingStore, current: () => boolean, now: () => number, id: () => string): RetainedTimers & { dispose(): void } {
  const tracking = new OfflineTracking(store, owner, now, id);
  let disposed = false;
  const valid = () => !disposed && current();
  const assertCurrent = () => { if (!valid()) throw new Error('retained_account_changed'); };
  const snapshot = async (): Promise<RetainedTimersSnapshot> => {
    assertCurrent();
    const state = await tracking.snapshot();
    assertCurrent();
    return { pending: state.operations.length > 0 || Boolean(state.activityOperations?.length), timers: state.timers.map(timer => ({
      id: timer.id, name: state.paths.find(path => path.id === timer.pathId)?.name ?? '', startedAt: Date.parse(timer.startedAt),
    })) };
  };
  return {
    valid, snapshot,
    async stop(timerId) {
      assertCurrent();
      await tracking.stop(timerId);
      return snapshot();
    },
    dispose() { disposed = true; tracking.dispose(); },
  };
}
