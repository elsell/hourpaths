import assert from 'node:assert/strict';
import test from 'node:test';

import { createNotificationRefreshLatch } from './notification-refresh.js';

test('notification refreshes serialize, coalesce, and stop after disposal', async () => {
  const releases: Array<() => void> = [];
  const owners: string[] = [];
  let active = 0;
  let maximumActive = 0;
  const latch = createNotificationRefreshLatch(async (ownerId) => {
    owners.push(ownerId);
    active += 1;
    maximumActive = Math.max(maximumActive, active);
    await new Promise<void>((resolve) => releases.push(resolve));
    active -= 1;
  });

  const first = latch.request('user-a');
  const coalesced = latch.request('user-a');
  assert.equal(first, coalesced);
  assert.deepEqual(owners, ['user-a']);
  releases.shift()?.();
  await new Promise((resolve) => setTimeout(resolve, 0));
  assert.deepEqual(owners, ['user-a', 'user-a']);
  releases.shift()?.();
  await first;
  assert.equal(maximumActive, 1);

  latch.dispose();
  await latch.request('user-a');
  assert.deepEqual(owners, ['user-a', 'user-a']);
});

test('a mutation queued behind a stale refresh receives a trailing authoritative refresh', async () => {
  let serverRead = false;
  let visibleRead = false;
  let releaseFirst: (() => void) | undefined;
  let refreshCount = 0;
  let active = 0;
  let maximumActive = 0;
  const latch = createNotificationRefreshLatch(async () => {
    active += 1;
    maximumActive = Math.max(maximumActive, active);
    refreshCount += 1;
    const admittedRead = serverRead;
    if (refreshCount === 1) {
      await new Promise<void>((resolve) => { releaseFirst = resolve; });
    }
    visibleRead = admittedRead;
    active -= 1;
  });

  const initialRefresh = latch.request('user-a');
  await new Promise((resolve) => setTimeout(resolve, 0));
  serverRead = true;
  visibleRead = true;
  const mutationRefresh = latch.request('user-a');
  releaseFirst?.();
  await mutationRefresh;

  assert.equal(initialRefresh, mutationRefresh);
  assert.equal(refreshCount, 2);
  assert.equal(maximumActive, 1);
  assert.equal(visibleRead, true);
});
