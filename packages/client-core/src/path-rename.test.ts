import assert from 'node:assert/strict';
import test from 'node:test';
import {
  applyPathRenameResult,
  createPathRenameOperationOwner,
  reviewPathRename,
} from './path-rename.js';

const managerCapabilities = {
  trackTime: true,
  renamePath: true,
  inviteMembers: true,
  manageGoals: true,
  manageLifecycle: false,
  transferOwnership: false,
};

test('rename review trims a valid proposed name without mutating the authoritative Path', () => {
  const path = { id: 'path-1', name: 'Read', capabilities: managerCapabilities };

  const review = reviewPathRename(path, '  Leer 🎸  ');

  assert.deepEqual(review, {
    pathId: 'path-1',
    expectedName: 'Read',
    name: 'Leer 🎸',
    changed: true,
  });
  assert.equal(Object.isFrozen(review), true);
  assert.equal(path.name, 'Read');
});

test('rename review rejects empty, control-character, and overlong names', () => {
  for (const name of ['', '   ', 'Morning\npractice', 'Morning\u200bpractice', '練'.repeat(101)]) {
    assert.throws(() => reviewPathRename(
      { id: 'path-1', name: 'Read', capabilities: managerCapabilities },
      name,
    ));
  }
});

test('confirmed rename retries one frozen request and ignores stale completion', async () => {
  const keys = ['rename-key-00000001', 'rename-key-00000002'];
  const owner = createPathRenameOperationOwner(() => keys.shift() ?? 'unexpected-key');
  const review = reviewPathRename(
    { id: 'path-1', name: 'Read', capabilities: managerCapabilities },
    'Reading',
  );
  const requests: Array<{
    body: Readonly<{ expectedName: string; name: string }>;
    key: string;
    resolve: (path: { id: string; name: string; capabilities: typeof managerCapabilities }) => void;
  }> = [];
  const request = (
    _pathId: string,
    body: Readonly<{ expectedName: string; name: string }>,
    key: string,
  ) => new Promise<{ id: string; name: string; capabilities: typeof managerCapabilities }>(
    (resolve) => requests.push({ body, key, resolve }),
  );

  const stale = owner.submit(review, request);
  const current = owner.submit(review, request);
  requests[0]?.resolve({ id: 'path-1', name: 'Reading', capabilities: managerCapabilities });
  assert.deepEqual(await stale, { kind: 'superseded' });
  requests[1]?.resolve({ id: 'path-1', name: 'Reading', capabilities: managerCapabilities });
  assert.deepEqual(await current, {
    kind: 'applied',
    path: { id: 'path-1', name: 'Reading', capabilities: managerCapabilities },
  });
  assert.equal(requests[0]?.key, 'rename-key-00000001');
  assert.equal(requests[1]?.key, 'rename-key-00000001');
  assert.strictEqual(requests[0]?.body, requests[1]?.body);
  assert.equal(Object.isFrozen(requests[0]?.body), true);
});

test('cancel performs no request and a changed proposal owns a new replay key', async () => {
  const keys = ['rename-key-00000001', 'rename-key-00000002'];
  const owner = createPathRenameOperationOwner(() => keys.shift() ?? 'unexpected-key');
  const path = { id: 'path-1', name: 'Read', capabilities: managerCapabilities };
  let calls = 0;
  const usedKeys: string[] = [];
  const request = async (_pathId: string, _body: unknown, key: string) => {
    calls += 1;
    usedKeys.push(key);
    return path;
  };

  assert.deepEqual(await owner.submit(reviewPathRename(path, 'Reading'), request, false), {
    kind: 'cancelled',
  });
  assert.equal(calls, 0);

  const first = await owner.submit(reviewPathRename(path, 'Reading'), request);
  const second = await owner.submit(reviewPathRename(path, 'Books'), request);
  assert.equal(first.kind, 'failed');
  assert.equal(second.kind, 'failed');
  assert.equal(calls, 2);
  assert.deepEqual(usedKeys, ['rename-key-00000001', 'rename-key-00000002']);
});

test('authoritative rename replaces Home and selected Path without disturbing timers or peers', () => {
  const path = { id: 'path-1', name: 'Read', capabilities: managerCapabilities };
  const peer = { id: 'path-2', name: 'Piano', capabilities: managerCapabilities };
  const timerStates = { 'path-1': { running: true, accumulatedSeconds: 42 } };
  const state = { paths: [path, peer], selectedPath: path, timerStates };
  const renamed = { ...path, name: 'Reading' };

  const next = applyPathRenameResult(state, renamed);

  assert.deepEqual(next.paths[0], renamed);
  assert.deepEqual(next.selectedPath, renamed);
  assert.strictEqual(next.paths[1], peer);
  assert.strictEqual(next.timerStates, timerStates);
  assert.equal(state.paths[0]?.name, 'Read');
});
