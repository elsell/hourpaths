export type PathVisibility = 'private' | 'followers' | 'public';
export type ProfileVisibility = 'private' | 'public';

export type PathVisibilityReviewPath = Readonly<{
  id: string;
  name: string;
  visibility: PathVisibility;
}>;

type ReviewFields = Readonly<{
  pathId: string;
  pathName: string;
  current: PathVisibility;
  proposed: PathVisibility;
}>;

export type PathVisibilityChangeReview =
  | (ReviewFields & Readonly<{ kind: 'unchanged' }>)
  | (ReviewFields & Readonly<{ kind: 'not-permitted' }>)
  | (ReviewFields & Readonly<{ kind: 'ready'; broader: boolean }>);

export type PathVisibilityChangeBody = Readonly<{
  confirmed: true;
  expectedVisibility: PathVisibility;
  visibility: PathVisibility;
}>;

export type PathVisibilityProjection = PathVisibilityReviewPath & Readonly<{
  capabilities: object;
  archivedAt?: string | null;
}>;

export type PathVisibilityMutationResult<P extends PathVisibilityProjection> =
  | Readonly<{ kind: 'applied'; path: P }>
  | Readonly<{ kind: 'failed'; cause: unknown }>
  | Readonly<{ kind: 'superseded' }>
  | Readonly<{ kind: 'cancelled' }>;

const visibilityOrder = Object.freeze({ private: 0, followers: 1, public: 2 } as const);
const privateProfileOptions = Object.freeze(['private', 'followers'] as const);
const publicProfileOptions = Object.freeze(['private', 'followers', 'public'] as const);

function validIdentifier(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value;
}

function validVisibility(value: unknown): value is PathVisibility {
  return value === 'private' || value === 'followers' || value === 'public';
}

export function pathVisibilityFromAPI(value: unknown): PathVisibility {
  if (!validVisibility(value)) throw new Error('invalid Path visibility');
  return value;
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

function validActiveProjection(value: unknown): value is PathVisibilityProjection {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Record<string, unknown>;
  return validIdentifier(candidate.id) &&
    validIdentifier(candidate.name) &&
    validVisibility(candidate.visibility) &&
    candidate.capabilities !== null &&
    typeof candidate.capabilities === 'object' &&
    (candidate.archivedAt === undefined || candidate.archivedAt === null);
}

export function pathVisibilityOptions(
  profileVisibility: ProfileVisibility,
): readonly PathVisibility[] {
  if (profileVisibility !== 'private' && profileVisibility !== 'public') {
    throw new Error('invalid profile visibility');
  }
  return profileVisibility === 'private' ? privateProfileOptions : publicProfileOptions;
}

export function comparePathVisibility(left: PathVisibility, right: PathVisibility): number {
  if (!validVisibility(left) || !validVisibility(right)) throw new Error('invalid Path visibility');
  return visibilityOrder[left] - visibilityOrder[right];
}

export function reviewPathVisibilityChange(
  path: PathVisibilityReviewPath,
  proposed: PathVisibility,
  profileVisibility: ProfileVisibility,
): PathVisibilityChangeReview {
  if (!validIdentifier(path.id) || !validIdentifier(path.name) || !validVisibility(path.visibility) ||
    !validVisibility(proposed) || (profileVisibility !== 'private' && profileVisibility !== 'public')) {
    throw new Error('invalid Path visibility state');
  }
  const fields = Object.freeze({
    pathId: path.id,
    pathName: path.name,
    current: path.visibility,
    proposed,
  });
  if (proposed === path.visibility) return Object.freeze({ kind: 'unchanged', ...fields });
  if (!pathVisibilityOptions(profileVisibility).includes(proposed)) {
    return Object.freeze({ kind: 'not-permitted', ...fields });
  }
  return Object.freeze({
    kind: 'ready',
    ...fields,
    broader: comparePathVisibility(proposed, path.visibility) > 0,
  });
}

export function createPathVisibilityOperationOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, Readonly<{ key: string; signature: string }> | undefined> = {};

  function cancel(pathId?: string): void {
    if (pathId) {
      epochs[pathId] = (epochs[pathId] ?? 0) + 1;
      delete retries[pathId];
      return;
    }
    for (const id of Object.keys(epochs)) epochs[id] = (epochs[id] ?? 0) + 1;
    for (const id of Object.keys(retries)) delete retries[id];
  }

  return {
    async submit<P extends PathVisibilityProjection>(
      review: Extract<PathVisibilityChangeReview, { kind: 'ready' }>,
      confirmed: boolean,
      request: (
        pathId: string,
        body: PathVisibilityChangeBody,
        idempotencyKey: string,
      ) => Promise<P>,
    ): Promise<PathVisibilityMutationResult<P>> {
      if (!confirmed) return { kind: 'cancelled' };
      const signature = `${review.current}\0${review.proposed}`;
      const previous = retries[review.pathId];
      const retry = previous?.signature === signature
        ? previous
        : Object.freeze({ signature, key: keyFactory() });
      if (!validIdempotencyKey(retry.key)) throw new Error('invalid Path visibility idempotency key');
      retries[review.pathId] = retry;
      const epoch = (epochs[review.pathId] ?? 0) + 1;
      epochs[review.pathId] = epoch;
      const body = Object.freeze({
        confirmed: true as const,
        expectedVisibility: review.current,
        visibility: review.proposed,
      });
      try {
        const projection = await request(review.pathId, body, retry.key);
        if (epochs[review.pathId] !== epoch) return { kind: 'superseded' };
        if (
          !validActiveProjection(projection) ||
          projection.id !== review.pathId ||
          projection.visibility !== review.proposed
        ) {
          return { kind: 'failed', cause: new Error('invalid authoritative Path visibility projection') };
        }
        delete retries[review.pathId];
        return Object.freeze({ kind: 'applied', path: projection });
      } catch (cause) {
        return epochs[review.pathId] === epoch
          ? { kind: 'failed', cause }
          : { kind: 'superseded' };
      }
    },
    cancel,
  };
}

export function applyPathVisibilityResult<P extends PathVisibilityProjection>(
  state: Readonly<{
    activePaths: readonly P[];
    archivedPaths: readonly P[];
    selectedPath: P | null;
  }>,
  authoritativePath: P,
): Readonly<{
  activePaths: P[];
  archivedPaths: P[];
  selectedPath: P | null;
}> {
  if (!validActiveProjection(authoritativePath)) {
    throw new Error('invalid authoritative active Path');
  }
  const matches = state.activePaths.filter(({ id }) => id === authoritativePath.id);
  if (matches.length !== 1 || state.archivedPaths.some(({ id }) => id === authoritativePath.id)) {
    throw new Error('authoritative Path is not one current active Path');
  }
  return Object.freeze({
    activePaths: state.activePaths.map((path) => path.id === authoritativePath.id ? authoritativePath : path),
    archivedPaths: [...state.archivedPaths],
    selectedPath: state.selectedPath?.id === authoritativePath.id ? authoritativePath : state.selectedPath,
  });
}
