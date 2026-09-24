export type PathMemberAccessRole = 'administrator' | 'participant' | 'supporter';

export type PathMemberRoleChangeReview = Readonly<{
  displayName: string;
  pathId: string;
  previousRole: PathMemberAccessRole;
  role: PathMemberAccessRole;
  runningTimer: boolean;
  sessionCount: number;
  totalTrackedSeconds: number;
  userId: string;
  username: string;
}>;

export type PathMemberRoleChangeBody = Readonly<{
  confirmed: true;
  expectedRole: PathMemberAccessRole;
  role: PathMemberAccessRole;
}>;

export type PathMemberRoleChangeReceipt = Readonly<{
  activityDeleted: boolean;
  pathId: string;
  role: PathMemberAccessRole;
  userId: string;
}>;

export type PathMemberRoleChangeResult =
  | { kind: 'applied'; receipt: PathMemberRoleChangeReceipt }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' }
  | { kind: 'cancelled' };

function validText(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

function validCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function accessRole(value: unknown): value is PathMemberAccessRole {
  return value === 'administrator' || value === 'participant' || value === 'supporter';
}

function supportedTransition(previousRole: PathMemberAccessRole, role: PathMemberAccessRole): boolean {
  return (previousRole === 'participant' && (role === 'supporter' || role === 'administrator'))
    || (previousRole === 'supporter' && role === 'participant')
    || (previousRole === 'administrator' && role === 'participant');
}

export function reviewPathMemberRoleChange(
  member: Readonly<{ displayName: string; pathId: string; role: string; runningTimer: boolean; sessionCount: number; totalTrackedSeconds: number; userId: string; username: string }>,
  role: PathMemberAccessRole,
): PathMemberRoleChangeReview {
  if (!validText(member.displayName) || !validText(member.pathId) || !validText(member.userId) || !validText(member.username)
    || !accessRole(member.role) || !accessRole(role) || typeof member.runningTimer !== 'boolean'
    || !validCount(member.sessionCount) || !validCount(member.totalTrackedSeconds)) throw new Error('invalid supported Path member role review');
  if (member.role === role) throw new Error('Path member role change requires a different role');
  if (!supportedTransition(member.role, role)) throw new Error('unsupported Path member role transition');
  return Object.freeze({ ...member, previousRole: member.role, role });
}

function validatedReceipt(value: unknown, review: PathMemberRoleChangeReview): PathMemberRoleChangeReceipt {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid Path member role change result');
  const record = value as Record<string, unknown>;
  if (Object.keys(record).length !== 4 || record.pathId !== review.pathId || record.userId !== review.userId
    || record.role !== review.role || record.activityDeleted !== (review.role === 'supporter')) throw new Error('invalid Path member role change result');
  return Object.freeze({ activityDeleted: review.role === 'supporter', pathId: review.pathId, role: review.role, userId: review.userId });
}

export function applyPathMemberRoleChangeResult<M extends Readonly<{ pathId: string; role: string; sessionCount: number; totalTrackedSeconds: number; userId: string }>>(
  state: Readonly<{ members: readonly M[] }>, result: PathMemberRoleChangeResult,
): { members: M[] } {
  if (result.kind !== 'applied') return { members: [...state.members] };
  return { members: state.members.map((member) => member.pathId === result.receipt.pathId && member.userId === result.receipt.userId
    ? { ...member, role: result.receipt.role, ...(result.receipt.activityDeleted ? { runningTimer: false, sessionCount: 0, totalTrackedSeconds: 0 } : {}) }
    : member) };
}

export function createPathMemberRoleChangeOperationOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, Readonly<{ body: PathMemberRoleChangeBody; idempotencyKey: string }> | undefined> = {};
  const operationKey = (pathId: string, userId: string) => `${pathId}:${userId}`;
  const cancel = (pathId?: string, userId?: string) => {
    const keys = pathId && userId ? [operationKey(pathId, userId)] : Object.keys(epochs);
    for (const key of keys) { epochs[key] = (epochs[key] ?? 0) + 1; delete retries[key]; }
  };
  return {
    async submit(review: PathMemberRoleChangeReview, confirmed: boolean, request: (
      pathId: string, userId: string, body: PathMemberRoleChangeBody, idempotencyKey: string,
    ) => Promise<unknown>): Promise<PathMemberRoleChangeResult> {
      if (!confirmed) { cancel(review.pathId, review.userId); return { kind: 'cancelled' }; }
      const validated = reviewPathMemberRoleChange({ ...review, role: review.previousRole }, review.role);
      const key = operationKey(validated.pathId, validated.userId);
      const epoch = (epochs[key] ?? 0) + 1;
      epochs[key] = epoch;
      const retry = retries[key] ?? Object.freeze({
        body: Object.freeze({ confirmed: true, expectedRole: validated.previousRole, role: validated.role }),
        idempotencyKey: keyFactory(),
      });
      if (retry.idempotencyKey.length < 16 || retry.idempotencyKey.length > 128 || !/^[\x20-\x7e]+$/u.test(retry.idempotencyKey)) throw new Error('invalid Path member role idempotency key');
      if (retry.body.expectedRole !== validated.previousRole || retry.body.role !== validated.role) throw new Error('Path member role changed');
      retries[key] = retry;
      try {
        const receipt = validatedReceipt(await request(validated.pathId, validated.userId, retry.body, retry.idempotencyKey), validated);
        if (epochs[key] !== epoch) return { kind: 'superseded' };
        delete retries[key];
        return { kind: 'applied', receipt };
      } catch (cause) {
        return epochs[key] === epoch ? { kind: 'failed', cause } : { kind: 'superseded' };
      }
    },
    cancel,
  };
}
