import type { components } from './schema';

export type TimerState = components['schemas']['TimerState'];
export type TimerStopResult = components['schemas']['StopResult'];
export type TimerMutationResult<T extends TimerState = TimerState> =
  | { kind: 'applied'; state: T }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' };

type Retry = { signature: string; idempotencyKey: string };

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

export function elapsedTimerSeconds(state: TimerState, now: number): number {
  const accumulated = Math.max(0, Math.floor(state.accumulatedSeconds));
  if (!state.running || !state.timer) return accumulated;
  const startedAt = Date.parse(state.timer.startedAt);
  if (!Number.isFinite(startedAt) || !Number.isFinite(now)) return accumulated;
  return accumulated + Math.max(0, Math.floor((now - startedAt) / 1000));
}

export function formatTimerDuration(value: number): string {
  const seconds = Math.max(0, Math.floor(Number.isFinite(value) ? value : 0));
  const hours = Math.floor(seconds / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const remainder = seconds % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(remainder).padStart(2, '0')}`
    : `${minutes}:${String(remainder).padStart(2, '0')}`;
}

export function timerMutationPresentation<T extends TimerState>(state: T): {
  state: T;
  notice: 'subsecond' | null;
  controlMessage: 'timer.start' | 'timer.stop';
} {
  const outcome = state as T & Partial<Pick<TimerStopResult, 'saved' | 'subsecond'>>;
  return {
    state,
    notice: outcome.saved === false && outcome.subsecond === true ? 'subsecond' : null,
    controlMessage: state.running && state.timer ? 'timer.stop' : 'timer.start',
  };
}

export function createTimerOperationOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, Retry | undefined> = {};

  async function mutate<T extends TimerState>(
    pathID: string,
    signature: string,
    request: (idempotencyKey: string) => Promise<T>,
  ): Promise<TimerMutationResult<T>> {
    const epoch = (epochs[pathID] ?? 0) + 1;
    epochs[pathID] = epoch;
    const retry = retries[pathID];
    const idempotencyKey = retry?.signature === signature ? retry.idempotencyKey : keyFactory();
    if (!validIdempotencyKey(idempotencyKey)) throw new Error('invalid timer idempotency key');
    retries[pathID] = { signature, idempotencyKey };
    try {
      const state = await request(idempotencyKey);
      if (epochs[pathID] !== epoch) return { kind: 'superseded' };
      delete retries[pathID];
      return { kind: 'applied', state };
    } catch (cause) {
      return epochs[pathID] === epoch ? { kind: 'failed', cause } : { kind: 'superseded' };
    }
  }

  return {
    start(pathID: string, request: (idempotencyKey: string) => Promise<TimerState>) {
      return mutate(pathID, 'start', request);
    },
    stop(pathID: string, timerID: string, request: (idempotencyKey: string) => Promise<TimerStopResult>) {
      return mutate(pathID, `stop:${timerID}`, request);
    },
    cancel(pathID?: string) {
      if (pathID) {
        epochs[pathID] = (epochs[pathID] ?? 0) + 1;
        delete retries[pathID];
        return;
      }
      for (const key of Object.keys(epochs)) epochs[key] = (epochs[key] ?? 0) + 1;
      for (const key of Object.keys(retries)) delete retries[key];
    },
  };
}
