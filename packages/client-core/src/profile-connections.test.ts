import assert from 'node:assert/strict';
import test from 'node:test';
import { createFollowerRemovalOwner } from './profile-connections';
test('removal retries keep their key, replacement accounts cannot dispatch or apply old work', async () => {
  let count = 0;
  const owner = createFollowerRemovalOwner('owner', () => `operation-${++count}`);
  const keys: string[] = [];
  assert.equal((await owner.remove('owner', 'person', async (_, key) => { keys.push(key); throw new Error('lost response'); })).kind, 'failed');
  assert.equal((await owner.remove('owner', 'person', async (_, key) => { keys.push(key); })).kind, 'applied');
  assert.deepEqual(keys, ['operation-1', 'operation-1']);
  let calls = 0;
  assert.equal((await owner.remove('replacement', 'person', async () => { calls++; })).kind, 'failed');
  assert.equal(calls, 0);
  let finish!: () => void;
  const pending = owner.remove('owner', 'person', () => new Promise<void>(resolve => { finish = resolve; }));
  assert.equal((await owner.remove('owner', 'person', async () => { calls++; })).kind, 'busy');
  owner.cancel(); finish();
  assert.equal((await pending).kind, 'superseded');
});
