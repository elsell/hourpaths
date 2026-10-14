import assert from 'node:assert/strict';
import test from 'node:test';
import { LiveActivityRefreshCoordinator, type LiveActivityRefreshRegistration } from './live-activity-refresh';

const registration: LiveActivityRefreshRegistration = {
  owner: 'alice', activityId: 'activity', accountLease: 'lease', token: 'ab',
  expiresAt: 10000, boundaries: [2000, 4000],
};
test('revocation fences queued rotation and runs after an in-flight registration', async () => {
  const events: string[] = [];
  let enter!: () => void;
  let release!: () => void;
  const entered = new Promise<void>(resolve => { enter = resolve; });
  const blocked = new Promise<void>(resolve => { release = resolve; });
  const coordinator = new LiveActivityRefreshCoordinator({
    register: async value => { events.push(value.token); enter(); await blocked; },
    revoke: async value => { events.push(`revoke:${value.accountLease}`); },
  }, () => 1000);
  const first = coordinator.update(registration);
  await entered;
  const rotated = coordinator.update({ ...registration, token: 'cd' });
  const revoked = coordinator.clear();
  release();
  await Promise.all([first, rotated, revoked]);
  assert.deepEqual(events, ['ab', 'revoke:lease']);
});
test('reconciliation retries network failure, deduplicates success, and preserves expiry', async () => {
  const received: LiveActivityRefreshRegistration[] = [];
  const coordinator = new LiveActivityRefreshCoordinator({
    register: async value => { received.push(value); if (received.length === 1) throw new Error('offline'); },
    revoke: async () => undefined,
  }, () => 1000);
  assert.equal(await coordinator.update(registration), false);
  assert.equal(await coordinator.update(registration), true);
  assert.equal(await coordinator.update(registration), true);
  assert.equal(received.length, 2);
  await coordinator.update({ ...registration, expiresAt: 20000, token: 'cd', boundaries: [4000, 15000] });
  assert.equal(received.at(-1)!.expiresAt, 10000);
  assert.deepEqual(received.at(-1)!.boundaries, [4000]);
});
test('account replacement revokes old binding before registering new owner', async () => {
  const events: string[] = [];
  const coordinator = new LiveActivityRefreshCoordinator({
    register: async value => { events.push(`put:${value.owner}`); },
    revoke: async value => { events.push(`delete:${value.owner}`); },
  }, () => 1000);
  await coordinator.update(registration);
  await coordinator.update({ ...registration, owner: 'bob', accountLease: 'new-lease' });
  assert.deepEqual(events, ['put:alice', 'delete:alice', 'put:bob']);
});
test('failed cleanup is retained for reconciliation before a replacement can register', async () => {
  const events: string[] = [];
  let offline = true;
  const coordinator = new LiveActivityRefreshCoordinator({
    register: async value => { events.push(value.owner); },
    revoke: async () => { if (offline) throw new Error('offline'); events.push('revoked'); },
  }, () => 1000);
  await coordinator.update(registration);
  assert.equal(await coordinator.clear(), false);
  const next = { ...registration, owner: 'bob', accountLease: 'bob-lease' };
  assert.equal(await coordinator.update(next), false);
  assert.deepEqual(events, ['alice']);
  offline = false;
  assert.equal(await coordinator.update(next), true);
  assert.deepEqual(events, ['alice', 'revoked', 'bob']);
});
