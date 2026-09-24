import type { PracticeComment, PracticeCommentPage } from './practice-comments';
import { publicProfileFromAPI } from './profile-discovery';

export type PracticeCommentHeartState = Readonly<{
  commentId: string;
  heartCount: number;
  heartedByViewer: boolean;
}>;

export type PracticeCommentHearter = Readonly<{
  userId: string;
  username: string;
  displayName: string;
  profilePictureURL?: string;
}>;

export type PracticeCommentHeartRosterPage = Readonly<{
  commentId: string;
  items: readonly PracticeCommentHearter[];
  nextCursor: string;
  revision: number;
}>;

export type PracticeCommentHeartMutationResult =
  | Readonly<{ kind: 'applied'; state: PracticeCommentHeartState }>
  | Readonly<{ kind: 'failed'; cause: unknown }>
  | Readonly<{ kind: 'superseded' }>;

function record(value: unknown, message = 'invalid practice comment heart'): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error(message);
  return value as Record<string, unknown>;
}

function exact(value: Record<string, unknown>, keys: readonly string[]): boolean {
  const actual = Object.keys(value).sort();
  const expected = [...keys].sort();
  return actual.length === expected.length && actual.every((key, index) => key === expected[index]);
}

function exactEnvelope(value: Record<string, unknown>, keys: readonly string[]): boolean {
  const allowed = new Set([...keys, '$schema']);
  return keys.every((key) => Object.hasOwn(value, key)) &&
    Object.keys(value).every((key) => allowed.has(key)) &&
    (value.$schema === undefined || (text(value.$schema) && value.$schema.length <= 2048));
}

function text(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0 && value.trim() === value &&
    !/[\u0000-\u0009\u000b-\u001f\u007f]/u.test(value);
}

function validIdempotencyKey(value: string): boolean {
  return value.length >= 16 && value.length <= 128 && /^[\x20-\x7e]+$/.test(value);
}

export function practiceCommentHeartStateFromAPI(value: unknown): PracticeCommentHeartState {
  const candidate = record(value);
  if (!exact(candidate, ['commentId', 'heartCount', 'heartedByViewer']) ||
    !text(candidate.commentId) || !Number.isSafeInteger(candidate.heartCount) ||
    (candidate.heartCount as number) < 0 || typeof candidate.heartedByViewer !== 'boolean' ||
    (candidate.heartedByViewer && candidate.heartCount === 0)) {
    throw new Error('invalid practice comment heart');
  }
  return Object.freeze({
    commentId: candidate.commentId,
    heartCount: candidate.heartCount as number,
    heartedByViewer: candidate.heartedByViewer,
  });
}

export function applyPracticeCommentHeartState(
  page: PracticeCommentPage,
  state: PracticeCommentHeartState,
): PracticeCommentPage {
  const authoritative = practiceCommentHeartStateFromAPI(state);
  if (!page.items.some(({ id }) => id === authoritative.commentId)) throw new Error('unknown practice comment');
  return Object.freeze({
    ...page,
    items: Object.freeze(page.items.map((comment) => comment.id === authoritative.commentId
      ? Object.freeze({
          ...comment,
          heartCount: authoritative.heartCount,
          heartedByViewer: authoritative.heartedByViewer,
          heartPending: false,
        })
      : comment)),
  });
}

export function updatePracticeCommentHeartOptimistically(
  page: PracticeCommentPage,
  commentId: string,
  hearted: boolean,
): PracticeCommentPage {
  const target = page.items.find(({ id }) => id === commentId);
  if (!target) throw new Error('unknown practice comment');
  if (target.heartedByViewer === hearted) return page;
  if (!hearted && target.heartCount < 1) throw new Error('invalid practice comment heart state');
  return Object.freeze({
    ...page,
    items: Object.freeze(page.items.map((comment) => comment.id === commentId
      ? Object.freeze({
          ...comment,
          heartCount: comment.heartCount + (hearted ? 1 : -1),
          heartedByViewer: hearted,
          heartPending: true,
        })
      : comment)),
  });
}

export function rollbackPracticeCommentHeart(
  page: PracticeCommentPage,
  previous: PracticeComment,
  optimisticHearted: boolean,
): PracticeCommentPage {
  const current = page.items.find(({ id }) => id === previous.id);
  const expectedCount = previous.heartCount + (optimisticHearted ? 1 : -1);
  if (!current || !current.heartPending || current.heartedByViewer !== optimisticHearted ||
    current.heartCount !== expectedCount) return page;
  return Object.freeze({
    ...page,
    items: Object.freeze(page.items.map((comment) => comment.id === previous.id
      ? Object.freeze({ ...previous, heartPending: false })
      : comment)),
  });
}

type Retry = Readonly<{ hearted: boolean; idempotencyKey: string }>;

export function createPracticeCommentHeartOperationOwner(keyFactory: () => string) {
  const epochs: Record<string, number | undefined> = {};
  const retries: Record<string, Retry | undefined> = {};
  const queues: Record<string, Promise<void> | undefined> = {};

  return Object.freeze({
    async submit(
      commentId: string,
      hearted: boolean,
      request: (
        commentId: string,
        hearted: boolean,
        idempotencyKey: string,
      ) => Promise<PracticeCommentHeartState>,
    ): Promise<PracticeCommentHeartMutationResult> {
      if (!text(commentId)) throw new Error('invalid practice comment id');
      const epoch = (epochs[commentId] ?? 0) + 1;
      epochs[commentId] = epoch;
      const previousRetry = retries[commentId];
      const retry = previousRetry?.hearted === hearted
        ? previousRetry
        : Object.freeze({ hearted, idempotencyKey: keyFactory() });
      if (!validIdempotencyKey(retry.idempotencyKey)) {
        throw new Error('invalid comment heart idempotency key');
      }
      retries[commentId] = retry;
      const priorQueue = queues[commentId] ?? Promise.resolve();
      const operation = (async (): Promise<PracticeCommentHeartMutationResult> => {
        await priorQueue;
        if (epochs[commentId] !== epoch) return Object.freeze({ kind: 'superseded' });
        try {
          const state = practiceCommentHeartStateFromAPI(
            await request(commentId, hearted, retry.idempotencyKey),
          );
          if (epochs[commentId] !== epoch) return { kind: 'superseded' };
          if (state.commentId !== commentId || state.heartedByViewer !== hearted) {
            throw new Error('mismatched practice comment heart');
          }
          delete retries[commentId];
          return Object.freeze({ kind: 'applied', state });
        } catch (cause) {
          return epochs[commentId] === epoch
            ? Object.freeze({ kind: 'failed', cause })
            : Object.freeze({ kind: 'superseded' });
        }
      })();
      const drained = operation.then(() => undefined, () => undefined);
      queues[commentId] = drained;
      void drained.finally(() => {
        if (queues[commentId] === drained) delete queues[commentId];
      });
      return operation;
    },
    invalidate(commentId: string): void {
      epochs[commentId] = (epochs[commentId] ?? 0) + 1;
      delete retries[commentId];
    },
  });
}

function hearerFromAPI(value: unknown): PracticeCommentHearter {
  try {
    const profile = publicProfileFromAPI({ data: value });
    return Object.freeze({
      userId: profile.userId,
      username: profile.username,
      displayName: profile.displayName,
      ...(profile.profilePictureUrl === undefined ? {} : { profilePictureURL: profile.profilePictureUrl }),
    });
  } catch {
    throw new Error('invalid heart roster');
  }
}

export function practiceCommentHeartRosterPageFromAPI(
  value: unknown,
  commentId: string,
): PracticeCommentHeartRosterPage {
  if (!text(commentId)) throw new Error('invalid heart roster');
  const envelope = record(value, 'invalid heart roster');
  const data = record(envelope.data, 'invalid heart roster');
  const meta = record(envelope.meta, 'invalid heart roster');
  if (!exactEnvelope(envelope, ['data', 'meta']) || !exact(data, ['items']) || !Array.isArray(data.items) ||
    !Object.keys(meta).every((key) => key === 'nextCursor') ||
    (meta.nextCursor !== undefined &&
      (typeof meta.nextCursor !== 'string' || meta.nextCursor.trim() !== meta.nextCursor))) {
    throw new Error('invalid heart roster');
  }
  const items = data.items.map(hearerFromAPI);
  if (new Set(items.map(({ userId }) => userId)).size !== items.length) throw new Error('invalid heart roster');
  return Object.freeze({
    commentId,
    items: Object.freeze(items),
    nextCursor: meta.nextCursor as string | undefined ?? '',
    revision: 0,
  });
}

export function mergePracticeCommentHeartRosterPage(
  current: PracticeCommentHeartRosterPage,
  incoming: PracticeCommentHeartRosterPage,
  requestedCursor: string,
  requestedRevision: number,
): PracticeCommentHeartRosterPage {
  if (requestedRevision !== current.revision) throw new Error('stale heart roster revision');
  if (requestedCursor !== current.nextCursor || incoming.commentId !== current.commentId) {
    throw new Error('stale heart roster cursor');
  }
  if (!requestedCursor) return Object.freeze({ ...incoming, revision: current.revision });
  const ids = new Set(current.items.map(({ userId }) => userId));
  return Object.freeze({
    commentId: current.commentId,
    items: Object.freeze([...current.items, ...incoming.items.filter(({ userId }) => !ids.has(userId))]),
    nextCursor: incoming.nextCursor,
    revision: current.revision,
  });
}

export function invalidatePracticeCommentHeartRoster(
  page: PracticeCommentHeartRosterPage,
  userId?: string,
): PracticeCommentHeartRosterPage {
  return Object.freeze({
    ...page,
    items: userId === undefined
      ? Object.freeze([])
      : Object.freeze(page.items.filter((item) => item.userId !== userId)),
    revision: page.revision + 1,
  });
}
