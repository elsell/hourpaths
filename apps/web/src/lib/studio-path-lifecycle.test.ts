const pathName = 'practice-path';
import assert from 'node:assert/strict';
import { test } from 'node:test';
import { reviewLifecycle, executeLifecycle } from './studio/paths/application/lifecycle';

const path = { id: 'path', name: pathName, archived: false, canManageLifecycle: true };
test('lifecycle review rejects missing creator capability and captures exact review state', () => {
  assert.throws(() => reviewLifecycle({ ...path, canManageLifecycle: false }, 'delete', 'key'));
  const review = reviewLifecycle(path, 'archive', 'key');
  path.name = 'Changed after review';
  assert.equal(review.name, 'practice-path');
  assert.equal(review.expectedArchived, false);
});
test('retry preserves reviewed state and idempotency key after an uncertain failure', async () => {
  const review = reviewLifecycle({ ...path, name: pathName }, 'delete', 'stable-key');
  const requests: unknown[] = [];
  const repository = { async lifecycle(value: typeof review) { requests.push(value); if (requests.length === 1) throw new Error('offline'); } };
  await assert.rejects(executeLifecycle(repository, review));
  await executeLifecycle(repository, review);
  assert.deepEqual(requests, [review, review]);
});
test('restore captures archived precondition and refuses invalid direction', () => {
  assert.equal(reviewLifecycle({ ...path, archived: true }, 'restore', 'key').expectedArchived, true);
  assert.throws(() => reviewLifecycle(path, 'restore', 'key'));
  assert.throws(() => reviewLifecycle({ ...path, archived: true }, 'archive', 'key'));
});
