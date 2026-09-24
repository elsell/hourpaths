import type {
  PathInvitation,
  PathInvitationFailure,
  PathInvitationProblemCode,
  PathInvitationRecipient,
} from './path-invitations';

export type ManagedPendingPathInvitation = Readonly<{
  invitation: PathInvitation;
  inviter: PathInvitationRecipient;
  recipient: PathInvitationRecipient;
}>;

export type ManagedPendingPathInvitationState = Readonly<{
  items: readonly ManagedPendingPathInvitation[];
  nextCursor: string;
}>;

export type ManagedPendingPathInvitationPage = Readonly<{
  items: readonly ManagedPendingPathInvitation[];
  nextCursor: string;
}>;

export type PathInvitationCancellation = Readonly<{
  invitationId: string;
  canceledAt: string;
}>;

export type PathInvitationCancelResult =
  | Readonly<{ kind: 'canceled'; cancellation: PathInvitationCancellation }>
  | Readonly<{ kind: 'failed'; failure: PathInvitationFailure }>
  | Readonly<{ kind: 'superseded' }>
  | Readonly<{ kind: 'cancelled' }>;

const problemCodes = new Set<PathInvitationProblemCode>([
  'bad_request',
  'validation_failed',
  'unauthenticated',
  'invalid_credential',
  'conflict',
  'idempotency_conflict',
  'rate_limited',
  'authorization_pending',
  'authorization_dead_lettered',
  'authorization_policy_not_configured',
  'unavailable',
  'internal_error',
  'request_failed',
]);

function validIdentifier(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value;
}

function validInstant(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && Number.isFinite(Date.parse(value));
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

function validatedIdentity(value: unknown): PathInvitationRecipient | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  if (
    keys.length !== 3 ||
    keys[0] !== 'displayName' ||
    keys[1] !== 'userId' ||
    keys[2] !== 'username' ||
    !validIdentifier(record.userId) ||
    !validIdentifier(record.username) ||
    !validIdentifier(record.displayName)
  ) {
    return undefined;
  }
  return Object.freeze({
    userId: record.userId,
    username: record.username,
    displayName: record.displayName,
  });
}

function validatedPendingInvitation(value: unknown): PathInvitation | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  if (
    keys.length !== 6 ||
    keys.join('\0') !== [
      'createdAt', 'id', 'inviterUserId', 'offeredRole', 'pathId', 'recipientUserId',
    ].join('\0') ||
    !validIdentifier(record.id) ||
    !validIdentifier(record.pathId) ||
    !validIdentifier(record.inviterUserId) ||
    !validIdentifier(record.recipientUserId) ||
    (record.offeredRole !== 'participant' && record.offeredRole !== 'supporter') ||
    !validInstant(record.createdAt)
  ) {
    return undefined;
  }
  return Object.freeze({
    id: record.id,
    pathId: record.pathId,
    inviterUserId: record.inviterUserId,
    recipientUserId: record.recipientUserId,
    offeredRole: record.offeredRole,
    createdAt: record.createdAt,
  });
}

function validatedManagedPendingInvitation(
  value: unknown,
): ManagedPendingPathInvitation | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  if (
    keys.length !== 3 ||
    keys[0] !== 'invitation' ||
    keys[1] !== 'inviter' ||
    keys[2] !== 'recipient'
  ) {
    return undefined;
  }
  const invitation = validatedPendingInvitation(record.invitation);
  const inviter = validatedIdentity(record.inviter);
  const recipient = validatedIdentity(record.recipient);
  if (
    !invitation ||
    !inviter ||
    !recipient ||
    inviter.userId !== invitation.inviterUserId ||
    recipient.userId !== invitation.recipientUserId
  ) {
    return undefined;
  }
  return Object.freeze({ invitation, inviter, recipient });
}

export function mergeManagedPendingInvitationPage(
  current: ManagedPendingPathInvitationState,
  page: ManagedPendingPathInvitationPage,
  requestedCursor: string,
): ManagedPendingPathInvitationState {
  if (
    typeof current.nextCursor !== 'string' ||
    typeof page.nextCursor !== 'string' ||
    typeof requestedCursor !== 'string'
  ) {
    throw new Error('invalid invitation cursor');
  }
  if (requestedCursor !== '' && current.nextCursor !== requestedCursor) {
    throw new Error('stale invitation cursor');
  }
  const base = requestedCursor === '' ? [] : [...current.items];
  const indexes = new Map<string, number>();
  base.forEach((candidate, index) => {
    const invitation = validatedManagedPendingInvitation(candidate);
    if (!invitation) throw new Error('invalid managed pending invitation');
    indexes.set(invitation.invitation.id, index);
    base[index] = invitation;
  });
  for (const candidate of page.items) {
    const invitation = validatedManagedPendingInvitation(candidate);
    if (!invitation) throw new Error('invalid managed pending invitation');
    const existing = indexes.get(invitation.invitation.id);
    if (existing === undefined) {
      indexes.set(invitation.invitation.id, base.length);
      base.push(invitation);
    } else {
      base[existing] = invitation;
    }
  }
  return Object.freeze({ items: Object.freeze(base), nextCursor: page.nextCursor });
}

function retainedFailure(value: unknown): PathInvitationFailure {
  if (!value || typeof value !== 'object') return { kind: 'unexpected' };
  const failure = value as { kind?: unknown; status?: unknown; code?: unknown };
  switch (failure.kind) {
    case 'network':
    case 'opaque':
    case 'warning_required':
    case 'invalid_response':
    case 'unexpected':
      return { kind: failure.kind };
    case 'http': {
      if (!Number.isInteger(failure.status)) return { kind: 'unexpected' };
      const status = failure.status as number;
      return typeof failure.code === 'string' && problemCodes.has(failure.code as PathInvitationProblemCode)
        ? { kind: 'http', status, code: failure.code as PathInvitationProblemCode }
        : { kind: 'http', status };
    }
    default:
      return { kind: 'unexpected' };
  }
}

function validatedCancellation(value: unknown): PathInvitationCancellation | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  if (
    keys.length !== 2 ||
    keys[0] !== 'canceledAt' ||
    keys[1] !== 'invitationId' ||
    !validIdentifier(record.invitationId) ||
    !validInstant(record.canceledAt)
  ) {
    return undefined;
  }
  return Object.freeze({ invitationId: record.invitationId, canceledAt: record.canceledAt });
}

export function createPathInvitationCancelOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, Readonly<{ pathId: string; key: string }> | undefined> = {};

  function cancel(invitationId?: string): void {
    if (invitationId) {
      epochs[invitationId] = (epochs[invitationId] ?? 0) + 1;
      delete retries[invitationId];
      return;
    }
    for (const id of Object.keys(epochs)) epochs[id] = (epochs[id] ?? 0) + 1;
    for (const id of Object.keys(retries)) delete retries[id];
  }

  return {
    async submit(
      pathId: string,
      invitationId: string,
      confirmed: boolean,
      request: (pathId: string, invitationId: string, idempotencyKey: string) => Promise<unknown>,
    ): Promise<PathInvitationCancelResult> {
      if (!validIdentifier(pathId) || !validIdentifier(invitationId)) {
        return { kind: 'failed', failure: { kind: 'invalid_response' } };
      }
      if (!confirmed) {
        cancel(invitationId);
        return { kind: 'cancelled' };
      }
      const epoch = (epochs[invitationId] ?? 0) + 1;
      epochs[invitationId] = epoch;
      const previous = retries[invitationId];
      const retry = previous?.pathId === pathId
        ? previous
        : Object.freeze({ pathId, key: keyFactory() });
      if (!validIdempotencyKey(retry.key)) {
        throw new Error('invalid Path invitation cancellation idempotency key');
      }
      retries[invitationId] = retry;
      try {
        const response = await request(pathId, invitationId, retry.key);
        if (epochs[invitationId] !== epoch) return { kind: 'superseded' };
        const cancellation = validatedCancellation(response);
        if (!cancellation || cancellation.invitationId !== invitationId) {
          return { kind: 'failed', failure: { kind: 'invalid_response' } };
        }
        delete retries[invitationId];
        return { kind: 'canceled', cancellation };
      } catch (cause) {
        return epochs[invitationId] === epoch
          ? { kind: 'failed', failure: retainedFailure(cause) }
          : { kind: 'superseded' };
      }
    },
    cancel,
  };
}
