import type { createSessionApiClient } from '@hourpaths/api-client';
import { TrackingReplaySuspended, type RetainedActivity } from '../offline-tracking';

type SessionAPI = ReturnType<typeof createSessionApiClient>;

/** Own-account history DTOs terminate here. A caller commits only the complete
 * result, so a failed page never replaces the last durable snapshot. */
export function apiTrackingHistory(clientForOwner: (owner: string) => SessionAPI | null, now: () => number) {
  return async (owner: string): Promise<RetainedActivity[]> => {
    const client = () => {
      const value = clientForOwner(owner);
      if (!value) throw new TrackingReplaySuspended();
      return value;
    };
    const accept = <T>(result: { response: Response; data?: { data: T } }): T => {
      client();
      if (result.response.status === 401) throw new TrackingReplaySuspended();
      if (!result.response.ok || !result.data) throw new Error('tracking_history_unavailable');
      return result.data.data;
    };
    const paths = new Map<string, string>();
    for (const archived of [false, true]) {
      let cursor: string | undefined;
      const seen = new Set<string>();
      do {
        const result = await (archived ? client().archivedPaths(cursor) : client().paths(cursor));
        for (const path of accept(result)) paths.set(path.id, path.name);
        cursor = result.data?.meta.nextCursor || undefined;
        if (cursor && seen.has(cursor)) throw new Error('tracking_history_cursor_repeated');
        if (cursor) seen.add(cursor);
      } while (cursor);
    }
    const cutoff = now() - 90 * 24 * 60 * 60 * 1000;
    const entries: RetainedActivity[] = [];
    for (const [pathId, pathName] of paths) {
      let cursor: string | undefined;
      const seen = new Set<string>();
      do {
        const result = await client().activities(pathId, cursor, owner);
        for (const { activity, version } of accept(result)) {
          if (activity.pathId !== pathId || activity.participantId !== owner
            || !Number.isFinite(Date.parse(activity.startedAt)) || !Number.isFinite(Date.parse(activity.endedAt))
            || Date.parse(activity.endedAt) <= Date.parse(activity.startedAt)) throw new Error('tracking_history_invalid');
          new Intl.DateTimeFormat('en', { timeZone: activity.occurrenceTimeZone });
          if (Date.parse(activity.endedAt) < cutoff) continue;
          entries.push({ id: activity.id, owner, pathId, pathName, startedAt: activity.startedAt,
            endedAt: activity.endedAt, timeZone: activity.occurrenceTimeZone,
            version, note: activity.note, createdAt: activity.createdAt, updatedAt: activity.updatedAt });
        }
        cursor = result.data?.meta.nextCursor || undefined;
        if (cursor && seen.has(cursor)) throw new Error('tracking_history_cursor_repeated');
        if (cursor) seen.add(cursor);
      } while (cursor);
    }
    client();
    return entries;
  };
}
