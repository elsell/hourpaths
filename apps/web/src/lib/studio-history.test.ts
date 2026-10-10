const utcTimeZone = 'utc';
import assert from 'node:assert/strict';
import test from 'node:test';
import { historyRepository } from './studio/history/application/history';
import type { RecordedActivity } from './studio/history/domain/activity';
const event = (id: string, pathId: string, startedAt: number): RecordedActivity => ({ id, pathId, pathName: pathId, startedAt, seconds: 60, timeZone: utcTimeZone });
test('history buffers account pages and follows empty authorized pages without mutating cursors', async () => {
  const requests: string[] = [];
  const history = historyRepository({
    initial: async () => 'owner',
    read: async (owner, cursor, window) => {
      assert.equal(owner, 'owner'); assert.equal(window, 'retained');
      requests.push(cursor ?? 'first');
      if (!cursor) return { items: [], next: 'accessible' };
      if (cursor === 'accessible') return { items: [event('a1', 'a', 5), event('b1', 'b', 4), event('a2', 'a', 3)], next: 'older' };
      return { items: [event('b2', 'b', 2)], next: null };
    },
  }, 2);
  const first = await history.page(null, undefined, 'retained');
  assert.deepEqual(first.items.map(value => value.id), ['a1', 'b1']);
  const retained = JSON.stringify(first.next);
  const second = await history.page(first.next);
  assert.deepEqual(second.items.map(value => value.id), ['a2', 'b2']);
  assert.equal(second.next, null);
  assert.equal(JSON.stringify(first.next), retained);
  assert.deepEqual(requests, ['first', 'accessible', 'older']);
  await assert.rejects(history.page(first.next, undefined, 'all'), /cursor_invalid/);
});
