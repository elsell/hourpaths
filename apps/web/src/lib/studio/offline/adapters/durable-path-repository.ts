import { PathRequestError } from '../../paths/adapters/path-mapping';
import type { OfflineTracking } from '@hourpaths/client-core';
import type { PathRepository } from '../../paths/ports/path-repository';
import type { HomeCache } from '../ports/home-cache';
import type { Path, TrackingSnapshot } from '../../paths/domain/path';

export interface TrackingRuntime {
  owner: string;
  timeZone: string;
  tracking: OfflineTracking;
  assertCurrent(): void;
  wake(): void;
  connected?(): boolean;
  reportNetwork?(available: boolean): void;
  refreshTimeZone?(): Promise<string>;
}

/** Existing presentation keeps its PathRepository port. Durable commands resolve
 * after local commit; network delivery runs independently through replay. */
export function durablePathRepository(
  remote: PathRepository,
  cache: HomeCache,
  runtime: () => Promise<TrackingRuntime>,
  temporary: (failure: unknown) => boolean,
): PathRepository {
  let reads: Promise<unknown> = Promise.resolve();
  const prefetchedTracking = new Set<string>(), prefetchedAppearance = new Set<string>();
  const ownedKey = (owner: string, id: string) => `${owner}\u0000${id}`;
  async function mapPaths<T>(paths: readonly Path[], read: (path: Path) => Promise<T>): Promise<T[]> {
    const result = new Array<T>(paths.length);
    let next = 0, failed = false;
    await Promise.all(Array.from({ length: Math.min(4, paths.length) }, async () => {
      while (!failed) {
        const index = next++;
        if (index >= paths.length) return;
        try { result[index] = await read(paths[index]); }
        catch (error) { failed = true; throw error; }
      }
    }));
    return result;
  }
  const requireView = async (current: TrackingRuntime, pathId: string): Promise<TrackingSnapshot> => {
    const value = await current.tracking.tracking(pathId);
    current.assertCurrent();
    if (!value) throw new Error('tracking_snapshot_unavailable');
    return value;
  };
  const tracking = (pathId: string, signal?: AbortSignal): Promise<TrackingSnapshot> => {
    // Summary and timer hydration share one revision with local commands.
    const work = reads.catch(() => undefined).then(async () => {
      const current = await runtime();
      current.assertCurrent();
      const before = await current.tracking.snapshot();
      if (prefetchedTracking.delete(ownedKey(current.owner, pathId))) return requireView(current, pathId);
      if (current.connected?.() === false && Object.hasOwn(before.summaries, pathId)) return requireView(current, pathId);
      if (before.operations.some(value => value.pathId === pathId) || before.activityOperations?.some(value => value.activity.pathId === pathId)) return requireView(current, pathId);
      try {
        const value = await remote.tracking(pathId, signal);
        current.assertCurrent();
        await current.tracking.retainTracking(pathId, value, value.activeSession ? {
          id: value.activeSession.id, pathId, startedAt: value.activeSession.originalStartedAt ?? new Date(value.activeSession.startedAt).toISOString(), timeZone: value.activeSession.timeZone ?? current.timeZone,
        } : null, before.revision);
      } catch (error) {
        if (!temporary(error)) throw error;
      }
      return requireView(current, pathId);
    });
    reads = work;
    return work;
  };
  return {
    ...remote,
    async list(archived, signal) {
      if (archived) return remote.list(archived, signal);
      const current = await runtime();
      current.assertCurrent();
      async function retainedPaths(): Promise<readonly Path[] | null> {
        const retained = await cache.readHome(current.owner);
        current.assertCurrent();
        if (!retained) return null;
        const unavailable = (await current.tracking.snapshot()).unavailablePaths ?? [];
        return retained.paths.filter(path => !unavailable.includes(path.id));
      }
      if (current.connected?.() === false) {
        const retained = await retainedPaths();
        if (retained) return retained;
      }
      async function hydrate(retries: number): Promise<readonly Path[]> {
        let remoteComplete = false;
        try {
          const before = await current.tracking.snapshot();
          const paths = await remote.list(false, signal);
          if (current.refreshTimeZone) current.timeZone = await current.refreshTimeZone();
          current.assertCurrent();
          signal?.throwIfAborted();
          const participating = paths.filter(path => path.canTrack && !path.archived);
          const values = await mapPaths(participating, async path => {
            const [summary, appearance] = await Promise.all([remote.tracking(path.id, signal), remote.appearance(path.id, signal)]);
            current.assertCurrent();
            return { path, summary, appearance };
          });
          remoteComplete = true;
          signal?.throwIfAborted();
          const retained = await current.tracking.retainHome(participating.map(path => ({ id: path.id, name: path.name, timeZone: current.timeZone, goal: path.goal })),
            values.map(({ path, summary }) => ({ pathId: path.id, summary, timer: summary.activeSession ? {
              id: summary.activeSession.id, pathId: path.id, startedAt: summary.activeSession.originalStartedAt ?? new Date(summary.activeSession.startedAt).toISOString(),
              timeZone: summary.activeSession.timeZone ?? current.timeZone,
            } : null })), before.revision);
          if (!retained) {
            signal?.throwIfAborted();
            if (retries > 0) return hydrate(retries - 1);
            throw new PathRequestError(503);
          }
          current.assertCurrent();
          await cache.saveHome(current.owner, participating, Object.fromEntries(values.map(value => [value.path.id, value.appearance])));
          current.assertCurrent();
          current.reportNetwork?.(true);
          for (const path of participating) {
            prefetchedTracking.add(ownedKey(current.owner, path.id));
            prefetchedAppearance.add(ownedKey(current.owner, path.id));
          }
          const unavailable = (await current.tracking.snapshot()).unavailablePaths ?? [];
          return paths.filter(path => !unavailable.includes(path.id));
        } catch (error) {
          if (signal?.aborted || !temporary(error)) throw error;
          if (!remoteComplete) current.reportNetwork?.(false);
          const retained = await retainedPaths();
          if (!retained) throw error;
          return retained;
        }
      }
      return hydrate(2);
    },
    async read(pathId, signal) {
      try { return await remote.read(pathId, signal); }
      catch (error) {
        if (!temporary(error)) throw error;
        const current = await runtime();
        const path = (await cache.readHome(current.owner))?.paths.find(value => value.id === pathId);
        current.assertCurrent();
        if (!path || (await current.tracking.snapshot()).unavailablePaths?.includes(pathId)) throw error;
        return path;
      }
    },
    async appearance(pathId, signal) {
      const current = await runtime();
      if (prefetchedAppearance.delete(ownedKey(current.owner, pathId)) || current.connected?.() === false) {
        const retained = await cache.readHome(current.owner);
        current.assertCurrent();
        if (retained && Object.hasOwn(retained.appearances, pathId)) return retained.appearances[pathId];
      }
      try {
        const value = await remote.appearance(pathId, signal);
        current.assertCurrent();
        await cache.saveAppearance(current.owner, pathId, value);
        return value;
      } catch (error) {
        if (!temporary(error)) throw error;
        const retained = await cache.readHome(current.owner);
        current.assertCurrent();
        const value = retained && Object.hasOwn(retained.appearances, pathId) ? retained.appearances[pathId] : undefined;
        if (!value) throw error;
        return value;
      }
    },
    tracking,
    async start(pathId) {
      const current = await runtime();
      current.assertCurrent();
      await requireView(current, pathId);
      await current.tracking.start(pathId);
      current.assertCurrent();
      current.wake();
      return requireView(current, pathId);
    },
    async stop(pathId, timerId) {
      const current = await runtime();
      current.assertCurrent();
      const existing = (await current.tracking.snapshot()).timers.find(timer => timer.id === timerId && timer.pathId === pathId);
      if (!existing) throw new Error('tracking_timer_unavailable');
      await current.tracking.stop(timerId);
      current.assertCurrent();
      current.wake();
      return requireView(current, pathId);
    },
  };
}
