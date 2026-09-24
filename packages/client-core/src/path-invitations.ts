export type PathInvitationRole = 'participant' | 'supporter';

export type PathInvitationRecipient = Readonly<{
  userId: string;
  username: string;
  displayName: string;
}>;

export type PathInvitationRecipientReview = Readonly<{
  pathId: string;
  requestedUsername: string;
  recipient: PathInvitationRecipient;
}>;

export type PathInvitation = Readonly<{
  id: string;
  pathId: string;
  inviterUserId: string;
  recipientUserId: string;
  offeredRole: PathInvitationRole;
  createdAt: string;
  acceptedAt?: string | null;
}>;

export type PathInvitationVisibilityWarning = Readonly<{
  pathVisibility: 'followers' | 'public';
  hasRetainedActivity: boolean;
}>;

export type PendingPathInvitation = Readonly<{
  invitation: PathInvitation;
  pathName: string;
  inviter: PathInvitationRecipient;
  warning?: PathInvitationVisibilityWarning;
}>;

export type PathInvitationAcceptanceReview =
  | Readonly<{
      kind: 'confirmation-required';
      invitationId: string;
      warning: PathInvitationVisibilityWarning;
    }>
  | Readonly<{
      kind: 'ready';
      invitationId: string;
    }>;

export type PathInvitationAcceptBody = Readonly<{
  visibilityWarningAcknowledgement: Readonly<{
    pathVisibility: 'followers' | 'public';
  }>;
}>;

export type PathInvitationFailure =
  | Readonly<{ kind: 'network' }>
  | Readonly<{ kind: 'opaque' }>
  | Readonly<{ kind: 'warning_required' }>
  | Readonly<{ kind: 'http'; status: number; code?: PathInvitationProblemCode }>
  | Readonly<{ kind: 'invalid_response' }>
  | Readonly<{ kind: 'unexpected' }>;

export type PathInvitationFailureMessageKey =
  | 'pathInvitation.unavailable'
  | 'pathInvitation.warningRequired'
  | 'pathInvitation.retry'
  | 'pathInvitation.rateLimited'
  | 'pathInvitation.dependencyUnavailable'
  | 'pathInvitation.failure';

export type PathInvitationProblemCode =
  | 'bad_request'
  | 'validation_failed'
  | 'unauthenticated'
  | 'invalid_credential'
  | 'conflict'
  | 'idempotency_conflict'
  | 'rate_limited'
  | 'authorization_pending'
  | 'authorization_dead_lettered'
  | 'authorization_policy_not_configured'
  | 'unavailable'
  | 'internal_error'
  | 'request_failed';

export type PathInvitationReviewResult =
  | Readonly<{ kind: 'reviewed'; review: PathInvitationRecipientReview }>
  | Readonly<{ kind: 'failed'; failure: PathInvitationFailure }>
  | Readonly<{ kind: 'superseded' }>;

export type PathInvitationSendBody = Readonly<{
  username: string;
  expectedRecipientUserId: string;
  offeredRole: PathInvitationRole;
}>;

export type PathInvitationSendResult =
  | Readonly<{ kind: 'sent'; invitation: PathInvitation }>
  | Readonly<{ kind: 'failed'; failure: PathInvitationFailure }>
  | Readonly<{ kind: 'superseded' }>
  | Readonly<{ kind: 'cancelled' }>;

export type PendingPathInvitationState = Readonly<{
  items: readonly PendingPathInvitation[];
  nextCursor: string;
}>;

export type PendingPathInvitationPage = Readonly<{
  items: readonly PendingPathInvitation[];
  nextCursor: string;
}>;

export type PathInvitationAcceptResult =
  | Readonly<{ kind: 'accepted'; invitation: PathInvitation }>
  | Readonly<{ kind: 'failed'; failure: PathInvitationFailure }>
  | Readonly<{ kind: 'superseded' }>;

export type PathInvitationAcceptSubmissionResult =
  | PathInvitationAcceptResult
  | Readonly<{ kind: 'cancelled' }>;

export type PathInvitationRejection = Readonly<{
  invitationId: string;
  rejectedAt: string;
  unreadCount: number;
}>;

export type PathInvitationRejectResult =
  | Readonly<{ kind: 'rejected'; rejection: PathInvitationRejection }>
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

function validRole(value: unknown): value is PathInvitationRole {
  return value === 'participant' || value === 'supporter';
}

function validatedVisibilityWarning(
  value: unknown,
): PathInvitationVisibilityWarning | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  if (
    keys.length !== 2 ||
    keys[0] !== 'hasRetainedActivity' ||
    keys[1] !== 'pathVisibility' ||
    (record.pathVisibility !== 'followers' && record.pathVisibility !== 'public') ||
    typeof record.hasRetainedActivity !== 'boolean'
  ) {
    return undefined;
  }
  return Object.freeze({
    pathVisibility: record.pathVisibility,
    hasRetainedActivity: record.hasRetainedActivity,
  });
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

function validatedRecipient(value: unknown, requestedUsername: string): PathInvitationRecipient | undefined {
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
    !validIdentifier(record.displayName) ||
    (requestedUsername !== '' &&
      record.username.toLocaleLowerCase('en-US') !== requestedUsername.toLocaleLowerCase('en-US'))
  ) {
    return undefined;
  }
  return Object.freeze({
    userId: record.userId,
    username: record.username,
    displayName: record.displayName,
  });
}

function validatedInvitation(value: unknown, accepted: boolean): PathInvitation | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  if (
    !validIdentifier(record.id) ||
    !validIdentifier(record.pathId) ||
    !validIdentifier(record.inviterUserId) ||
    !validIdentifier(record.recipientUserId) ||
    !validRole(record.offeredRole) ||
    !validInstant(record.createdAt)
  ) {
    return undefined;
  }
  const acceptedAt = record.acceptedAt;
  if (accepted ? !validInstant(acceptedAt) : acceptedAt !== undefined && acceptedAt !== null) {
    return undefined;
  }
  if (accepted && Date.parse(acceptedAt as string) < Date.parse(record.createdAt)) return undefined;
  return {
    id: record.id,
    pathId: record.pathId,
    inviterUserId: record.inviterUserId,
    recipientUserId: record.recipientUserId,
    offeredRole: record.offeredRole,
    createdAt: record.createdAt,
    ...(acceptedAt ? { acceptedAt: acceptedAt as string } : {}),
  };
}

function validatedPendingInvitation(value: unknown): PendingPathInvitation | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  const hasWarning = Object.hasOwn(record, 'warning');
  if (
    keys.length !== (hasWarning ? 4 : 3) ||
    keys[0] !== 'invitation' ||
    keys[1] !== 'inviter' ||
    keys[2] !== 'pathName' ||
    (hasWarning && keys[3] !== 'warning') ||
    !validIdentifier(record.pathName)
  ) {
    return undefined;
  }
  const invitation = validatedInvitation(record.invitation, false);
  const inviter = validatedRecipient(record.inviter, '');
  const warning = hasWarning
    ? validatedVisibilityWarning(record.warning)
    : undefined;
  if (
    !invitation ||
    !inviter ||
    inviter.userId !== invitation.inviterUserId ||
    (hasWarning && !warning)
  ) {
    return undefined;
  }
  return Object.freeze({
    invitation,
    pathName: record.pathName,
    inviter,
    ...(warning ? { warning } : {}),
  });
}

export function reviewPendingPathInvitationAcceptance(
  pendingInvitation: PendingPathInvitation,
): PathInvitationAcceptanceReview {
  const retained = validatedPendingInvitation(pendingInvitation);
  if (!retained) throw new Error('invalid pending invitation');
  return retained.warning
    ? Object.freeze({
        kind: 'confirmation-required',
        invitationId: retained.invitation.id,
        warning: retained.warning,
      })
    : Object.freeze({
        kind: 'ready',
        invitationId: retained.invitation.id,
      });
}

export function pathInvitationOutputData(value: unknown): unknown {
  if (
    !value ||
    typeof value !== 'object' ||
    !Object.hasOwn(value, 'data') ||
    (value as { data?: unknown }).data === undefined
  ) {
    throw { kind: 'invalid_response' } as const;
  }
  return (value as { data: unknown }).data;
}

function isFailure(value: unknown): value is PathInvitationFailure {
  if (!value || typeof value !== 'object') return false;
  const failure = value as { kind?: unknown; status?: unknown; code?: unknown };
  switch (failure.kind) {
    case 'network':
    case 'opaque':
    case 'warning_required':
    case 'invalid_response':
    case 'unexpected':
      return true;
    case 'http':
      return Number.isInteger(failure.status) &&
        (failure.code === undefined || problemCodes.has(failure.code as PathInvitationProblemCode));
    default:
      return false;
  }
}

function retainedFailure(value: unknown): PathInvitationFailure {
  if (!isFailure(value)) return { kind: 'unexpected' };
  switch (value.kind) {
    case 'http':
      return value.code === undefined
        ? { kind: 'http', status: value.status }
        : { kind: 'http', status: value.status, code: value.code };
    case 'network':
      return { kind: 'network' };
    case 'opaque':
      return { kind: 'opaque' };
    case 'warning_required':
      return { kind: 'warning_required' };
    case 'invalid_response':
      return { kind: 'invalid_response' };
    case 'unexpected':
      return { kind: 'unexpected' };
  }
}

export function pathInvitationFailureFromProblem(
  status: number,
  problem?: unknown,
): PathInvitationFailure {
  if (!Number.isInteger(status) || status < 100 || status > 599) {
    return { kind: 'invalid_response' };
  }
  if (status === 404) return { kind: 'opaque' };
  const rawCode = problem && typeof problem === 'object'
    ? (problem as { code?: unknown }).code
    : undefined;
  if (status === 409 && rawCode === 'invitation_warning_required') {
    return { kind: 'warning_required' };
  }
  const code = typeof rawCode === 'string' && problemCodes.has(rawCode as PathInvitationProblemCode)
    ? rawCode as PathInvitationProblemCode
    : undefined;
  return code === undefined ? { kind: 'http', status } : { kind: 'http', status, code };
}

export function pathInvitationFailureMessageKey(
  failure: PathInvitationFailure,
): PathInvitationFailureMessageKey {
  switch (failure.kind) {
    case 'opaque':
      return 'pathInvitation.unavailable';
    case 'warning_required':
      return 'pathInvitation.warningRequired';
    case 'network':
      return 'pathInvitation.retry';
    case 'http':
      if (failure.code === 'rate_limited') return 'pathInvitation.rateLimited';
      if (
        failure.code === 'authorization_pending' ||
        failure.code === 'authorization_dead_lettered' ||
        failure.code === 'authorization_policy_not_configured' ||
        failure.code === 'unavailable' ||
        failure.code === 'internal_error' ||
        failure.status >= 500
      ) {
        return 'pathInvitation.dependencyUnavailable';
      }
      return 'pathInvitation.failure';
    case 'invalid_response':
    case 'unexpected':
      return 'pathInvitation.dependencyUnavailable';
  }
}

export function createPathInvitationRecipientReviewOwner() {
  let epoch = 0;

  return {
    async review(
      pathId: string,
      exactUsername: string,
      request: (pathId: string, exactUsername: string) => Promise<unknown>,
    ): Promise<PathInvitationReviewResult> {
      if (!validIdentifier(pathId) || !validIdentifier(exactUsername)) {
        return { kind: 'failed', failure: { kind: 'invalid_response' } };
      }
      const operationEpoch = ++epoch;
      try {
        const response = await request(pathId, exactUsername);
        if (epoch !== operationEpoch) return { kind: 'superseded' };
        const recipient = validatedRecipient(response, exactUsername);
        if (!recipient) return { kind: 'failed', failure: { kind: 'invalid_response' } };
        return {
          kind: 'reviewed',
          review: Object.freeze({
            pathId,
            requestedUsername: exactUsername,
            recipient,
          }),
        };
      } catch (cause) {
        return epoch === operationEpoch
          ? { kind: 'failed', failure: retainedFailure(cause) }
          : { kind: 'superseded' };
      }
    },
    cancel(): void {
      epoch += 1;
    },
  };
}

type SendRetry = {
  signature: string;
  idempotencyKey: string;
  body: PathInvitationSendBody;
};

export function createPathInvitationSendOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, SendRetry | undefined> = {};

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
    async submit(
      review: PathInvitationRecipientReview,
      role: PathInvitationRole,
      confirmed: boolean,
      request: (
        pathId: string,
        body: PathInvitationSendBody,
        idempotencyKey: string,
      ) => Promise<unknown>,
    ): Promise<PathInvitationSendResult> {
      if (!confirmed) {
        cancel(review.pathId);
        return { kind: 'cancelled' };
      }
      const recipient = validatedRecipient(review.recipient, review.requestedUsername);
      if (!validIdentifier(review.pathId) || !recipient || !validRole(role)) {
        return { kind: 'failed', failure: { kind: 'invalid_response' } };
      }
      const signature = `${recipient.userId}\0${recipient.username}\0${role}`;
      const epoch = (epochs[review.pathId] ?? 0) + 1;
      epochs[review.pathId] = epoch;
      const previous = retries[review.pathId];
      const retry = previous?.signature === signature
        ? previous
        : {
            signature,
            idempotencyKey: keyFactory(),
            body: Object.freeze({
              username: recipient.username,
              expectedRecipientUserId: recipient.userId,
              offeredRole: role,
            }),
          };
      if (!validIdempotencyKey(retry.idempotencyKey)) {
        throw new Error('invalid Path invitation idempotency key');
      }
      retries[review.pathId] = retry;
      try {
        const response = await request(review.pathId, retry.body, retry.idempotencyKey);
        if (epochs[review.pathId] !== epoch) return { kind: 'superseded' };
        const invitation = validatedInvitation(response, false);
        if (
          !invitation ||
          invitation.pathId !== review.pathId ||
          invitation.recipientUserId !== recipient.userId ||
          invitation.offeredRole !== role
        ) {
          return { kind: 'failed', failure: { kind: 'invalid_response' } };
        }
        delete retries[review.pathId];
        return { kind: 'sent', invitation };
      } catch (cause) {
        return epochs[review.pathId] === epoch
          ? { kind: 'failed', failure: retainedFailure(cause) }
          : { kind: 'superseded' };
      }
    },
    cancel,
  };
}

export function mergePendingInvitationPage(
  current: PendingPathInvitationState,
  page: PendingPathInvitationPage,
  requestedCursor: string,
): PendingPathInvitationState {
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
  base.forEach((invitation, index) => {
    const valid = validatedPendingInvitation(invitation);
    if (!valid) throw new Error('invalid pending invitation');
    indexes.set(valid.invitation.id, index);
    base[index] = valid;
  });
  for (const candidate of page.items) {
    const invitation = validatedPendingInvitation(candidate);
    if (!invitation) throw new Error('invalid pending invitation');
    const existing = indexes.get(invitation.invitation.id);
    if (existing === undefined) {
      indexes.set(invitation.invitation.id, base.length);
      base.push(invitation);
    } else {
      base[existing] = invitation;
    }
  }
  return { items: base, nextCursor: page.nextCursor };
}

type AcceptRetry = {
  signature: string;
  idempotencyKey: string;
  body: PathInvitationAcceptBody | undefined;
};

export function createPathInvitationAcceptOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, AcceptRetry | undefined> = {};

  function cancel(invitationId?: string): void {
    if (invitationId) {
      epochs[invitationId] = (epochs[invitationId] ?? 0) + 1;
      delete retries[invitationId];
      return;
    }
    for (const id of Object.keys(epochs)) epochs[id] = (epochs[id] ?? 0) + 1;
    for (const id of Object.keys(retries)) delete retries[id];
  }

  async function submitOwned(
    invitationId: string,
    signature: string,
    body: PathInvitationAcceptBody | undefined,
    request: (
      invitationId: string,
      idempotencyKey: string,
      body?: PathInvitationAcceptBody,
    ) => Promise<unknown>,
  ): Promise<PathInvitationAcceptResult> {
    if (!validIdentifier(invitationId)) {
      return { kind: 'failed', failure: { kind: 'invalid_response' } };
    }
    const epoch = (epochs[invitationId] ?? 0) + 1;
    epochs[invitationId] = epoch;
    const previous = retries[invitationId];
    const retry = previous?.signature === signature
      ? previous
      : {
          signature,
          idempotencyKey: keyFactory(),
          body,
        };
    if (!validIdempotencyKey(retry.idempotencyKey)) {
      throw new Error('invalid Path invitation acceptance idempotency key');
    }
    retries[invitationId] = retry;
    try {
      const response = await request(invitationId, retry.idempotencyKey, retry.body);
      if (epochs[invitationId] !== epoch) return { kind: 'superseded' };
      const invitation = validatedInvitation(response, true);
      if (!invitation || invitation.id !== invitationId) {
        return { kind: 'failed', failure: { kind: 'invalid_response' } };
      }
      delete retries[invitationId];
      return { kind: 'accepted', invitation };
    } catch (cause) {
      return epochs[invitationId] === epoch
        ? { kind: 'failed', failure: retainedFailure(cause) }
        : { kind: 'superseded' };
    }
  }

  return {
    async accept(
      invitationId: string,
      request: (invitationId: string, idempotencyKey: string) => Promise<unknown>,
    ): Promise<PathInvitationAcceptResult> {
      return submitOwned(invitationId, 'legacy', undefined, request);
    },
    async submit(
      review: PathInvitationAcceptanceReview,
      confirmed: boolean,
      request: (
        invitationId: string,
        idempotencyKey: string,
        body?: PathInvitationAcceptBody,
      ) => Promise<unknown>,
    ): Promise<PathInvitationAcceptSubmissionResult> {
      if (
        !review ||
        typeof review !== 'object' ||
        !validIdentifier(review.invitationId)
      ) {
        return { kind: 'failed', failure: { kind: 'invalid_response' } };
      }
      if (review.kind === 'ready') {
        if (Object.keys(review).length !== 2) {
          return { kind: 'failed', failure: { kind: 'invalid_response' } };
        }
        return submitOwned(review.invitationId, 'ready', undefined, request);
      }
      if (review.kind !== 'confirmation-required' || Object.keys(review).length !== 3) {
        return { kind: 'failed', failure: { kind: 'invalid_response' } };
      }
      const warning = validatedVisibilityWarning(review.warning);
      if (!warning) return { kind: 'failed', failure: { kind: 'invalid_response' } };
      if (!confirmed) {
        cancel(review.invitationId);
        return { kind: 'cancelled' };
      }
      const acknowledgement = Object.freeze({ pathVisibility: warning.pathVisibility });
      const body = Object.freeze({ visibilityWarningAcknowledgement: acknowledgement });
      const signature = `warning\0${warning.pathVisibility}\0${warning.hasRetainedActivity}`;
      return submitOwned(review.invitationId, signature, body, request);
    },
    cancel,
  };
}

function validatedRejection(value: unknown): PathInvitationRejection | undefined {
  if (!value || typeof value !== 'object') return undefined;
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort();
  if (
    keys.length !== 3 ||
    keys[0] !== 'invitationId' ||
    keys[1] !== 'rejectedAt' ||
    keys[2] !== 'unreadCount' ||
    !validIdentifier(record.invitationId) ||
    !validInstant(record.rejectedAt) ||
    !Number.isSafeInteger(record.unreadCount) ||
    (record.unreadCount as number) < 0
  ) {
    return undefined;
  }
  return Object.freeze({
    invitationId: record.invitationId,
    rejectedAt: record.rejectedAt,
    unreadCount: record.unreadCount as number,
  });
}

export function createPathInvitationRejectOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, string | undefined> = {};

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
      invitationId: string,
      confirmed: boolean,
      request: (invitationId: string, idempotencyKey: string) => Promise<unknown>,
    ): Promise<PathInvitationRejectResult> {
      if (!validIdentifier(invitationId)) {
        return { kind: 'failed', failure: { kind: 'invalid_response' } };
      }
      if (!confirmed) {
        cancel(invitationId);
        return { kind: 'cancelled' };
      }
      const epoch = (epochs[invitationId] ?? 0) + 1;
      epochs[invitationId] = epoch;
      const idempotencyKey = retries[invitationId] ?? keyFactory();
      if (!validIdempotencyKey(idempotencyKey)) {
        throw new Error('invalid Path invitation rejection idempotency key');
      }
      retries[invitationId] = idempotencyKey;
      try {
        const response = await request(invitationId, idempotencyKey);
        if (epochs[invitationId] !== epoch) return { kind: 'superseded' };
        const rejection = validatedRejection(response);
        if (!rejection || rejection.invitationId !== invitationId) {
          return { kind: 'failed', failure: { kind: 'invalid_response' } };
        }
        delete retries[invitationId];
        return { kind: 'rejected', rejection };
      } catch (cause) {
        return epochs[invitationId] === epoch
          ? { kind: 'failed', failure: retainedFailure(cause) }
          : { kind: 'superseded' };
      }
    },
    cancel,
  };
}
