import type { CapabilityPath } from './path-capabilities';

export type PathLeavePath = CapabilityPath & Readonly<{ name: string }>;

export type PathLeaveReview = Readonly<{
  pathId: string;
  pathName: string;
  retainActivity: boolean;
}>;

export type PathLeaveRequestBody = Readonly<{
  confirmed: true;
  retainActivity: boolean;
}>;

export type PathLeaveReceipt = Readonly<{
  pathId: string;
  left: true;
  activityRetained: boolean;
}>;

export type PathLeaveMutationResult =
  | { kind: 'applied'; receipt: PathLeaveReceipt }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' }
  | { kind: 'cancelled' };

type Retry = Readonly<{
  body: PathLeaveRequestBody;
  idempotencyKey: string;
}>;

function validText(value: string): boolean {
  return value.length > 0 && value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/u.test(value);
}

function receipt(value: unknown, pathId: string, retainActivity: boolean): PathLeaveReceipt {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid Path leave result');
  const record = value as Record<string, unknown>;
  if (Object.keys(record).length !== 3 || record.pathId !== pathId || record.left !== true || record.activityRetained !== retainActivity) {
    throw new Error('invalid Path leave result');
  }
  return Object.freeze({ pathId, left: true, activityRetained: retainActivity });
}

export function reviewPathLeave(path: PathLeavePath, retainActivity = true): PathLeaveReview {
  if (!validText(path.id) || !validText(path.name) || path.capabilities.leavePath !== true || (!retainActivity && path.capabilities.trackTime !== true)) {
    throw new Error('Path cannot be left');
  }
  return Object.freeze({ pathId: path.id, pathName: path.name, retainActivity });
}

export function applyPathLeaveResult<
  P extends PathLeavePath,
  T,
  S extends {
    activePaths: P[];
    archivedPaths: P[];
    selectedPath?: P | null;
    timerStates: Record<string, T>;
  },
>(state: S, result: PathLeaveMutationResult): S {
  if (result.kind !== 'applied') return state;
  const pathId = result.receipt.pathId;
  const { [pathId]: _departedTimer, ...retainedTimers } = state.timerStates;
  return {
    ...state,
    activePaths: state.activePaths.filter((path) => path.id !== pathId),
    archivedPaths: state.archivedPaths.filter((path) => path.id !== pathId),
    selectedPath: state.selectedPath?.id === pathId ? null : state.selectedPath,
    timerStates: retainedTimers as Record<string, T>,
  };
}

export function createPathLeaveOperationOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, Retry | undefined> = {};

  function cancel(pathId?: string): void {
    if (pathId) {
      epochs[pathId] = (epochs[pathId] ?? 0) + 1;
      delete retries[pathId];
      return;
    }
    for (const key of Object.keys(epochs)) epochs[key] = (epochs[key] ?? 0) + 1;
    for (const key of Object.keys(retries)) delete retries[key];
  }

  return {
    async submit(
      review: PathLeaveReview,
      confirmed: boolean,
      request: (pathId: string, body: PathLeaveRequestBody, idempotencyKey: string) => Promise<unknown>,
    ): Promise<PathLeaveMutationResult> {
      if (!confirmed) {
        cancel(review.pathId);
        return { kind: 'cancelled' };
      }
      const epoch = (epochs[review.pathId] ?? 0) + 1;
      epochs[review.pathId] = epoch;
      const retry = retries[review.pathId] ?? Object.freeze({
        body: Object.freeze({ confirmed: true, retainActivity: review.retainActivity }),
        idempotencyKey: keyFactory(),
      });
      if (retry.body.retainActivity !== review.retainActivity) throw new Error('Path leave choice changed during retry');
      if (!validIdempotencyKey(retry.idempotencyKey)) throw new Error('invalid Path leave idempotency key');
      retries[review.pathId] = retry;
      try {
        const acknowledged = receipt(await request(review.pathId, retry.body, retry.idempotencyKey), review.pathId, retry.body.retainActivity);
        if (epochs[review.pathId] !== epoch) return { kind: 'superseded' };
        delete retries[review.pathId];
        return { kind: 'applied', receipt: acknowledged };
      } catch (cause) {
        return epochs[review.pathId] === epoch ? { kind: 'failed', cause } : { kind: 'superseded' };
      }
    },
    cancel,
  };
}
