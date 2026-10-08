import assert from 'node:assert/strict';
import test from 'node:test';
import { createTimerSubscriptionOperationOwner } from './timer-subscriptions';

test('ambiguous timer preference retry keeps intent while a changed subject cannot reuse it', async () => {
  let sequence = 0;
  const owner = createTimerSubscriptionOperationOwner(() => `key-${++sequence}`);
  const subject = { scope: 'person', id: 'person-1' } as const;
  const value = { enabled: true, revision: 0 };
  const keys: string[] = [];
  const fail = async (_subject: unknown, _value: unknown, key: string) => { keys.push(key); throw Error('network'); };
  await owner.submit(subject, value, fail);
  await owner.submit(subject, value, fail);
  await owner.submit({ scope: 'path', id: subject.id }, value, fail);
  assert.deepEqual(keys, ['key-1', 'key-1', 'key-2']);
});

test('sign-out fences late preference results and admits a new account operation', async () => {
  const owner = createTimerSubscriptionOperationOwner(() => 'key');
  let release!: (value: { enabled: boolean; revision: number }) => void;
  const pending = owner.submit({ scope: 'path', id: 'path-1' }, { enabled: false, revision: 2 }, () => new Promise(resolve => { release = resolve; }));
  owner.cancel();
  const next = await owner.submit({ scope: 'path', id: 'path-1' }, { enabled: true, revision: 0 }, async () => ({ enabled: true, revision: 1 }));
  release({ enabled: false, revision: 3 });
  assert.equal((await pending).kind, 'superseded');
  assert.equal(next.kind, 'applied');
});
