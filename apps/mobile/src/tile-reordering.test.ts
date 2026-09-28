import assert from 'node:assert/strict';
import test from 'node:test';
import { mergeVisibleTileOrder, moveTile, nearestTileSlot } from './tile-reordering';

test('dragging a filtered collection preserves hidden and archived slots without losing paths', () => {
  const saved = ['pinned', 'a', 'hidden', 'b', 'archived', 'c'];
  const moved = moveTile(['a', 'b', 'c'], 'c', 0);
  assert.deepEqual(mergeVisibleTileOrder(saved, moved), ['pinned', 'c', 'hidden', 'a', 'archived', 'b']);
  assert.deepEqual(mergeVisibleTileOrder(saved, ['a', 'a']), saved);
  assert.deepEqual(mergeVisibleTileOrder(saved, ['foreign']), saved);
});

test('drop targeting follows two columns and a final partial row, including auto-scroll distance', () => {
  const slots = [
    { x: 0, y: 0, width: 150, height: 220 }, { x: 165, y: 0, width: 150, height: 220 },
    { x: 0, y: 232, width: 150, height: 190 },
  ];
  assert.equal(nearestTileSlot(slots, 240, 100), 1);
  assert.equal(nearestTileSlot(slots, 80, 100 + 240), 2);
  assert.deepEqual(moveTile(['a', 'b', 'c'], 'a', 2), ['b', 'c', 'a']);
  assert.deepEqual(moveTile(['a', 'b'], 'a', -1), ['a', 'b']);
});
