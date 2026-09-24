import type { CapabilityPath } from './path-capabilities';

export type PathArchivePath = CapabilityPath & {
  [key: string]: unknown;
};

export type PathArchiveReview = Readonly<{
  pathId: string;
  expectedArchived: boolean;
  archived: boolean;
}>;

export type PathArchiveRequestBody = Readonly<{
  confirmed: true;
  expectedArchived: boolean;
  archived: boolean;
}>;

export type PathArchiveMutationResult<P extends PathArchivePath> =
  | { kind: 'applied'; path: P }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' }
  | { kind: 'cancelled' };

type PathTimerState = {
  running: boolean;
  timer?: unknown;
  [key: string]: unknown;
};

type Retry = {
  signature: string;
  idempotencyKey: string;
  body: PathArchiveRequestBody;
};

function archived(path: PathArchivePath): boolean {
  return path.archivedAt !== undefined && path.archivedAt !== null;
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

export function pathArchiveLists<P extends PathArchivePath>(
  paths: readonly P[],
): { activePaths: P[]; archivedPaths: P[] } {
  return {
    activePaths: paths.filter((path) => !archived(path)),
    archivedPaths: paths.filter(archived),
  };
}

export function reviewPathArchiveChange(path: PathArchivePath): PathArchiveReview {
  if (!path.id || path.id.trim() !== path.id) throw new Error('invalid Path archive review');
  const expectedArchived = archived(path);
  return Object.freeze({
    pathId: path.id,
    expectedArchived,
    archived: !expectedArchived,
  });
}

function replaceOrAppend<P extends PathArchivePath>(paths: readonly P[], path: P): P[] {
  const existing = paths.findIndex((candidate) => candidate.id === path.id);
  if (existing < 0) return [...paths, path];
  return paths.map((candidate, index) => index === existing ? path : candidate);
}

export function applyPathArchiveResult<
  P extends PathArchivePath,
  T extends PathTimerState,
  S extends {
    activePaths: P[];
    archivedPaths: P[];
    selectedPath?: P | null;
    timerStates: Record<string, T>;
  },
>(state: S, path: NoInfer<P>): S {
  const isArchived = archived(path);
  const activePaths = isArchived
    ? state.activePaths.filter((candidate) => candidate.id !== path.id)
    : replaceOrAppend(
      state.activePaths.filter((candidate) => candidate.id !== path.id),
      path,
    );
  const archivedPaths = isArchived
    ? replaceOrAppend(
      state.archivedPaths.filter((candidate) => candidate.id !== path.id),
      path,
    )
    : state.archivedPaths.filter((candidate) => candidate.id !== path.id);
  const timer = state.timerStates[path.id];
  const timerStates = isArchived && timer
    ? {
        ...state.timerStates,
        [path.id]: { ...timer, running: false, timer: undefined },
      }
    : state.timerStates;

  return {
    ...state,
    activePaths,
    archivedPaths,
    selectedPath: state.selectedPath?.id === path.id ? path : state.selectedPath,
    timerStates,
  };
}

export function createPathArchiveOperationOwner(keyFactory: () => string) {
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
    async submit<P extends PathArchivePath>(
      review: PathArchiveReview,
      confirmed: boolean,
      request: (
        pathId: string,
        body: PathArchiveRequestBody,
        idempotencyKey: string,
      ) => Promise<P>,
    ): Promise<PathArchiveMutationResult<P>> {
      if (!confirmed) {
        cancel(review.pathId);
        return { kind: 'cancelled' };
      }
      const signature = `${review.expectedArchived ? 'archived' : 'active'}:${review.archived ? 'archived' : 'active'}`;
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
              expectedArchived: review.expectedArchived,
              archived: review.archived,
            }),
          };
      if (!validIdempotencyKey(retry.idempotencyKey)) {
        throw new Error('invalid Path archive idempotency key');
      }
      retries[review.pathId] = retry;

      try {
        const path = await request(review.pathId, retry.body, retry.idempotencyKey);
        if (epochs[review.pathId] !== epoch) return { kind: 'superseded' };
        if (path.id !== review.pathId || archived(path) !== review.archived) {
          throw new Error('invalid Path archive result');
        }
        delete retries[review.pathId];
        return { kind: 'applied', path };
      } catch (cause) {
        return epochs[review.pathId] === epoch
          ? { kind: 'failed', cause }
          : { kind: 'superseded' };
      }
    },
    cancel,
  };
}
