import test from 'node:test';
import assert from 'node:assert/strict';
import { createPathAppearanceStore, type SavedPathAppearance } from './path-appearance';

test('appearance saves retain retry identity, reject stale refreshes, and discard late account results', async () => {
  let resolveRead: ((value: SavedPathAppearance) => void) | undefined;
  let delayed = false;
  let fail = true;
  let changes = 0;
  const keys: string[] = [];
  const store = createPathAppearanceStore({
    read: async () => delayed ? new Promise((resolve) => { resolveRead = resolve; }) : { revision: 0 },
    save: async (_id, value, revision, key) => {
      keys.push(key);
      if (fail) { fail = false; throw new Error('connection_lost'); }
      return { ...value, revision: revision + 1 };
    },
  }, () => `attempt-${keys.length}`, () => { changes++; });
  await store.refresh(['path']);
  delayed = true;
  const stale = store.refresh(['path']);
  const choice = { color: 'mint', emoji: '🌱' } as const;
  await assert.rejects(store.save('path', choice));
  assert.equal(await store.save('path', choice), true);
  assert.deepEqual(keys, ['attempt-0', 'attempt-0']);
  resolveRead!({ revision: 0 }); await stale;
  assert.deepEqual(store.appearance('path'), choice);
  const late = store.refresh(['path']);
  store.dispose();
  const before = changes;
  resolveRead!({ revision: 2, color: 'blue', emoji: '🎨' }); await late;
  assert.equal(changes, before);
  assert.equal(store.isLoaded('path'), false);
});

test('an open draft saves against its original revision after another client changes appearance', async () => {
  let server: SavedPathAppearance = { revision: 1, color: 'mint', emoji: '🌱' };
  const store = createPathAppearanceStore({
    read: async () => server,
    save: async (_id, appearance, revision) => {
      if (revision !== server.revision) throw { status: 409 };
      server = { ...appearance, revision: revision + 1 };
      return server;
    },
  }, () => 'new-attempt', () => {});
  await store.refresh(['path']);
  const openedRevision = store.revision('path');
  server = { revision: 2, color: 'blue', emoji: '🎨' };
  await store.refresh(['path']);
  const draft = { color: 'coral', emoji: '🎹' } as const;
  await assert.rejects(store.save('path', draft, openedRevision));
  assert.equal(server.color, 'blue');
  assert.equal(await store.save('path', draft, store.revision('path')), true);
  assert.equal(server.color, 'coral');
});
