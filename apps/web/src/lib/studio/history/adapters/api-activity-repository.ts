import { createSessionApiClient, type ActivityDetail as APIDetail, type ActivityRevision as APIRevision } from '@hourpaths/api-client';
import { ActivityFailure } from '../domain/detail';
import type { ActivitySnapshot } from '../domain/detail';
import type { ActivityRepository } from '../ports/activity-repository';
function required<T>(result: { data?: { data: T }; response: Response }): T {
  if (!result.response.ok || !result.data) throw new ActivityFailure(result.response.status >= 500 || result.response.status === 429);
  return result.data.data;
}
function snapshot(value: APIDetail['activity'], version: number, pathId: string, activityId: string, owner: string): ActivitySnapshot {
  const startedAt = Date.parse(value.startedAt), endedAt = Date.parse(value.endedAt);
  if (value.id !== activityId || value.pathId !== pathId || !value.participantId || !Number.isFinite(startedAt) || !Number.isFinite(endedAt) || !Number.isSafeInteger(value.durationSeconds) || value.durationSeconds <= 0 || (endedAt - startedAt) / 1000 !== value.durationSeconds || !Number.isSafeInteger(version) || version < 1) throw new Error('activity_invalid');
  new Intl.DateTimeFormat('en', { timeZone: value.occurrenceTimeZone }).format(startedAt);
  return { editStamp: value.editOrder ? { authoredAt: value.editOrder.authoredAt, counter: value.editOrder.counter } : undefined, originalStartedAt: value.startedAt, originalEndedAt: value.endedAt, createdAt: value.createdAt, updatedAt: value.updatedAt, id: activityId, pathId, participantId: value.participantId, startedAt, endedAt, seconds: value.durationSeconds, timeZone: value.occurrenceTimeZone, version, note: value.participantId === owner ? value.note ?? null : null };
}
export function apiActivityRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): ActivityRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  return {
    async defaults(pathId, signal) {
      const api = client(signal);
      const path = required(await api.path(pathId));
      const defaults = required(await api.manualActivityDefaults(pathId));
      const currentInstant = Date.parse(defaults.currentInstant);
      if (path.id !== pathId || !path.name || !Number.isFinite(currentInstant)) throw new Error('activity_defaults_invalid');
      new Intl.DateTimeFormat('en', { timeZone: defaults.timeZone }).format(currentInstant);
      return { pathName: path.name, currentInstant, timeZone: defaults.timeZone, canTrack: path.capabilities.trackTime };
    },
    async save(review) {
      const body = { localDate: review.input.localDate, localStartTime: review.input.localTime, durationSeconds: review.input.seconds, note: review.input.note };
      const result = required(await (review.activityId ? client().updateActivity(review.pathId, review.activityId, body, review.operationId) : client().createManualActivity(review.pathId, body, review.operationId)));
      if (!result.activity.id || result.activity.pathId !== review.pathId || (review.activityId && result.activity.id !== review.activityId) || !Number.isSafeInteger(result.version) || result.version < 1) throw new Error('activity_save_invalid');
      return { id: result.activity.id, version: result.version };
    },
    async detail(pathId, activityId, signal) {
      const api = client(signal);
      const profile = required(await api.profile());
      const path = required(await api.path(pathId));
      if (!profile.id || path.id !== pathId || !path.name) throw new Error('activity_identity_invalid');
      const dto = required(await api.activity(pathId, activityId));
      return { ...snapshot(dto.activity, dto.version, pathId, activityId, profile.id), pathName: path.name, owned: dto.activity.participantId === profile.id };
    },
    async revisions(pathId, activityId, cursor, signal) {
      const api = client(signal);
      const profile = required(await api.profile());
      if (!profile.id) throw new Error('activity_identity_invalid');
      const response = await api.activityRevisions(pathId, activityId, cursor ?? undefined);
      const items = required(response).map((value: APIRevision) => {
        const replacedAt = Date.parse(value.replacedAt);
        if (!Number.isFinite(replacedAt)) throw new Error('activity_revision_invalid');
        return { ...snapshot(value, value.version, pathId, activityId, profile.id), replacedAt };
      });
      const next = response.data?.meta.nextCursor ?? null;
      if (next && (next === cursor || !items.length)) throw new Error('activity_revision_cursor_invalid');
      return { items, next };
    },
    async remove(review) {
      const result = required(await client().deleteActivity(review.pathId, review.activityId, review.operationId));
      if (![result.sessionCount, result.unreadNotificationCount].every(value => Number.isSafeInteger(value) && value >= 0) || !Number.isSafeInteger(result.accumulatedSeconds) || result.accumulatedSeconds < 0 || !Array.isArray(result.removedFeedEventIds)) throw new Error('activity_deletion_invalid');
      const progress = result.intervalProgress;
      const period = progress?.startedAt && progress.endedAt ? { savedSeconds: progress.accumulatedSeconds, targetSeconds: progress.targetSeconds, startsAt: Date.parse(progress.startedAt), endsAt: Date.parse(progress.endedAt) } : null;
      if (period && (!Object.values(period).every(Number.isFinite) || period.endsAt <= period.startsAt || period.savedSeconds < 0 || period.targetSeconds <= 0)) throw new Error('activity_deletion_invalid');
      return { accumulatedSeconds: result.accumulatedSeconds, sessionCount: result.sessionCount, unreadNotificationCount: result.unreadNotificationCount, removedFeedEventIds: result.removedFeedEventIds, period };
    },
  };
}
