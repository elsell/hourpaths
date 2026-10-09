import assert from 'node:assert/strict';
import test from 'node:test';
import { createWeekStartOperationOwner, type WeekStartPreference } from './week-start-preference';

test('week-start retries keep their identity and discarded account work cannot complete', async () => {
  let keys = 0; const owner = createWeekStartOperationOwner('alice', () => `key-${++keys}`);
  const change = { userId: 'alice', reviewedFirstDayOfWeek: 1, proposedFirstDayOfWeek: 7 };
  const received: string[] = [];
  const lost = await owner.submit(change, async (_, key) => { received.push(key); throw new Error('lost response'); });
  assert.equal(lost.kind, 'failed');
  let complete!: (value: WeekStartPreference) => void;
  const pending = owner.submit(change, (_, key) => { received.push(key); return new Promise(resolve => { complete = resolve; }); });
  assert.equal((await owner.submit(change, async () => { throw new Error('duplicate request'); })).kind, 'busy');
  owner.cancel(); complete({ userId: 'alice', firstDayOfWeek: 7 });
  assert.equal((await pending).kind, 'superseded'); assert.deepEqual(received, ['key-1', 'key-1']);
  let called = false;
  assert.equal((await owner.submit({ ...change, userId: 'bob' }, async () => { called = true; return { userId: 'bob', firstDayOfWeek: 7 }; })).kind, 'failed');
  assert.equal(called, false);
});
