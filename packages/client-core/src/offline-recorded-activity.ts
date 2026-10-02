import type { RetainedActivity, TrackingSnapshot } from './offline-tracking';

function clone<T>(value: T): T { return JSON.parse(JSON.stringify(value)) as T; }

export interface ActivityEditStamp { authoredAt: string; counter: number }
export interface RecordedActivityInput { startedAt: string; durationSeconds: number; note: string }
export interface RecordedActivityOperation {
  kind: 'create' | 'edit'; operationId: string; activity: RetainedActivity;
  previous?: RetainedActivity; stamp: ActivityEditStamp;
}

function occurrence(input: RecordedActivityInput, now: number): { startedAt: string; endedAt: string; note: string } {
  const start = Date.parse(input.startedAt);
  const duration = input.durationSeconds;
  const note = input.note.normalize('NFC');
  if (!/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?Z$/.test(input.startedAt) || !Number.isFinite(start) || !Number.isSafeInteger(duration) || duration < 1
    || new Date(start).toISOString().slice(0, 19) !== input.startedAt.slice(0, 19)
    || !Number.isFinite(start + duration * 1000) || start + duration * 1000 > now
    || [...note].length > 2000 || /[\p{Cc}\p{Cs}]/u.test(note.replace(/[\r\n]/g, ''))) throw new Error('activity_invalid');
  // Whole-second additions preserve any finer precision of an adopted server entry.
  const fraction = /\.(\d+)Z$/.exec(input.startedAt)?.[1];
  const endedAt = new Date(start + duration * 1000).toISOString().replace(/\.\d{3}Z$/, fraction ? `.${fraction}Z` : 'Z');
  return { startedAt: input.startedAt, endedAt, note: note.trim() ? note : '' };
}
function writable(state: TrackingSnapshot, pathId: string) {
  const path = state.paths.find(value => value.id === pathId);
  if (!path || state.unavailablePaths?.includes(pathId)) throw new Error('tracking_path_unavailable');
  return path;
}
export function createRecordedActivity(state: TrackingSnapshot, input: RecordedActivityInput & { pathId: string }, now: number, id: string, operationId: string): RetainedActivity {
  const path = writable(state, input.pathId);
  const instant = new Date(now).toISOString();
  const stamp = { authoredAt: instant, counter: 0 };
  const activity = { id, owner: state.owner, pathId: path.id, pathName: path.name, timeZone: path.timeZone,
    ...occurrence(input, now), createdAt: instant, updatedAt: instant, version: 1, editStamp: stamp };
  (state.activityOperations ??= []).push({ kind: 'create', operationId, activity, stamp });
  return clone(activity);
}
export function editRecordedActivity(state: TrackingSnapshot, activityId: string, input: RecordedActivityInput, now: number, operationId: string): RetainedActivity {
  const queued = state.activityOperations?.filter(value => value.activity.id === activityId).at(-1)?.activity;
  const previous = queued ?? state.history.find(value => value.id === activityId);
  if (!previous || previous.owner !== state.owner) throw new Error('activity_unavailable');
  writable(state, previous.pathId);
  const observed = previous.editStamp ?? { authoredAt: previous.updatedAt ?? previous.createdAt ?? new Date(now).toISOString(), counter: 0 };
  const authored = Math.max(now, Date.parse(observed.authoredAt));
  const stamp = { authoredAt: new Date(authored).toISOString(), counter: authored === Date.parse(observed.authoredAt) ? observed.counter + 1 : 0 };
  const activity = { ...previous, ...occurrence(input, now), updatedAt: new Date(Math.max(now, Date.parse(previous.createdAt ?? new Date(now).toISOString()))).toISOString(),
    version: (previous.version ?? 1) + 1, editStamp: stamp };
  (state.activityOperations ??= []).push({ kind: 'edit', operationId, activity, previous: clone(previous), stamp });
  return clone(activity);
}
export function recordedActivityChanges(state: TrackingSnapshot) {
  const entries = new Map<string, { previous?: RetainedActivity; activity: RetainedActivity }>();
  for (const operation of state.activityOperations ?? []) {
    const prior = entries.get(operation.activity.id);
    entries.set(operation.activity.id, { previous: prior ? prior.previous : operation.previous, activity: operation.activity });
  }
  return [...entries.values()];
}
export function recordedActivityDelta(state: TrackingSnapshot, pathId: string, period?: { startsAt: number; endsAt: number } | null): number {
  const duration = (entry?: RetainedActivity) => entry ? Math.max(0, Math.floor((Math.min(Date.parse(entry.endedAt), period?.endsAt ?? Infinity)
    - Math.max(Date.parse(entry.startedAt), period?.startsAt ?? -Infinity)) / 1000)) : 0;
  return recordedActivityChanges(state).filter(value => value.activity.pathId === pathId)
    .reduce((total, value) => total + duration(value.activity) - duration(value.previous), 0);
}
