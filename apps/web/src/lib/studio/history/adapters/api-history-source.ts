import { createSessionApiClient } from '@hourpaths/api-client';
import type { HistorySource, PathHistorySource } from '../ports/history-source';
export class HistoryRequestError extends Error {
  constructor(readonly status: number) { super("history_unavailable"); }
}
function required<T>(response: { data?: { data: T }; response: Response }): T {
  if (!response.response.ok || !response.data) throw new HistoryRequestError(response.response.status);
  return response.data.data;
}
export function apiHistorySource(apiURL: string, token: () => string | null, rejected: (token: string | null) => void): HistorySource {
  const accepted = <T>(response: { data?: { data: T }; response: Response }): T => {
    return required(response);
  };
  return {
    async initial(signal) {
      const profile = accepted(await createSessionApiClient(apiURL, token, signal, rejected, { retryRateLimitedReads: true }).profile());
      return profile.id;
    },
    async read(participantId, cursor, window, signal) {
      const response = await createSessionApiClient(apiURL, token, signal, rejected, { retryRateLimitedReads: true }).ownActivityHistory(window, cursor ?? undefined);
      const items = accepted(response).map(record => mapHistoryRecord(record, record.activity.pathId, record.pathName, participantId));
      return { items, next: response.data?.meta.nextCursor ?? null };
    },
  };
}

type ActivityRecord = NonNullable<Awaited<ReturnType<ReturnType<typeof createSessionApiClient>['activities']>>['data']>['data'][number];
function mapHistoryRecord({ activity, version }: ActivityRecord, pathId: string, pathName: string, participantId: string) {
        const startedAt = Date.parse(activity.startedAt);
        const endedAt = Date.parse(activity.endedAt);
        if (activity.pathId !== pathId || !pathName.trim() || activity.participantId !== participantId || !Number.isFinite(startedAt) || !Number.isFinite(endedAt) || endedAt <= startedAt || !Number.isFinite(activity.durationSeconds) || activity.durationSeconds < 0) throw new Error('history_invalid');
        // Validate the authoritative occurrence zone before presentation formats it.
        new Intl.DateTimeFormat('en', { timeZone: activity.occurrenceTimeZone }).format(startedAt);
        return { editStamp: activity.editOrder ? { authoredAt: activity.editOrder.authoredAt, counter: activity.editOrder.counter } : undefined, originalStartedAt: activity.startedAt, originalEndedAt: activity.endedAt, note: activity.note ?? '', version, createdAt: activity.createdAt, updatedAt: activity.updatedAt, id: activity.id, pathId, pathName, startedAt, endedAt, seconds: activity.durationSeconds, timeZone: activity.occurrenceTimeZone };

}
export function apiPathHistorySource(apiURL: string, token: () => string | null, rejected: (token: string | null) => void): PathHistorySource {
  return { async read(pathId, pathName, participantId, cursor, signal) {
    const response = await createSessionApiClient(apiURL, token, signal, rejected, { retryRateLimitedReads: true }).activities(pathId, cursor ?? undefined, participantId);
    const items = required(response).map(record => mapHistoryRecord(record, pathId, pathName, participantId));
    return { items, next: response.data?.meta.nextCursor ?? null };
  } };
}
