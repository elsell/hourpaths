import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { DeletionIntent } from '@hourpaths/client-core';
import { deletionSurfaceJournal } from './account-deletion-journal';

test('native deletion persists notification cleanup before admitting a recoverable intent', async () => {
  let intent: DeletionIntent | null = null;
  let surfaces: string[] | null = null;
  let failCapture = true;
  const events: string[] = [];
  const journal = deletionSurfaceJournal({
    currentOwner: () => 'alice',
    capture: async () => { if (failCapture) throw new Error('native_unavailable'); return ['notice']; },
    local: {
      read: async () => intent,
      save: async value => { assert.deepEqual(surfaces, ['notice']); events.push('intent'); intent = value; return value; },
      forget: async () => { intent = null; },
      readSurfaces: async () => surfaces,
      saveSurfaces: async (_owner, ids) => { events.push('surfaces'); surfaces = ids; },
    },
  });
  const request: DeletionIntent = { owner: 'alice', receiptSecret: '01'.repeat(32), phase: 'pending' };
  await assert.rejects(journal.save(request), /native_unavailable/);
  assert.equal(intent, null);
  failCapture = false;
  await journal.save(request);
  assert.deepEqual(events, ['surfaces', 'intent']);
});

test('account switch during notification capture cannot persist a deletion intent', async () => {
  let owner = 'alice';
  let writes = 0;
  const journal = deletionSurfaceJournal({
    currentOwner: () => owner,
    capture: async () => { owner = 'bob'; return ['bob-notice']; },
    local: {
      read: async () => null, save: async value => { writes++; return value; }, forget: async () => {},
      readSurfaces: async () => null, saveSurfaces: async () => { writes++; },
    },
  });
  await assert.rejects(journal.save({ owner: 'alice', receiptSecret: '01'.repeat(32), phase: 'pending' }), /account_changed/);
  assert.equal(writes, 0);
});


test('saved cleanup identifiers allow completion after restart without the old credential', async () => {
  let saved: DeletionIntent = { owner: 'alice', receiptSecret: '01'.repeat(32), phase: 'pending' };
  const journal = deletionSurfaceJournal({
    currentOwner: () => null,
    capture: async () => { throw new Error('old_session_required'); },
    local: {
      read: async () => saved,
      save: async value => { saved = value; return value; },
      forget: async () => {},
      readSurfaces: async () => ['alice-notice'],
      saveSurfaces: async () => { throw new Error('unexpected_recapture'); },
    },
  });
  await journal.save({ ...saved, phase: 'confirmed' });
  assert.equal(saved.phase, 'confirmed');
});
