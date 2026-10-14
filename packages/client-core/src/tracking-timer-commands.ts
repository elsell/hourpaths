import type { LocalTimer, TrackingSnapshot } from './offline-tracking';

/** Shared synchronous mutations for in-app and native background transactions.
 * Callers own the account fence and atomic revision comparison/commit. */
export function startTrackingTimer(state: TrackingSnapshot, pathId: string, id: string, operationId: string, startedAt: string): LocalTimer {
  const path = state.paths.find(candidate => candidate.id === pathId);
  if (!path || state.unavailablePaths?.includes(pathId)) throw new Error('tracking_path_unavailable');
  if (state.timers.some(timer => timer.pathId === pathId)) throw new Error('tracking_timer_already_running');
  const timer = { id, pathId, startedAt, timeZone: path.timeZone };
  state.timers.push(timer);
  state.operations.push({ ...timer, timerId: id, operationId, kind: 'start' });
  return { ...timer };
}

export function stopTrackingTimer(state: TrackingSnapshot, timerId: string, operationId: string, endedAt: string): void {
  const timer = state.timers.find(candidate => candidate.id === timerId);
  if (!timer) throw new Error('tracking_timer_unavailable');
  state.timers = state.timers.filter(candidate => candidate.id !== timerId);
  if (Date.parse(endedAt) <= Date.parse(timer.startedAt)) {
    state.corrections.push({ timer, endedAt });
  } else {
    state.operations.push({ ...timer, timerId, operationId, kind: 'stop', endedAt });
    if (Date.parse(endedAt) - Date.parse(timer.startedAt) < 1000) state.notices.push({ id: operationId, reason: 'subsecond', pathId: timer.pathId });
  }
}
