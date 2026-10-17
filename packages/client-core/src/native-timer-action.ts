import { nativeGoalPeriodSchedule } from './native-goal-period';
import type { TrackingSnapshot } from './offline-tracking';
import { startTrackingTimer, stopTrackingTimer } from './tracking-timer-commands';

export type NativeTimerAction = Readonly<{
  owner: string;
  revision: number;
  pathId: string;
  kind: 'start' | 'stop';
  timerId: string;
  operationId: string;
}>;

/** Executed inside the native adapter's SQLite transaction. No framework or
 * asynchronous runtime dependency: the same mutation rules run in JavaScriptCore.
 * The adapter must separately enforce the current account lease/deletion fence. */
export function applyNativeTimerAction(snapshot: TrackingSnapshot, action: NativeTimerAction, now: number): TrackingSnapshot {
  if (!action.owner || snapshot.owner !== action.owner) throw new Error('tracking_owner_mismatch');
  // Stop names an exact existing timer. Unrelated ledger writes must not disable
  // that control; Start still requires the displayed state to be current.
  if (!Number.isSafeInteger(action.revision) || action.revision < 0 || action.revision > snapshot.revision
    || action.kind !== 'stop' && action.revision !== snapshot.revision) throw new Error('tracking_native_action_stale');
  if (!Number.isFinite(now) || !action.pathId || !action.timerId || !action.operationId
    || !['start', 'stop'].includes(action.kind)) throw new Error('tracking_native_action_invalid');
  if (!snapshot.paths.some(path => path.id === action.pathId) || snapshot.unavailablePaths?.includes(action.pathId)) {
    throw new Error('tracking_path_unavailable');
  }
  const state: TrackingSnapshot = JSON.parse(JSON.stringify(snapshot));
  const instant = new Date(now).toISOString();
  if (action.kind === 'start') {
    if (state.operations.some(operation => operation.operationId === action.operationId || operation.timerId === action.timerId)) {
      throw new Error('tracking_native_action_stale');
    }
    startTrackingTimer(state, action.pathId, action.timerId, action.operationId, instant);
  } else {
    if (!state.timers.some(timer => timer.id === action.timerId && timer.pathId === action.pathId)) throw new Error('tracking_timer_unavailable');
    stopTrackingTimer(state, action.timerId, action.operationId, instant);
  }
  delete state.historyRefreshRange;
  state.revision++;
  return state;
}

/** Rebind every visible control to the committed revision, including stopped
 * rows which offer Start. Localized copy and appearance are supplied by the UI. */
export function refreshNativeTimerPresentation(presentation: {
  accountLease: string; owner: string; startLabel: string; stopLabel: string;
  timers: Array<Record<string, unknown> & { pathId: string }>;
  [key: string]: unknown;
}, state: TrackingSnapshot, now: number, identifiers: string[]) {
  if (presentation.owner !== state.owner) throw new Error('tracking_owner_mismatch');
  const timers = presentation.timers.map((row, index) => {
    const active = state.timers.find(timer => timer.pathId === row.pathId);
    const wasStopped = typeof row.stoppedAt === 'number';
    const timerId = active?.id ?? identifiers[index * 2];
    const operationId = identifiers[index * 2 + 1];
    if (!timerId || !operationId) throw new Error('tracking_native_action_invalid');
    const previousGoal = row.goal as { lower?: number; upper?: number; progressUpper?: number } | undefined;
    const gap = active && wasStopped ? Date.parse(active.startedAt) - Number(row.stoppedAt) : 0;
    const resumedGoal = active && wasStopped && previousGoal?.lower !== undefined && previousGoal.upper !== undefined
      && now < previousGoal.upper && gap >= 0
      ? { ...previousGoal, lower: previousGoal.lower + gap,
        progressUpper: previousGoal.progressUpper === undefined ? undefined : previousGoal.progressUpper + gap }
      : undefined;
    const target = JSON.stringify({ lease: presentation.accountLease, action: {
      owner: state.owner, revision: state.revision, pathId: row.pathId,
      timerId, operationId, kind: active ? 'stop' : 'start',
    } });
    return { ...row, id: timerId,
      ...(typeof presentation.activityExpiresAt === 'number' ? {
        goalPeriods: nativeGoalPeriodSchedule(state, presentation.owner, row.pathId, now, presentation.activityExpiresAt),
      } : {}),
      ...(active ? { startedAt: Date.parse(active.startedAt), stoppedAt: undefined,
        ...(wasStopped ? { goal: typeof presentation.activityExpiresAt === 'number' ? previousGoal : resumedGoal } : {}) } : { stoppedAt: row.stoppedAt ?? now }),
      action: { target, label: active ? presentation.stopLabel : presentation.startLabel },
    };
  });
  const titles = presentation.titles as Record<string, string> | undefined;
  return { ...presentation, trackingRevision: state.revision, timers, title: titles?.[state.timers.length] ?? presentation.title };
}
