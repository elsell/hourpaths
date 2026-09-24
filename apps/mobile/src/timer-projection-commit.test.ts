import assert from 'node:assert/strict';
import test from 'node:test';

import { commitTimerProjectionBeforeRender } from './timer-projection-commit';

test('an acknowledged timer projection is authoritative before a delayed React render', () => {
  const oldProjection = { running: false };
  const nextProjection = { running: true };
  let authoritative = oldProjection;
  let rendered = oldProjection;
  let queuedRender: (() => void) | undefined;

  commitTimerProjectionBeforeRender(
    nextProjection,
    (next) => { authoritative = next; },
    (next) => { queuedRender = () => { rendered = next; }; },
  );

  assert.equal(authoritative, nextProjection);
  assert.equal(rendered, oldProjection);
  queuedRender?.();
  assert.equal(rendered, nextProjection);
});
