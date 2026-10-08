import test from 'node:test';
import assert from 'node:assert/strict';
import { createNotificationChannelOperationOwner, type NotificationChannelPreference } from './notification-channels';
import { notificationChannelPreferencesFromAPI } from './adapters/api-notification-channels';

test('a retried write reuses its intent but another channel receives a different key', async () => {
  let next = 0;
  const owner = createNotificationChannelOperationOwner(() => `notification-key-${++next}`);
  const keys: string[] = [];
  const change: NotificationChannelPreference = { channel: 'comments', enabled: false, revision: 2 };
  assert.equal((await owner.submit(change, async (_, key) => { keys.push(key); throw new Error('network'); })).kind, 'failed');
  const request = async (value: NotificationChannelPreference, key: string) => { keys.push(key); return { ...value, revision: value.revision + 1 }; };
  assert.equal((await owner.submit(change, request)).kind, 'applied');
  assert.equal((await owner.submit({ ...change, channel: 'reactions' }, request)).kind, 'applied');
  assert.equal(keys[0], keys[1]);
  assert.notEqual(keys[1], keys[2]);
});

test('a held result from a replaced account cannot become the new account’s settings', async () => {
  const owner = createNotificationChannelOperationOwner(() => 'notification-operation-key');
  let finish!: (value: NotificationChannelPreference) => void;
  const change: NotificationChannelPreference = { channel: 'comments', enabled: false, revision: 2 };
  const old = owner.submit(change, () => new Promise(resolve => { finish = resolve; }));
  owner.cancel();
  const fresh = await owner.submit({ ...change, channel: 'nudges' }, async value => ({ ...value, revision: 3 }));
  finish({ ...change, revision: 3 });
  assert.equal((await old).kind, 'superseded');
  assert.equal(fresh.kind, 'applied');
});

test('an incomplete or duplicate catalog fails closed instead of inventing enabled settings', () => {
  assert.throws(() => notificationChannelPreferencesFromAPI(null));
  assert.throws(() => notificationChannelPreferencesFromAPI([{ channel: 'comments', enabled: true, revision: 0 }]));
  assert.throws(() => notificationChannelPreferencesFromAPI(Array.from({ length: 10 }, () => ({ channel: 'comments', enabled: true, revision: 0 }))));
});
