import { manualActivityParticipantNow, reviewedManualActivityInterval, type RetainedActivity } from '@hourpaths/client-core';
import type { ActivityRepository } from '../../history/ports/activity-repository';
import type { ActivityDetail } from '../../history/domain/detail';
import type { TrackingRuntime } from './durable-path-repository';

/** Form writes commit to the account ledger before network replay starts. */
export function durableActivityRepository(remote: ActivityRepository, runtime: () => Promise<TrackingRuntime>, temporary: (failure: unknown) => boolean, now: () => number = Date.now): ActivityRepository {
  const detail = (entry: RetainedActivity, current: TrackingRuntime, name: string): ActivityDetail => ({
    id: entry.id, pathId: entry.pathId, participantId: entry.owner, owned: entry.owner === current.owner,
    pathName: entry.pathName ?? name, startedAt: Date.parse(entry.startedAt), endedAt: Date.parse(entry.endedAt),
    seconds: Math.floor((Date.parse(entry.endedAt) - Date.parse(entry.startedAt)) / 1000), timeZone: entry.timeZone,
    note: entry.note ?? '', version: entry.version ?? 1, editStamp: entry.editStamp, originalStartedAt: entry.startedAt, originalEndedAt: entry.endedAt, createdAt: entry.createdAt, updatedAt: entry.updatedAt,
  });
  return {
    ...remote,
    async defaults(pathId, signal) {
      const current = await runtime(); current.assertCurrent();
      try { const value = await remote.defaults(pathId, signal); current.assertCurrent(); return value; }
      catch (error) {
        if (signal?.aborted || !temporary(error)) throw error;
        const state = await current.tracking.snapshot(); current.assertCurrent();
        const path = state.paths.find(value => value.id === pathId);
        if (!path || state.unavailablePaths?.includes(pathId)) throw error;
        return { pathName: path.name, currentInstant: now(), timeZone: path.timeZone, canTrack: true };
      }
    },
    async detail(pathId, activityId, signal) {
      const current = await runtime(); current.assertCurrent();
      const retained = await current.tracking.localHistory(pathId); current.assertCurrent();
      const local = retained.items.find(value => value.id === activityId);
      const state = await current.tracking.snapshot(); current.assertCurrent();
      const name = state.paths.find(value => value.id === pathId)?.name ?? '';
      if (local?.pending && local.note !== undefined) return detail(local, current, name);
      try {
        const value = await remote.detail(pathId, activityId, signal); current.assertCurrent();
        if (value.owned && value.participantId === current.owner) {
          await current.tracking.retainActivity({ id: value.id, owner: current.owner, pathId, pathName: value.pathName,
            startedAt: value.originalStartedAt ?? new Date(value.startedAt).toISOString(), endedAt: value.originalEndedAt ?? new Date(value.endedAt).toISOString(),
            editStamp: value.editStamp, createdAt: value.createdAt, updatedAt: value.updatedAt,
            timeZone: value.timeZone, note: value.note ?? '', version: value.version });
          current.assertCurrent();
        }
        return value;
      } catch (error) {
        if (signal?.aborted || !temporary(error) || !local || local.note === undefined) throw error;
        return detail(local, current, name);
      }
    },
    async save(review) {
      const current = await runtime(); current.assertCurrent();
      const state = await current.tracking.snapshot(); current.assertCurrent();
      const existing = review.activityId ? (await current.tracking.localHistory(review.pathId)).items.find(value => value.id === review.activityId) : undefined;
      current.assertCurrent();
      const path = state.paths.find(value => value.id === review.pathId);
      if (!path || review.activityId && !existing) throw new Error('activity_not_retained');
      const timeZone = existing?.timeZone ?? path.timeZone;
      const interval = reviewedManualActivityInterval({ localDate: review.input.localDate, localTime: review.input.localTime, durationSeconds: String(review.input.seconds), occurrenceTouched: true }, manualActivityParticipantNow(new Date(now()).toISOString(), timeZone), existing?.startedAt);
      const input = { startedAt: interval.startedAt, durationSeconds: review.input.seconds, note: review.input.note };
      const saved = review.activityId ? await current.tracking.editRecordedActivity(review.activityId, input) : await current.tracking.createManualActivity({ ...input, pathId: review.pathId });
      current.assertCurrent(); current.wake();
      return { id: saved.id, version: saved.version ?? 1 };
    },
  };
}
