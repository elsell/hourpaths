import { manualActivityParticipantNow, OfflineTracking, TrackingReplayWorker, TrackingReplaySuspended, type RetainedActivity, type SessionExchangeCredential, type TrackingSchedule, type TrackingSnapshot, type TrackingStore, type TrackingSync } from '@hourpaths/client-core';
import type { MobileHomeProfile } from '../session-destination';
import type { MobileHomeCache } from './mobile-home-cache';
import { mobileTrackingAdapter } from './mobile-tracking-adapter';

type Credential = SessionExchangeCredential;
class HydrationChanged extends Error { constructor() { super('tracking_hydration_changed'); } }
type Context = {
  owner: string; credential: Credential | null; paused: boolean; tracking: OfflineTracking;
  adapter: ReturnType<typeof mobileTrackingAdapter>; profile: MobileHomeProfile | null;
  refreshing: Promise<MobileHomeProfile> | null;
  refreshWorker: TrackingReplayWorker;
  historyWorker: TrackingReplayWorker;
  historyRefreshing?: Promise<void>;
};

/** Native account orchestration. Framework callbacks publish only a matching
 * account; local restoration never waits for the network or OIDC discovery. */
export function mobileOfflineHome(dependencies: {
  store: TrackingStore; home: MobileHomeCache;
  now(): number; newId(): string; schedule: TrackingSchedule;
  owner(credential: Credential): Promise<string>;
  bind(credential: Credential, owner: string, current: () => boolean): Promise<void>;
  remote(credential: Credential): Promise<MobileHomeProfile>;
  timeZone(credential: Credential): Promise<string>;
  history?(credential: Credential, owner: string): Promise<RetainedActivity[]>;
  sync(owner: string, credential: () => Credential | null): TrackingSync;
  failure?(owner: string, cause: unknown): void | Promise<void>;
  publish(owner: string, profile: MobileHomeProfile, state: TrackingSnapshot, replace: boolean): void;
}) {
  let context: Context | null = null;
  function dispose() { context?.historyWorker.dispose(); context?.refreshWorker.dispose(); context?.adapter.dispose(); context = null; }
  function valid(current: Context) {
    if (context !== current) throw new Error('tracking_session_superseded');
  }
  function adopt(owner: string, credential: Credential | null): Context {
    if (context?.owner === owner) {
      if (context.credential?.token !== credential?.token) context.paused = !credential;
      context.credential = credential; return context;
    }
    dispose();
    const tracking = new OfflineTracking(dependencies.store, owner, dependencies.now, dependencies.newId);
    const next = { owner, credential, paused: !credential, tracking, profile: null, refreshing: null } as Context;
    context = next;
    next.adapter = mobileTrackingAdapter({ owner, tracking, schedule: dependencies.schedule,
      current: () => context === next,
      sync: dependencies.sync(owner, () => context === next && next.credential && Date.parse(next.credential.expiresAt) > dependencies.now() ? next.credential : null),
      changed: async () => {
        await publish(next);
        const state = await tracking.snapshot();
        if (!state.operations.length && !state.activityOperations?.length && context === next && !next.paused) void next.refreshWorker.wake();
      },
    });
    next.refreshWorker = new TrackingReplayWorker(async () => {
      if (context !== next || next.paused) throw new TrackingReplaySuspended();
      try { await refreshContext(next); }
      catch (cause) {
        if (context === next && !(cause instanceof HydrationChanged)) await dependencies.failure?.(owner, cause);
        throw cause;
      }
    }, dependencies.schedule, () => {});
    next.historyWorker = new TrackingReplayWorker(async () => {
      try { await refreshHistory(next); }
      catch (cause) {
        if (context === next && !(cause instanceof HydrationChanged)) await dependencies.failure?.(owner, cause);
        throw cause;
      }
    }, dependencies.schedule, () => {});
    return next;
  }
  async function refreshHistory(current: Context): Promise<void> {
    if (current.historyRefreshing) return current.historyRefreshing;
    const work = (async () => {
      valid(current);
      const credential = current.credential;
      if (!credential || current.paused) throw new TrackingReplaySuspended();
      if (!dependencies.history) return;
      const before = await current.tracking.snapshot();
      const entries = await dependencies.history(credential, current.owner);
      valid(current);
      if (current.credential !== credential || current.paused) throw new TrackingReplaySuspended();
      if (!await current.tracking.retainHistory(entries, before.revision)) throw new HydrationChanged();
    })();
    current.historyRefreshing = work;
    try { await work; } finally { if (current.historyRefreshing === work) current.historyRefreshing = undefined; }
  }
  async function project(current: Context, profile: MobileHomeProfile): Promise<MobileHomeProfile> {
    if (profile.id !== current.owner) throw new Error('tracking_owner_mismatch');
    const timers = { ...profile.timers };
    const snapshot = await current.tracking.snapshot();
    const unavailable = new Set(snapshot.unavailablePaths ?? []);
    for (const id of unavailable) delete timers[id];
    for (const path of snapshot.paths) if (!unavailable.has(path.id)) timers[path.id] = await current.adapter.timer(path.id);
    valid(current);
    return { ...profile, paths: profile.paths.filter(path => !unavailable.has(path.id)), timers };
  }
  async function publish(current: Context, replace = false): Promise<void> {
    valid(current);
    if (!current.profile) return;
    const profile = await project(current, current.profile);
    const snapshot = await current.tracking.snapshot();
    valid(current);
    current.profile = profile;
    dependencies.publish(current.owner, profile, snapshot, replace);
  }
  async function refreshContext(current: Context, guard: () => boolean = () => true): Promise<MobileHomeProfile> {
    if (current.refreshing) return current.refreshing;
    const work = (async () => {
      const revision = await current.adapter.beginHydration();
      const credential = current.credential;
      if (!credential || current.paused) throw new TrackingReplaySuspended();
      const [profile, timeZone] = await Promise.all([dependencies.remote(credential), dependencies.timeZone(credential)]);
      valid(current);
      if (!guard() || current.credential !== credential) throw new Error('tracking_session_superseded');
      if (profile.id !== current.owner) throw new Error('tracking_owner_mismatch');
      if (!await current.adapter.retain(profile, timeZone, revision)) throw new HydrationChanged();
      await current.tracking.confirmOnline();
      await dependencies.home.saveHome({ owner: current.owner, timeZone, profile });
      valid(current);
      current.profile = await project(current, profile);
      await publish(current, true);
      void current.historyWorker.wake();
      return current.profile;
    })();
    current.refreshing = work;
    try { return await work; }
    finally { if (current.refreshing === work) current.refreshing = null; }
  }
  return {
    async restoreRetained(owner: string): Promise<{ profile: MobileHomeProfile; state: TrackingSnapshot } | null> {
      const retained = await dependencies.home.readHome(owner);
      if (!retained) return null;
      const current = adopt(owner, null);
      current.paused = true;
      current.adapter.pause(); current.refreshWorker.setPaused(true); current.historyWorker.setPaused(true);
      current.profile = await project(current, retained.profile);
      return { profile: current.profile, state: await current.tracking.snapshot() };
    },
    async retained(): Promise<{ profile: MobileHomeProfile; state: TrackingSnapshot } | null> {
      if (!context?.profile) return null;
      const current = context;
      const profile = await project(current, current.profile!);
      const state = await current.tracking.snapshot();
      valid(current);
      return { profile, state };
    },
    async cached(credential: Credential): Promise<boolean> {
      return !!credential.ownerId && !!await dependencies.home.readHome(credential.ownerId);
    },
    async load(credential: Credential, guard: () => boolean): Promise<{ profile: MobileHomeProfile; retained: boolean }> {
      if (Date.parse(credential.expiresAt) <= dependencies.now() || credential.nextAction !== 'home') throw new Error('tracking_session_unavailable');
      // A new interactive session must prove its owner before using retained data.
      // Only a previously persisted owner binding permits network-free entry.
      const owner = credential.ownerId ?? await dependencies.owner(credential);
      if (typeof owner !== 'string' || !owner || owner.trim() !== owner || owner.length > 128) throw new Error('tracking_owner_invalid');
      if (!guard()) throw new Error('tracking_session_superseded');
      if (!credential.ownerId) await dependencies.bind(credential, owner, guard);
      if (!guard() || credential.ownerId !== owner) throw new Error('tracking_owner_mismatch');
      const current = adopt(owner, credential);
      const retained = await dependencies.home.readHome(owner);
      valid(current);
      if (!guard()) throw new Error('tracking_session_superseded');
      if (retained) {
        current.profile = await project(current, retained.profile);
        return { profile: current.profile, retained: true };
      }
      return { profile: await refreshContext(current, guard), retained: false };
    },
    async history(pathId: string) {
      if (!context || !context.credential || context.paused || Date.parse(context.credential.expiresAt) <= dependencies.now()) throw new Error('tracking_session_unavailable');
      const current = context;
      const result = await current.tracking.localHistory(pathId);
      valid(current);
      return result;
    },
    refreshHistory(): Promise<void> {
      if (!context) return Promise.reject(new Error('tracking_session_unavailable'));
      return refreshHistory(context);
    },
    refresh(): Promise<MobileHomeProfile> {
      if (!context) return Promise.reject(new Error('tracking_session_unavailable'));
      return refreshContext(context);
    },
    async activityDefaults(pathId: string) {
      if (!context || !context.credential || context.paused || Date.parse(context.credential.expiresAt) <= dependencies.now()) throw new Error('tracking_session_unavailable');
      const current = context;
      const state = await current.tracking.snapshot(); valid(current);
      const path = state.paths.find(value => value.id === pathId);
      if (!path || state.unavailablePaths?.includes(pathId)) throw new Error('tracking_path_unavailable');
      const local = manualActivityParticipantNow(new Date(dependencies.now()).toISOString(), path.timeZone);
      return { currentInstant: local.currentInstant, timeZone: path.timeZone, localDate: local.localDate, localStartTime: local.localTime };
    },
    async retainActivity(entry: RetainedActivity) {
      if (!context || !context.credential || context.paused || Date.parse(context.credential.expiresAt) <= dependencies.now()) throw new Error('tracking_session_unavailable');
      const current = context;
      await current.tracking.retainActivity(entry); valid(current);
    },
    async saveActivity(pathId: string, activityId: string | null, input: { startedAt: string; durationSeconds: number; note: string }) {
      if (!context || !context.credential || context.paused || Date.parse(context.credential.expiresAt) <= dependencies.now()) throw new Error('tracking_session_unavailable');
      return context.adapter.saveActivity(pathId, activityId, input);
    },
    async start(pathId: string) {
      if (!context || !context.credential || context.paused || Date.parse(context.credential.expiresAt) <= dependencies.now()) throw new Error('tracking_session_unavailable');
      return context.adapter.start(pathId);
    },
    async stop(pathId: string, timerId: string) {
      if (!context) throw new Error('tracking_session_unavailable');
      return context.adapter.stop(pathId, timerId);
    },
    async correct(timerId: string, startedAt: string, endedAt: string): Promise<void> {
      if (!context || !context.credential || context.paused || Date.parse(context.credential.expiresAt) <= dependencies.now()) throw new Error('tracking_session_unavailable');
      const current = context;
      await current.tracking.correct(timerId, startedAt, endedAt);
      await publish(current);
      current.adapter.wake();
    },
    async dismissOfflineBanner(): Promise<void> {
      if (!context) throw new Error('tracking_session_unavailable');
      const current = context;
      await current.tracking.dismissOfflineBanner();
      await publish(current);
    },
    async dismissNotice(id: string): Promise<void> {
      if (!context) throw new Error('tracking_session_unavailable');
      const current = context;
      await current.tracking.dismissNotice(id);
      await publish(current);
    },
    async present(): Promise<void> { if (context) await publish(context); },
    wake(): void { if (context && !context.paused) { context.adapter.wake(); context.refreshWorker.setPaused(false); context.historyWorker.setPaused(false); void context.refreshWorker.wake(); void context.historyWorker.wake(); } },
    pause(): void { if (context) { context.paused = true; context.adapter.pause(); context.refreshWorker.setPaused(true); context.historyWorker.setPaused(true); } },
    dispose,
  };
}
