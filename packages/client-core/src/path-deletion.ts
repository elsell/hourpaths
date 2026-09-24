import type { CapabilityPath } from './path-capabilities';

export type PathDeletionPath = CapabilityPath & {
  name: string;
  [key: string]: unknown;
};

export type PathDeletionReview = Readonly<{
  pathId: string;
  expectedName: string;
}>;

export type PathDeletionRequestBody = Readonly<{
  confirmed: true;
  expectedName: string;
}>;

export type PathDeletionReceipt = Readonly<{
  pathId: string;
  deleted: true;
}>;

export type PathDeletionMutationResult =
  | { kind: 'applied'; deletion: PathDeletionReceipt }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' }
  | { kind: 'cancelled' };

type Retry = {
  signature: string;
  idempotencyKey: string;
  body: PathDeletionRequestBody;
};

function validText(value: string): boolean {
  return value.length > 0 && value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

function deletionReceipt(value: unknown, pathId: string): PathDeletionReceipt {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid Path deletion result');
  }
  const record = value as Record<string, unknown>;
  if (
    Object.keys(record).length !== 2 ||
    record.pathId !== pathId ||
    record.deleted !== true
  ) {
    throw new Error('invalid Path deletion result');
  }
  return Object.freeze({ pathId, deleted: true });
}

export function reviewPathDeletion(path: PathDeletionPath): PathDeletionReview {
  if (!validText(path.id) || !validText(path.name)) {
    throw new Error('invalid Path deletion review');
  }
  return Object.freeze({ pathId: path.id, expectedName: path.name });
}

export function applyPathDeletionResult<
  P extends PathDeletionPath,
  T,
  S extends {
    activePaths: P[];
    archivedPaths: P[];
    selectedPath?: P | null;
    timerStates: Record<string, T>;
  },
>(state: S, result: PathDeletionMutationResult): S {
  if (result.kind !== 'applied') return state;

  const pathId = result.deletion.pathId;
  let timerStates = state.timerStates;
  if (Object.prototype.hasOwnProperty.call(timerStates, pathId)) {
    const { [pathId]: _deletedTimer, ...retainedTimers } = timerStates;
    timerStates = retainedTimers as Record<string, T>;
  }
  return {
    ...state,
    activePaths: state.activePaths.filter((path) => path.id !== pathId),
    archivedPaths: state.archivedPaths.filter((path) => path.id !== pathId),
    selectedPath: state.selectedPath?.id === pathId ? null : state.selectedPath,
    timerStates,
  };
}

export function createPathDeletionOperationOwner(keyFactory: () => string) {
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
      review: PathDeletionReview,
      confirmed: boolean,
      request: (
        pathId: string,
        body: PathDeletionRequestBody,
        idempotencyKey: string,
      ) => Promise<unknown>,
    ): Promise<PathDeletionMutationResult> {
      if (!confirmed) {
        cancel(review.pathId);
        return { kind: 'cancelled' };
      }

      const signature = review.expectedName;
      const epoch = (epochs[review.pathId] ?? 0) + 1;
      epochs[review.pathId] = epoch;
      const previous = retries[review.pathId];
      const retry = previous?.signature === signature
        ? previous
        : {
            signature,
            idempotencyKey: keyFactory(),
            body: Object.freeze({
              confirmed: true,
              expectedName: review.expectedName,
            }),
          };
      if (!validIdempotencyKey(retry.idempotencyKey)) {
        throw new Error('invalid Path deletion idempotency key');
      }
      retries[review.pathId] = retry;

      try {
        const value = await request(review.pathId, retry.body, retry.idempotencyKey);
        if (epochs[review.pathId] !== epoch) return { kind: 'superseded' };
        const deletion = deletionReceipt(value, review.pathId);
        delete retries[review.pathId];
        return { kind: 'applied', deletion };
      } catch (cause) {
        return epochs[review.pathId] === epoch
          ? { kind: 'failed', cause }
          : { kind: 'superseded' };
      }
    },
    cancel,
  };
}
