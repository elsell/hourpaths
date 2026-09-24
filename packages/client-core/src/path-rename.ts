import type { CapabilityPath } from './path-capabilities';

export type PathRenamePath = CapabilityPath & {
  name: string;
  [key: string]: unknown;
};

export type PathRenameReview = Readonly<{
  pathId: string;
  expectedName: string;
  name: string;
  changed: boolean;
}>;

export type PathRenameRequestBody = Readonly<{
  expectedName: string;
  name: string;
}>;

export type PathRenameMutationResult<P extends PathRenamePath> =
  | { kind: 'applied'; path: P }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' }
  | { kind: 'cancelled' };

type Retry = {
  signature: string;
  idempotencyKey: string;
  body: PathRenameRequestBody;
};

function validPathName(value: string): boolean {
  const characters = Array.from(value);
  return characters.length >= 1
    && characters.length <= 100
    && !/[\p{C}\p{Zl}\p{Zp}]/u.test(value);
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

export function reviewPathRename(path: PathRenamePath, proposedName: string): PathRenameReview {
  const expectedName = path.name.trim();
  const name = proposedName.trim();
  if (!path.id || path.id.trim() !== path.id || !validPathName(expectedName) || !validPathName(name)) {
    throw new Error('invalid Path rename review');
  }
  return Object.freeze({
    pathId: path.id,
    expectedName,
    name,
    changed: expectedName !== name,
  });
}

export function applyPathRenameResult<
  P extends PathRenamePath,
  S extends {
    paths: P[];
    selectedPath?: P | null;
  },
>(state: S, path: NoInfer<P>): S {
  return {
    ...state,
    paths: state.paths.map((candidate) => candidate.id === path.id ? path : candidate),
    selectedPath: state.selectedPath?.id === path.id ? path : state.selectedPath,
  };
}

export function createPathRenameOperationOwner(keyFactory: () => string) {
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
    async submit<P extends PathRenamePath>(
      review: PathRenameReview,
      request: (
        pathId: string,
        body: PathRenameRequestBody,
        idempotencyKey: string,
      ) => Promise<P>,
      confirmed = true,
    ): Promise<PathRenameMutationResult<P>> {
      if (!confirmed) {
        cancel(review.pathId);
        return { kind: 'cancelled' };
      }
      if (!review.changed) {
        throw new Error('unchanged Path rename cannot be submitted');
      }

      const signature = `${review.expectedName}\0${review.name}`;
      const epoch = (epochs[review.pathId] ?? 0) + 1;
      epochs[review.pathId] = epoch;
      const previous = retries[review.pathId];
      const retry = previous?.signature === signature
        ? previous
        : {
            signature,
            idempotencyKey: keyFactory(),
            body: Object.freeze({
              expectedName: review.expectedName,
              name: review.name,
            }),
          };
      if (!validIdempotencyKey(retry.idempotencyKey)) {
        throw new Error('invalid Path rename idempotency key');
      }
      retries[review.pathId] = retry;

      try {
        const path = await request(review.pathId, retry.body, retry.idempotencyKey);
        if (epochs[review.pathId] !== epoch) return { kind: 'superseded' };
        if (path.id !== review.pathId || path.name !== review.name) {
          throw new Error('invalid Path rename result');
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
