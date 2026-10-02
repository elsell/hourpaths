import { PathRequestError } from '../../paths/adapters/path-mapping';
import type { TrackingStatus } from '../ports/tracking-status';
import { createSessionApiClient } from '@hourpaths/api-client';
import { OfflineTracking, TrackingReplayWorker, apiTrackingSync } from '@hourpaths/client-core';
import type { SessionController } from '../../session/application/session-controller';
import type { TrackingRuntime } from './durable-path-repository';
import { IndexedDBTrackingStore } from './indexeddb-tracking-store';

export function browserTrackingRuntime(apiURL: string, session: SessionController, store: IndexedDBTrackingStore, connected: () => boolean) {
  let disposed = false;
  let bannerDismissed = false;
  let serverAvailable = true;
  let pending: Promise<TrackingRuntime> | null = null;
  let current: TrackingRuntime | null = null;
  let worker: TrackingReplayWorker | null = null;
  const listeners = new Set<(refreshHome?: boolean) => void>();
  const notify = (refreshHome = false) => { if (!disposed) for (const listener of listeners) listener(refreshHome); };
  const assertAlive = () => { if (disposed || !session.token()) throw new Error('tracking_session_unavailable'); };
  async function initialize(): Promise<TrackingRuntime> {
    assertAlive();
    let owner = session.owner();
    const credential = session.token()!;
    const api = createSessionApiClient(apiURL, () => session.token(), undefined, value => session.reject(value));
    if (!owner) {
      const profile = await api.profile();
      if (!profile.response.ok || !profile.data || !session.bindOwner(credential, profile.data.data.id)) throw new Error('tracking_session_unavailable');
      owner = session.owner();
    }
    if (!owner) throw new Error('tracking_session_unavailable');
    const boundOwner = owner;
    const assertCurrent = () => { assertAlive(); if (session.owner() !== boundOwner) throw new Error('tracking_session_unavailable'); };
    const tracking = new OfflineTracking(store, boundOwner, () => Date.now(), () => crypto.randomUUID());
    const retained = await tracking.snapshot();
    let timeZone = retained.paths[0]?.timeZone;
    // Restoring a locally known account and zone must not wait for a server
    // request that can stall while connectivity is unavailable.
    if (!timeZone) {
      const configured = await api.configuredTimeZone();
      assertCurrent();
      if (configured.response.ok && configured.data) timeZone = configured.data.data.timeZone;
      else throw new Error('tracking_zone_unavailable');
    }
    if (!timeZone) throw new Error('tracking_zone_unavailable');
    new Intl.DateTimeFormat('en', { timeZone });
    assertCurrent();
    const sync = apiTrackingSync(requested => !disposed && session.owner() === requested && session.token()
      ? createSessionApiClient(apiURL, () => session.token(), undefined, value => session.reject(value)) : null);
    worker = new TrackingReplayWorker(() => tracking.replay(sync), (work, delay) => {
      const timer = setTimeout(work, delay); return () => clearTimeout(timer);
    }, () => { assertCurrent(); notify(); });
    current = { owner: boundOwner, timeZone, tracking, assertCurrent, connected,
      reportNetwork: available => {
        assertCurrent();
        if (serverAvailable === available) return;
        serverAvailable = available;
        if (available) bannerDismissed = false;
        notify();
      },
      refreshTimeZone: async () => {
        const configured = await api.configuredTimeZone();
        assertCurrent();
        if (!configured.response.ok || !configured.data) throw new PathRequestError(configured.response.ok ? 502 : configured.response.status);
        new Intl.DateTimeFormat('en', { timeZone: configured.data.data.timeZone });
        return configured.data.data.timeZone;
      }, wake: () => { notify(); void worker?.wake(); } };
    if (retained.operations.length) current.wake();
    return current;
  }
  function runtime(): Promise<TrackingRuntime> {
    if (!pending) pending = initialize().catch(error => { pending = null; throw error; });
    return pending;
  }
  return {
    runtime,
    async snapshot(): Promise<TrackingStatus> {
      const context = await runtime();
      context.assertCurrent();
      const state = await context.tracking.snapshot();
      context.assertCurrent();
      const offline = !connected() || !serverAvailable;
      return { offline, showBanner: offline && !bannerDismissed,
        pending: state.operations.some(operation => !state.corrections.some(value => value.timer.id === operation.timerId)), unavailablePathIds: state.unavailablePaths ?? [],
        corrections: state.corrections.map(value => ({ id: value.timer.id, pathName: state.paths.find(path => path.id === value.timer.pathId)?.name ?? '',
          reviewedStartedAt: value.reviewedStartedAt, startedAt: value.timer.startedAt, endedAt: value.endedAt, timeZone: value.timer.timeZone })),
        notices: state.notices.map(notice => ({ ...notice, pathName: state.paths.find(path => path.id === notice.pathId)?.name })) };
    },
    async correct(id: string, start: string, end: string): Promise<void> {
      const context = await runtime();
      context.assertCurrent();
      await context.tracking.correct(id, start, end);
      context.assertCurrent();
      context.wake();
    },
    dismissBanner(): void { bannerDismissed = true; notify(); },
    async dismissNotice(id: string): Promise<void> {
      const context = await runtime();
      context.assertCurrent();
      await context.tracking.dismissNotice(id);
      context.assertCurrent();
      notify();
    },
    subscribe(listener: (refreshHome?: boolean) => void): () => void { listeners.add(listener); return () => { listeners.delete(listener); }; },
    retry(): void {
      if (disposed) return;
      if (connected()) bannerDismissed = false;
      notify(true);
      if (connected()) { worker?.setPaused(false); void worker?.wake(); }
    },
    dispose(): void { disposed = true; worker?.dispose(); current?.tracking.dispose(); listeners.clear(); void store.close(); },
  };
}
