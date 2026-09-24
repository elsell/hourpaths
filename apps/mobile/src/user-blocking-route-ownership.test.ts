import assert from 'node:assert/strict';
import test from 'node:test';
import type { BlockedAccountPage, BlockResult, UnblockResult, UserBlockingPort } from '@hourpaths/client-core';
import { createOwnedUserBlockingPort } from './ui/user-blocking-route-presentation';

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((accept) => { resolve = accept; });
  return { promise, resolve };
}

test('late blocking results fail closed after owner replacement and never project into the replacement account', async () => {
  const page = deferred<BlockedAccountPage>();
  const block = deferred<BlockResult>();
  let current = true;
  const projected: string[] = [];
  const target = { displayName: 'target-a', userId: 'target-a', username: 'target.a' };
  const acknowledgement = { expiresAt: '2026-08-15T18:00:00Z', token: 'review-token', version: 1 as const };
  const port: UserBlockingPort = {
    blockUser: () => block.promise,
    listBlockedAccounts: () => page.promise,
    reviewBlock: async () => ({ acknowledgement, sharedPaths: [], target }),
    unblockUser: async () => ({ blocked: false, target } as UnblockResult),
  };
  const owned = createOwnedUserBlockingPort(() => port, () => current, (identity) => projected.push(identity.userId));
  const listResult = owned.listBlockedAccounts();
  const blockResult = owned.blockUser('target.a', 'key', acknowledgement);
  current = false;
  page.resolve({ items: [], nextCursor: '' });
  block.resolve({ blocked: true, target });
  await assert.rejects(listResult, /blocking_superseded/);
  await assert.rejects(blockResult, /blocking_superseded/);
  assert.deepEqual(projected, []);
});

test('same-owner token rotations resolve the latest transport without remounting the owned presentation', async () => {
  const calls: string[] = [];
  const port = (token: string): UserBlockingPort => ({
    blockUser: async () => { throw new Error('unused'); },
    listBlockedAccounts: async () => { calls.push(token); return { items: [], nextCursor: token }; },
    reviewBlock: async () => ({
      acknowledgement: { expiresAt: '2026-08-15T18:00:00Z', token: 'review-token', version: 1 as const },
      sharedPaths: [],
      target: { displayName: 'target-a', userId: 'target-a', username: 'target.a' },
    }),
    unblockUser: async () => { throw new Error('unused'); },
  });
  let currentPort = port('token-a');
  const owned = createOwnedUserBlockingPort(() => currentPort, () => true, () => undefined);
  assert.equal((await owned.listBlockedAccounts()).nextCursor, 'token-a');
  currentPort = port('token-b');
  assert.equal((await owned.listBlockedAccounts()).nextCursor, 'token-b');
  currentPort = port('token-c');
  assert.equal((await owned.listBlockedAccounts()).nextCursor, 'token-c');
  assert.deepEqual(calls, ['token-a', 'token-b', 'token-c']);
});

test('replacement between transport completion and social projection fails closed', async () => {
  const target = { displayName: 'target-a', userId: 'target-a', username: 'target.a' };
  const acknowledgement = { expiresAt: '2026-08-15T18:00:00Z', token: 'review-token', version: 1 as const };
  let ownershipCheck = 0;
  let projected = false;
  const port: UserBlockingPort = {
    blockUser: async () => ({ blocked: true, target }),
    listBlockedAccounts: async () => ({ items: [], nextCursor: '' }),
    reviewBlock: async () => ({ acknowledgement, sharedPaths: [], target }),
    unblockUser: async () => ({ blocked: false, target }),
  };
  const owned = createOwnedUserBlockingPort(
    () => port,
    () => ++ownershipCheck < 3,
    () => { projected = true; },
  );
  await assert.rejects(owned.blockUser('target.a', 'key', acknowledgement), /blocking_superseded/);
  assert.equal(projected, false);
});
