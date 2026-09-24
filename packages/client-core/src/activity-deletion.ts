export type ActivityDeletionIdentity = Readonly<{
  activityId: string;
  ownerId: string;
  pathId: string;
  sessionToken: string;
}>;

export type ActivityDeletionResult = Readonly<{
  accumulatedSeconds: number;
  intervalProgress?: Readonly<{
    accumulatedSeconds: number;
    targetSeconds: number;
  }>;
  removedFeedEventIds: readonly string[];
  sessionCount: number;
  unreadNotificationCount: number;
}>;

export type ActivityDeletionMutationResult =
  | { kind: 'applied'; result: ActivityDeletionResult }
  | { kind: 'busy' }
  | { kind: 'cancelled' }
  | { kind: 'failed'; cause: unknown }
  | { kind: 'superseded' };

function validText(value: string): boolean {
  return value.length > 0 && value.trim() === value && !/[\u0000-\u001f\u007f]/u.test(value);
}

function signature(identity: ActivityDeletionIdentity): string {
  const parts = [identity.sessionToken, identity.ownerId, identity.pathId, identity.activityId];
  if (!parts.every(validText)) throw new Error('invalid activity deletion identity');
  return JSON.stringify(parts);
}

function deletionResult(value: unknown, activityId: string): ActivityDeletionResult {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error('invalid activity deletion result');
  }
  const record = value as Record<string, unknown>;
  const keys = Object.keys(record).sort().join(',');
  if (
    keys !== 'accumulatedSeconds,removedFeedEventIds,sessionCount,unreadNotificationCount' &&
    keys !== 'accumulatedSeconds,intervalProgress,removedFeedEventIds,sessionCount,unreadNotificationCount'
  ) {
    throw new Error('invalid activity deletion result');
  }
  if (!Number.isSafeInteger(record.accumulatedSeconds) || (record.accumulatedSeconds as number) < 0) {
    throw new Error('invalid activity deletion result');
  }
  if (!Number.isSafeInteger(record.sessionCount) || (record.sessionCount as number) < 0) {
    throw new Error('invalid activity deletion result');
  }
  if (!Number.isSafeInteger(record.unreadNotificationCount) || (record.unreadNotificationCount as number) < 0) {
    throw new Error('invalid activity deletion result');
  }
  if (
    !Array.isArray(record.removedFeedEventIds) ||
    record.removedFeedEventIds.length === 0 ||
    !record.removedFeedEventIds.every((eventId): eventId is string =>
      typeof eventId === 'string' && validText(eventId)) ||
    new Set(record.removedFeedEventIds).size !== record.removedFeedEventIds.length
  ) throw new Error('invalid activity deletion result');
  const expectedPracticeEventId = `practice:${activityId}`;
  if (
    !record.removedFeedEventIds.includes(expectedPracticeEventId) ||
    record.removedFeedEventIds.some((eventId) =>
      eventId !== expectedPracticeEventId && !eventId.startsWith('achievement:'))
  ) throw new Error('invalid activity deletion result');
  let intervalProgress: ActivityDeletionResult['intervalProgress'];
  if (record.intervalProgress !== undefined) {
    if (!record.intervalProgress || typeof record.intervalProgress !== 'object' || Array.isArray(record.intervalProgress)) {
      throw new Error('invalid activity deletion result');
    }
    const progress = record.intervalProgress as Record<string, unknown>;
    if (
      Object.keys(progress).sort().join(',') !== 'accumulatedSeconds,targetSeconds' ||
      !Number.isSafeInteger(progress.accumulatedSeconds) ||
      (progress.accumulatedSeconds as number) < 0 ||
      !Number.isSafeInteger(progress.targetSeconds) ||
      (progress.targetSeconds as number) <= 0
    ) throw new Error('invalid activity deletion result');
    intervalProgress = Object.freeze({
      accumulatedSeconds: progress.accumulatedSeconds as number,
      targetSeconds: progress.targetSeconds as number,
    });
  }
  return Object.freeze({
    accumulatedSeconds: record.accumulatedSeconds as number,
    ...(intervalProgress ? { intervalProgress } : {}),
    removedFeedEventIds: Object.freeze([...record.removedFeedEventIds]),
    sessionCount: record.sessionCount as number,
    unreadNotificationCount: record.unreadNotificationCount as number,
  });
}

export function createActivityDeletionOperationOwner(keyFactory: () => string) {
  let epoch = 0;
  let pending = false;
  let retry: { idempotencyKey: string; signature: string } | undefined;

  function intent(identity: ActivityDeletionIdentity) {
    const nextSignature = signature(identity);
    if (retry?.signature !== nextSignature) {
      const idempotencyKey = keyFactory();
      if (idempotencyKey.length < 16 || idempotencyKey.length > 128 || !/^[\x20-\x7e]+$/.test(idempotencyKey)) {
        throw new Error('invalid activity deletion idempotency key');
      }
      epoch += 1;
      pending = false;
      retry = { idempotencyKey, signature: nextSignature };
    }
    return Object.freeze({ idempotencyKey: retry.idempotencyKey });
  }

  function invalidate() {
    epoch += 1;
    pending = false;
    retry = undefined;
  }

  return {
    intent,
    invalidate,
    async submit(
      identity: ActivityDeletionIdentity,
      confirmed: boolean,
      request: (idempotencyKey: string) => Promise<unknown>,
    ): Promise<ActivityDeletionMutationResult> {
      if (!confirmed) return { kind: 'cancelled' };
      const operation = intent(identity);
      if (pending) return { kind: 'busy' };
      pending = true;
      const ownedEpoch = epoch;
      const ownedSignature = signature(identity);
      try {
        const value = await request(operation.idempotencyKey);
        if (epoch !== ownedEpoch || retry?.signature !== ownedSignature) return { kind: 'superseded' };
        const result = deletionResult(value, identity.activityId);
        retry = undefined;
        return { kind: 'applied', result };
      } catch (cause) {
        return epoch === ownedEpoch && retry?.signature === ownedSignature
          ? { kind: 'failed', cause }
          : { kind: 'superseded' };
      } finally {
        if (epoch === ownedEpoch) pending = false;
      }
    },
  };
}

export function applyActivityDeletionResult<
  A extends { activity: { id: string } },
  T extends { running: boolean; accumulatedSeconds: number; intervalProgress?: unknown },
  S extends { activities: readonly A[]; timer: T },
>(state: S, mutation: ActivityDeletionMutationResult, activityId: string): S {
  if (mutation.kind !== 'applied') return state;
  return {
    ...state,
    activities: state.activities.filter(({ activity }) => activity.id !== activityId),
    timer: {
      ...state.timer,
      accumulatedSeconds: mutation.result.accumulatedSeconds,
      intervalProgress: mutation.result.intervalProgress,
    },
  };
}
