import { createSessionApiClient, type ActivityDetail as APIDetail, type ActivityRevision as APIRevision } from '@hourpaths/api-client';
import type { ActivitySnapshot } from '../domain/detail';
import type { ActivityRepository } from '../ports/activity-repository';
function required<T>(result: { data?: { data: T }; response: Response }): T {
  if (!result.response.ok || !result.data) throw new Error('activity_unavailable');
  return result.data.data;
}
function snapshot(value: APIDetail['activity'], version: number, pathId: string, activityId: string, owner: string): ActivitySnapshot {
  const startedAt = Date.parse(value.startedAt), endedAt = Date.parse(value.endedAt);
  if (value.id !== activityId || value.pathId !== pathId || !value.participantId || !Number.isFinite(startedAt) || !Number.isFinite(endedAt) || !Number.isSafeInteger(value.durationSeconds) || value.durationSeconds <= 0 || (endedAt - startedAt) / 1000 !== value.durationSeconds || !Number.isSafeInteger(version) || version < 1) throw new Error('activity_invalid');
  new Intl.DateTimeFormat('en', { timeZone: value.occurrenceTimeZone }).format(startedAt);
  return { id: activityId, pathId, participantId: value.participantId, startedAt, endedAt, seconds: value.durationSeconds, timeZone: value.occurrenceTimeZone, version, note: value.participantId === owner ? value.note ?? null : null };
}
export function apiActivityRepository(baseURL: string, token: () => string | null, rejected: (token: string | null) => void): ActivityRepository {
  const client = (signal?: AbortSignal) => createSessionApiClient(baseURL, token, signal, rejected);
  return {
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
      if (!Number.isSafeInteger(result.accumulatedSeconds) || result.accumulatedSeconds < 0 || !Array.isArray(result.removedFeedEventIds)) throw new Error('activity_deletion_invalid');
    },
  };
}
