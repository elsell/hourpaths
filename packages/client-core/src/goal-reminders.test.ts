import assert from 'node:assert/strict';
import test from 'node:test';
import { createGoalReminderOperationOwner } from './goal-reminders';

test('ambiguous goal reminder preference retry keeps intent while a changed subject cannot reuse it', async () => {
  let sequence = 0;
  const owner = createGoalReminderOperationOwner(() => `key-${++sequence}`);
  const subject = { id: 'path-1' } as const;
  const value = { enabled: true, revision: 0 };
  const keys: string[] = [];
  const fail = async (_subject: unknown, _value: unknown, key: string) => { keys.push(key); throw Error('network'); };
  await owner.submit(subject, value, fail);
  await owner.submit(subject, value, fail);
  await owner.submit({ id: 'path-2' }, value, fail);
  assert.deepEqual(keys, ['key-1', 'key-1', 'key-2']);
});

test('sign-out fences late preference results and admits a new account operation', async () => {
  const owner = createGoalReminderOperationOwner(() => 'key');
  let release!: (value: { enabled: boolean; revision: number }) => void;
  const pending = owner.submit({ id: 'path-1' }, { enabled: false, revision: 2 }, () => new Promise(resolve => { release = resolve; }));
  owner.cancel();
  const next = await owner.submit({ id: 'path-1' }, { enabled: true, revision: 0 }, async () => ({ enabled: true, revision: 1 }));
  release({ enabled: false, revision: 3 });
  assert.equal((await pending).kind, 'superseded');
  assert.equal(next.kind, 'applied');
});
