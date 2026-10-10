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
    const cutoff = now() - 90 * 24 * 60 * 60 * 1000;
    const entries: RetainedActivity[] = [];
    {
      let cursor: string | undefined;
      const seen = new Set<string>();
      do {
        const result = await client().ownActivityHistory('retained', cursor);
        for (const { activity, version, pathName } of accept(result)) {
          const pathId = activity.pathId;
          if (!pathId || !pathName.trim() || activity.participantId !== owner
            || !Number.isFinite(Date.parse(activity.startedAt)) || !Number.isFinite(Date.parse(activity.endedAt))
            || Date.parse(activity.endedAt) <= Date.parse(activity.startedAt)) throw new Error('tracking_history_invalid');
          new Intl.DateTimeFormat('en', { timeZone: activity.occurrenceTimeZone });
          if (Date.parse(activity.endedAt) < cutoff) continue;
          entries.push({ id: activity.id, owner, pathId, pathName, startedAt: activity.startedAt,
            endedAt: activity.endedAt, timeZone: activity.occurrenceTimeZone,
            version, note: activity.note ?? '', editStamp: activity.editOrder ? { authoredAt: activity.editOrder.authoredAt, counter: activity.editOrder.counter } : undefined, createdAt: activity.createdAt, updatedAt: activity.updatedAt });
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
