import assert from 'node:assert/strict';
import test from 'node:test';
import {
  createUserBlockingGeneratedPort,
  type GeneratedUserBlockingClient,
  type GeneratedUserBlockingResult,
} from './user-blocking-port';

const identity = { userId: 'user-alex', username: 'alex.r', displayName: 'alex.r' };
const success = (data: unknown): GeneratedUserBlockingResult => ({
  data,
  response: new Response(null, { status: 200 }),
});

test('blocking adapter delegates exact targets and replay keys to the generated client', async () => {
  const calls: Array<readonly unknown[]> = [];
  const client: GeneratedUserBlockingClient = {
    async reviewProfileBlock(username) {
      calls.push(['review', username]);
      return success({ data: { target: identity, sharedPaths: [], acknowledgement: { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' } } });
    },
    async blockProfile(username, idempotencyKey, acknowledgement) {
      calls.push(['block', username, idempotencyKey, acknowledgement]);
      return success({ data: { target: identity, blocked: true } });
    },
    async blockedAccounts(cursor) {
      calls.push(['list', cursor]);
      return success({ data: [{ ...identity, blockedAt: '2026-07-29T10:00:00Z' }], meta: { nextCursor: 'next' } });
    },
    async unblockAccount(userId, idempotencyKey) {
      calls.push(['unblock', userId, idempotencyKey]);
      return success({ data: { target: identity, blocked: false } });
    },
  };
  const port = createUserBlockingGeneratedPort(() => client);

  const review = await port.reviewBlock('alex.r');
  await port.blockUser('alex.r', 'block-key', review.acknowledgement);
  await port.listBlockedAccounts('cursor/value');
  await port.unblockUser(identity.userId, 'unblock-key');

  assert.deepEqual(calls, [
    ['review', 'alex.r'],
    ['block', 'alex.r', 'block-key', { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' }],
    ['list', 'cursor/value'],
    ['unblock', 'user-alex', 'unblock-key'],
  ]);
});

test('blocking adapter preserves generated HTTP failures and rejects invalid success payloads', async () => {
  const failure = createUserBlockingGeneratedPort(() => ({
    reviewProfileBlock: async () => ({
      error: { code: 'unauthenticated' },
      response: new Response(null, { status: 401 }),
    }),
    blockProfile: async () => { throw new Error('unused'); },
    blockedAccounts: async () => { throw new Error('unused'); },
    unblockAccount: async () => { throw new Error('unused'); },
  }));
  await assert.rejects(() => failure.reviewBlock('alex.r'), {
    code: 'unauthenticated', kind: 'http', status: 401,
  });

  const malformed = createUserBlockingGeneratedPort(() => ({
    reviewProfileBlock: async () => success({
      data: { target: { ...identity, email: 'private@example.test' }, sharedPaths: [], acknowledgement: { version: 1, token: 'signed-review-token', expiresAt: '2026-07-29T10:05:00Z' } },
    }),
    blockProfile: async () => { throw new Error('unused'); },
    blockedAccounts: async () => { throw new Error('unused'); },
    unblockAccount: async () => { throw new Error('unused'); },
  }));
  await assert.rejects(() => malformed.reviewBlock('alex.r'), /invalid block review/);
});
