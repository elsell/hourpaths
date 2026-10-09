import { projectTrackingHistory, type TrackingSnapshot, type RetainedActivity } from '@hourpaths/client-core';
import { newestActivitiesFirst, type ActivityDetail } from '../activity-history';

/** Retained rows are summaries. Unknown server metadata stays absent and these
 * rows cannot navigate to details or revision/edit controls while offline. */
export function retainedHistoryDetails(items: (RetainedActivity & { pending: boolean })[]): ActivityDetail[] {
  return items.map(entry => ({ retained: true, pending: entry.pending, version: entry.version ?? 1, activity: {
    id: entry.id, pathId: entry.pathId, participantId: entry.owner, startedAt: entry.startedAt, endedAt: entry.endedAt,
    occurrenceTimeZone: entry.timeZone, durationSeconds: Math.floor((Date.parse(entry.endedAt) - Date.parse(entry.startedAt)) / 1000),
    createdAt: entry.createdAt ?? '', updatedAt: entry.updatedAt ?? '', note: entry.note,
  } }));
}

/** Use published durable state rather than the rows captured when a route opened. */
export function retainedHistoryFromSnapshot(snapshot: TrackingSnapshot, ownerID: string, pathID: string): ActivityDetail[] {
  if (snapshot.owner !== ownerID || snapshot.unavailablePaths?.includes(pathID)) return [];
  return retainedHistoryDetails(projectTrackingHistory(snapshot, ownerID, pathID).items);
}

/** Only an account-scoped tombstone proves deletion; cache absence does not. */
export function activityDeletionFromSnapshot(snapshot: TrackingSnapshot | null, ownerID: string, activityID: string): boolean {
  if (snapshot?.owner !== ownerID) return false;
  const canonicalID = snapshot.activityAliases?.[activityID] ?? activityID;
  return snapshot.deletedActivityIds?.includes(canonicalID) === true;
}


/** Overlay this account's durable changes on the loaded online window. Other
 * participants and older loaded pages remain server-owned. */
export function onlineHistoryFromSnapshot(rows: readonly ActivityDetail[], snapshot: TrackingSnapshot, ownerID: string, pathID: string): ActivityDetail[] {
  if (snapshot.owner !== ownerID || snapshot.unavailablePaths?.includes(pathID)) return [];
  const deleted = new Set(snapshot.deletedActivityIds ?? []);
  const loaded = rows.filter(row => row.activity.pathId === pathID);
  const floor = loaded.length ? Math.min(...loaded.map(row => Date.parse(row.activity.startedAt))) : -Infinity;
  const visible = new Map(loaded.filter(row => !deleted.has(row.activity.id)).map(row => [row.activity.id, row]));
  for (const row of retainedHistoryFromSnapshot(snapshot, ownerID, pathID)) {
    if (deleted.has(row.activity.id)) continue;
    const existing = visible.get(row.activity.id);
    if (existing && (existing.activity.participantId !== ownerID || (!row.pending && existing.version >= row.version))) continue;
    if (existing || row.pending || Date.parse(row.activity.startedAt) >= floor) {
      visible.set(row.activity.id, { ...row, retained: row.pending === true });
    }
  }
  return newestActivitiesFirst([...visible.values()]);
}
