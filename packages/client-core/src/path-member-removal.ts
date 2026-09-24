export type PathMemberRemovalRole = 'participant' | 'supporter';

export type PathMemberRemovalReview = Readonly<{
  displayName: string;
  pathId: string;
  role: PathMemberRemovalRole;
  runningTimer: boolean;
  sessionCount: number;
  totalTrackedSeconds: number;
  userId: string;
  username: string;
}>;

export type PathMemberRemovalRequestBody = Readonly<{
  confirmed: true;
  expectedRole: PathMemberRemovalRole;
}>;

export type PathMemberRemovalReceipt = Readonly<{
  activityDeleted: boolean;
  pathId: string;
  removed: true;
  userId: string;
}>;

export type PathMemberRemovalMutationResult =
  | { kind: 'applied'; receipt: PathMemberRemovalReceipt }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' }
  | { kind: 'cancelled' };

type Retry = Readonly<{
  body: PathMemberRemovalRequestBody;
  idempotencyKey: string;
}>;

function validText(value: unknown): value is string {
  return typeof value === 'string'
    && value.length > 0
    && value.trim() === value
    && !/[\u0000-\u001f\u007f]/u.test(value);
}

function validCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/u.test(value);
}

function operationKey(pathId: string, userId: string): string {
  return `${pathId}:${userId}`;
}

function receipt(value: unknown, review: PathMemberRemovalReview): PathMemberRemovalReceipt {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid Path member removal result');
  const record = value as Record<string, unknown>;
  const expectedActivityDeleted = review.role === 'participant';
  if (
    Object.keys(record).length !== 4
    || record.activityDeleted !== expectedActivityDeleted
    || record.pathId !== review.pathId
    || record.removed !== true
    || record.userId !== review.userId
  ) {
    throw new Error('invalid Path member removal result');
  }
  return Object.freeze({
    activityDeleted: expectedActivityDeleted,
    pathId: review.pathId,
    removed: true,
    userId: review.userId,
  });
}

export function reviewPathMemberRemoval(
  review: Omit<PathMemberRemovalReview, 'role'> & Readonly<{ role: string }>,
): PathMemberRemovalReview {
  if (
    !validText(review.displayName)
    || !validText(review.pathId)
    || !validText(review.userId)
    || !validText(review.username)
    || (review.role !== 'participant' && review.role !== 'supporter')
    || typeof review.runningTimer !== 'boolean'
    || !validCount(review.sessionCount)
    || !validCount(review.totalTrackedSeconds)
  ) {
    throw new Error('invalid ordinary Path member removal review');
  }
  return Object.freeze({ ...review, role: review.role as PathMemberRemovalRole });
}

export function applyPathMemberRemovalResult<
  M extends Readonly<{ pathId: string; userId: string }>,
  R extends Readonly<{ pathId: string; userId: string }>,
  S extends { members: M[]; selected?: R | null },
>(state: S, result: PathMemberRemovalMutationResult): S {
  if (result.kind !== 'applied') return state;
  return {
    ...state,
    members: state.members.filter((member) => (
      member.pathId !== result.receipt.pathId || member.userId !== result.receipt.userId
    )),
    selected: state.selected?.pathId === result.receipt.pathId
      && state.selected.userId === result.receipt.userId
      ? null
      : state.selected,
  };
}

export function createPathMemberRemovalOperationOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, Retry | undefined> = {};

  function cancel(pathId?: string, userId?: string): void {
    if (pathId && userId) {
      const key = operationKey(pathId, userId);
      epochs[key] = (epochs[key] ?? 0) + 1;
      delete retries[key];
      return;
    }
    for (const key of Object.keys(epochs)) epochs[key] = (epochs[key] ?? 0) + 1;
    for (const key of Object.keys(retries)) delete retries[key];
  }

  return {
    async submit(
      unvalidatedReview: PathMemberRemovalReview,
      confirmed: boolean,
      request: (
        pathId: string,
        userId: string,
        body: PathMemberRemovalRequestBody,
        idempotencyKey: string,
      ) => Promise<unknown>,
    ): Promise<PathMemberRemovalMutationResult> {
      const review = reviewPathMemberRemoval(unvalidatedReview);
      if (!confirmed) {
        cancel(review.pathId, review.userId);
        return { kind: 'cancelled' };
      }
      const key = operationKey(review.pathId, review.userId);
      const epoch = (epochs[key] ?? 0) + 1;
      epochs[key] = epoch;
      const retry = retries[key] ?? Object.freeze({
        body: Object.freeze({ confirmed: true, expectedRole: review.role }),
        idempotencyKey: keyFactory(),
      });
      if (!validIdempotencyKey(retry.idempotencyKey)) throw new Error('invalid Path member removal idempotency key');
      if (retry.body.expectedRole !== review.role) throw new Error('Path member removal role changed');
      retries[key] = retry;
      try {
        const acknowledged = receipt(
          await request(review.pathId, review.userId, retry.body, retry.idempotencyKey),
          review,
        );
        if (epochs[key] !== epoch) return { kind: 'superseded' };
        delete retries[key];
        return { kind: 'applied', receipt: acknowledged };
      } catch (cause) {
        return epochs[key] === epoch ? { kind: 'failed', cause } : { kind: 'superseded' };
      }
    },
    cancel,
  };
}
