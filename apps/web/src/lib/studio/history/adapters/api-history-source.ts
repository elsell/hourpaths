import { createSessionApiClient } from '@hourpaths/api-client';
import type { HistorySource } from '../ports/history-source';
import type { PathRepository } from '../../paths/ports/path-repository';
export class HistoryRequestError extends Error {
  constructor(readonly status: number) { super("history_unavailable"); }
}
function required<T>(response: { data?: { data: T }; response: Response }): T {
  if (!response.response.ok || !response.data) throw new HistoryRequestError(response.response.status);
  return response.data.data;
}
export function apiHistorySource(apiURL: string, token: () => string | null, paths: PathRepository, rejected: (token: string | null) => void): HistorySource {
  const accepted = <T>(response: { data?: { data: T }; response: Response }): T => {
    return required(response);
  };
  return {
    async initial(signal) {
      const profile = accepted(await createSessionApiClient(apiURL, token, signal, rejected).profile());
      const all = [...await paths.list(false, signal), ...await paths.list(true, signal)];
      const unique = [...new Map(all.map(path => [path.id, path])).values()];
      return { participantId: profile.id, streams: unique.map(path => ({ pathId: path.id, pathName: path.name, remaining: [], cursor: null, loaded: false })) };
    },
    async read(pathId, pathName, participantId, cursor, signal) {
      const response = await createSessionApiClient(apiURL, token, signal, rejected).activities(pathId, cursor ?? undefined, participantId);
      const items = accepted(response).map(({ activity }) => {
        const startedAt = Date.parse(activity.startedAt);
        const endedAt = Date.parse(activity.endedAt);
        if (activity.pathId !== pathId || activity.participantId !== participantId || !Number.isFinite(startedAt) || !Number.isFinite(endedAt) || endedAt <= startedAt || !Number.isFinite(activity.durationSeconds) || activity.durationSeconds < 0) throw new Error('history_invalid');
        // Validate the authoritative occurrence zone before presentation formats it.
        new Intl.DateTimeFormat('en', { timeZone: activity.occurrenceTimeZone }).format(startedAt);
        return { id: activity.id, pathId, pathName, startedAt, endedAt, seconds: activity.durationSeconds, timeZone: activity.occurrenceTimeZone };
      });
      return { items, next: response.data?.meta.nextCursor ?? null };
    },
  };
}
