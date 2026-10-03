import assert from 'node:assert/strict';
import test from 'node:test';
import { AccountDeletion, type AccountDeletionPorts, type DeletionIntent } from './account-deletion';

function environment() {
  let owner: string | null = 'first';
  let intent: DeletionIntent | null = null;
  let removed = false;
  let loseResponse = false;
  let storageFailure = false;
  const retained = new Set(['first', 'second']);
  const fenced = new Set<string>();
  const ports: AccountDeletionPorts = {
    currentOwner: () => owner,
    newSecret: () => 'a'.repeat(64),
    journal: {
      read: async account => intent?.owner === account ? structuredClone(intent) : null,
      save: async value => { if (storageFailure) throw new Error('storage_failed'); if (!intent || intent.receiptSecret === value.receiptSecret && intent.phase !== 'confirmed') intent = structuredClone(value); return structuredClone(intent); },
      forget: async account => { if (intent?.owner === account) intent = null; },
    },
    fence: async account => { fenced.add(account); },
    remote: {
      remove: async account => {
        assert.ok(fenced.has(account)); assert.equal(intent?.owner, account);
        assert.equal(owner, account); removed = true;
        if (loseResponse) throw new TypeError('response_lost');
      },
      receipt: async account => removed && account === 'first',
    },
    purge: async account => { retained.delete(account); },
    clearSession: async account => { if (owner === account) owner = null; },
  };
  return { ports, retained, fenced, owner: () => owner, intent: () => intent, removed: () => removed,
    switchAccount: () => { owner = 'second'; }, loseResponse: () => { loseResponse = true; }, failStorage: () => { storageFailure = true; } };
}

test('lost deletion response is proven by receipt and cleanup cannot erase a replacement account', async () => {
  const state = environment(); state.loseResponse();
  const remove = state.ports.remote.remove;
  state.ports.remote.remove = async account => { try { await remove(account, 'a'.repeat(64)); } finally { state.switchAccount(); } };
  await new AccountDeletion(state.ports).confirm('first');
  assert.equal(state.owner(), 'second');
  assert.deepEqual([...state.retained], ['second']);
  assert.equal(state.intent(), null);
});

test('restart completes proven deletion after cleanup failure without deleting twice', async () => {
  const state = environment(); const purge = state.ports.purge;
  state.ports.purge = async () => { throw new Error('disk_unavailable'); };
  await assert.rejects(new AccountDeletion(state.ports).confirm('first'), /disk_unavailable/);
  assert.equal(state.intent()?.phase, 'confirmed');
  state.switchAccount();
  state.ports.purge = purge;
  state.ports.remote.remove = async () => { throw new Error('must_not_delete_again'); };
  state.ports.remote.receipt = async () => { throw new Error('receipt_expired'); };
  await new AccountDeletion(state.ports).resume('first');
  assert.deepEqual([...state.retained], ['second']);
  assert.equal(state.owner(), 'second');
});

test('failure to persist intent cannot send deletion or clear retained activity', async () => {
  const state = environment(); state.failStorage();
  await assert.rejects(new AccountDeletion(state.ports).confirm('first'), /storage_failed/);
  assert.equal(state.removed(), false);
  assert.deepEqual([...state.retained], ['first', 'second']);
});
