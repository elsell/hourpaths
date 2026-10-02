import type { RetainedActivity } from '@hourpaths/client-core';
import type { ActivityDetail } from '../activity-history';

/** Retained rows are summaries. Unknown server metadata stays absent and these
 * rows cannot navigate to details or revision/edit controls while offline. */
export function retainedHistoryDetails(items: (RetainedActivity & { pending: boolean })[]): ActivityDetail[] {
  return items.map(entry => ({ retained: true, pending: entry.pending, version: entry.version ?? 1, activity: {
    id: entry.id, pathId: entry.pathId, participantId: entry.owner, startedAt: entry.startedAt, endedAt: entry.endedAt,
    occurrenceTimeZone: entry.timeZone, durationSeconds: Math.floor((Date.parse(entry.endedAt) - Date.parse(entry.startedAt)) / 1000),
    createdAt: entry.createdAt ?? '', updatedAt: entry.updatedAt ?? '', note: entry.note,
  } }));
}
