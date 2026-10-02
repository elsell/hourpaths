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
  const id = activityIdentity(state, activityId);
  const queued = state.activityOperations?.filter(value => activityIdentity(state, value.activity.id) === id).at(-1)?.activity;
  const stopped = state.operations.find(value => value.operationId === activityId && value.kind !== 'start' && value.endedAt);
  const pendingTimer: RetainedActivity | undefined = stopped?.endedAt && Date.parse(stopped.endedAt) - Date.parse(stopped.correctedStartedAt ?? stopped.startedAt) >= 1000 ? { id: activityId, owner: state.owner, pathId: stopped.pathId,
    startedAt: stopped.correctedStartedAt ?? stopped.startedAt, endedAt: stopped.endedAt, timeZone: stopped.timeZone } : undefined;
  const previous = queued ?? state.history.find(value => value.id === id) ?? pendingTimer;
  if (state.deletedActivityIds?.includes(id)) throw new Error('activity_unavailable');
  if (!previous || previous.owner !== state.owner) throw new Error('activity_unavailable');
  writable(state, previous.pathId);
  const observed = previous.editStamp ?? { authoredAt: previous.updatedAt ?? previous.createdAt ?? new Date(now).toISOString(), counter: 0 };
  if (!Number.isFinite(Date.parse(observed.authoredAt)) || !Number.isSafeInteger(observed.counter) || observed.counter < 0) throw new Error('activity_invalid');
  const authored = Math.max(now, Date.parse(observed.authoredAt));
  const stamp = { authoredAt: new Date(authored).toISOString(), counter: authored === Date.parse(observed.authoredAt) ? observed.counter + 1 : 0 };
  if (!Number.isSafeInteger(stamp.counter)) throw new Error('activity_invalid');
  const activity = { ...previous, ...occurrence(input, now), updatedAt: new Date(Math.max(now, Date.parse(previous.createdAt ?? new Date(now).toISOString()))).toISOString(),
    version: (previous.version ?? 1) + 1, editStamp: stamp };
  (state.activityOperations ??= []).push({ kind: 'edit', operationId, activity, previous: clone(previous), stamp });
  return clone(activity);
}
export function recordedActivityChanges(state: TrackingSnapshot) {
  const entries = new Map<string, { previous?: RetainedActivity; activity: RetainedActivity }>();
  for (const operation of state.activityOperations ?? []) {
    const id = activityIdentity(state, operation.activity.id);
    const prior = entries.get(id);
    entries.set(id, { previous: prior ? prior.previous : operation.previous, activity: { ...operation.activity, id } });
  }
  return [...entries.values()];
}
export function recordedActivityDelta(state: TrackingSnapshot, pathId: string, period?: { startsAt: number; endsAt: number } | null, includeAcknowledged = true): number {
  const duration = (entry?: RetainedActivity) => entry ? Math.max(0, Math.floor((Math.min(Date.parse(entry.endedAt), period?.endsAt ?? Infinity)
    - Math.max(Date.parse(entry.startedAt), period?.startsAt ?? -Infinity)) / 1000)) : 0;
  return [...(includeAcknowledged ? state.activityDeltas ?? [] : []), ...recordedActivityChanges(state)].filter(value => (value.activity?.pathId ?? value.previous?.pathId) === pathId)
    .reduce((total, value) => total + duration(value.activity) - duration(value.previous), 0);
}

export type RecordedActivityOutcome = { kind: 'accepted'; activity: RetainedActivity } | {
  kind: 'rejected'; reason: 'deleted' | 'membership' | 'archived' | 'validation'; disclosePath: boolean;
};
export interface RecordedActivityDelta { previous?: RetainedActivity; activity?: RetainedActivity }
export function activityIdentity(state: TrackingSnapshot, id: string): string { return state.activityAliases?.[id] ?? id; }

export function settleRecordedActivity(state: TrackingSnapshot, operation: RecordedActivityOperation, outcome: RecordedActivityOutcome): void {
  if (!state.activityOperations?.some(value => value.operationId === operation.operationId)) return;
  const id = activityIdentity(state, operation.activity.id), pathId = operation.activity.pathId;
  if (outcome.kind === 'accepted') {
    const previous = state.history.find(value => value.id === id);
    (state.activityDeltas ??= []).push({ previous: previous && clone(previous), activity: clone(outcome.activity) });
    state.history = state.history.filter(value => value.id !== id);
    state.history.push(clone(outcome.activity));
    state.activityOperations = state.activityOperations.filter(value => value.operationId !== operation.operationId);
    const next = state.activityOperations.find(value => activityIdentity(state, value.activity.id) === id);
    if (next) next.previous = clone(outcome.activity);
  } else {
    const entityRejected = outcome.reason === 'deleted' || operation.kind === 'create';
    if (outcome.reason === 'membership') {
      state.unavailablePaths = [...new Set([...(state.unavailablePaths ?? []), pathId])];
      state.activityOperations = state.activityOperations.filter(value => value.activity.pathId !== pathId);
      state.operations = state.operations.filter(value => value.pathId !== pathId);
      state.timers = state.timers.filter(value => value.pathId !== pathId);
      state.corrections = state.corrections.filter(value => value.timer.pathId !== pathId);
    } else {
      state.activityOperations = state.activityOperations.filter(value => entityRejected
        ? activityIdentity(state, value.activity.id) !== id : value.operationId !== operation.operationId);
    }
    if (outcome.reason === 'deleted') {
      const previous = state.history.find(value => value.id === id);
      if (previous) (state.activityDeltas ??= []).push({ previous: clone(previous) });
      state.deletedActivityIds = [...new Set([...(state.deletedActivityIds ?? []), id])];
      state.history = state.history.filter(value => value.id !== id);
    } else {
      const next = state.activityOperations.find(value => activityIdentity(state, value.activity.id) === id);
      if (next) next.previous = state.history.find(value => value.id === id);
    }
    state.notices.push({ id: operation.operationId, reason: outcome.reason, subject: 'activity',
      ...(outcome.disclosePath ? { pathId } : {}) });
  }
}
