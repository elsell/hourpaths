const utcTimeZone = 'utc';
import assert from 'node:assert/strict';
import test from 'node:test';
import { historyRepository } from './studio/history/application/history';
import type { RecordedActivity } from './studio/history/domain/activity';
const event = (id: string, pathId: string, startedAt: number): RecordedActivity => ({ id, pathId, pathName: pathId, startedAt, seconds: 60, timeZone: utcTimeZone });
test('history merges interleaved path pages without skipping buffered events or mutating cursors', async () => {
  const history = historyRepository({
    initial: async () => ({ participantId: 'owner', streams: ['a', 'b'].map(pathId => ({ pathId, pathName: pathId, remaining: [], cursor: null, loaded: false })) }),
    read: async (pathId, _name, owner, cursor) => {
      assert.equal(owner, 'owner');
      if (pathId === 'a') return cursor ? { items: [event('a2', 'a', 3)], next: null } : { items: [event('a1', 'a', 5)], next: 'older' };
      return { items: [event('b1', 'b', 4), event('b2', 'b', 2)], next: null };
    },
  }, 2);
  const first = await history.page(null);
  assert.deepEqual(first.items.map(value => value.id), ['a1', 'b1']);
  const retained = JSON.stringify(first.next);
  const second = await history.page(first.next);
  assert.deepEqual(second.items.map(value => value.id), ['a2', 'b2']);
  assert.equal(second.next, null);
  assert.equal(JSON.stringify(first.next), retained);
});
