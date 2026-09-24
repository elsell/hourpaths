import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionOperationOwner, type SessionAccessState, type SessionFailure } from '@hourpaths/client-core';
import { applyMobileSessionFailure, createSerializedMobileSessionStorage, shouldTransitionMobileSessionForFeatureFailure, type MobileSessionState } from './session-state';

const current = { token: 'stored-token', expiresAt: '2026-07-21T12:00:00Z', nextAction: 'home' as const };

test('secure deletion failure still clears memory before remote revocation and surfaces unreadable state', async () => {
  const events: string[] = [];
  let state: MobileSessionState = {
    session: current,
    accessState: 'authenticated_online',
    retryable: false,
    storageUnreadable: false,
  };
  await assert.doesNotReject(() => applyMobileSessionFailure(
    { kind: 'expired' },
    current,
    {
      discardStored: async () => { events.push('discard'); throw new Error(); },
      transition: (next) => { events.push('transition'); state = next; },
      revoke: async () => { events.push('revoke'); throw new Error(); },
    },
  ));
  assert.deepEqual(events, ['transition', 'discard', 'revoke', 'transition']);
  assert.deepEqual(state, {
    session: null,
    accessState: 'local_session_unreadable',
    retryable: false,
    storageUnreadable: true,
  });
});

test('a pending secure deletion cannot delay the in-memory disposal transition', async () => {
  let state: MobileSessionState | undefined;
  let resolved = false;
  let finishDeletion!: () => void;
  const pendingDeletion = new Promise<void>((resolve) => { finishDeletion = resolve; });
  const disposal = applyMobileSessionFailure(
    { kind: 'expired' },
    current,
    {
      discardStored: async () => { await pendingDeletion; },
      transition: (next) => { state = next; },
      revoke: async () => {},
    },
  ).then(() => { resolved = true; });
  await Promise.resolve();
  await Promise.resolve();
  assert.deepEqual(state, {
    session: null,
    accessState: 'authentication_required',
    retryable: false,
    storageUnreadable: false,
  });
  assert.equal(resolved, false);
  finishDeletion();
  await disposal;
  assert.equal(resolved, true);
});

test('a queued late persistence cannot resurrect a credential after authoritative discard', async () => {
  const events: string[] = [];
  let finishDeletion!: () => void;
  const pendingDeletion = new Promise<void>((resolve) => { finishDeletion = resolve; });
  const storage = createSerializedMobileSessionStorage({
    read: async () => null,
    write: async () => { events.push('write'); },
    discard: async () => { events.push('discard-start'); await pendingDeletion; events.push('discard-end'); },
  });
  const owner = createSessionOperationOwner();
  const ticket = owner.issue();
  const deleting = storage.discard();
  let ready = false;
  const readiness = storage.ready().then(() => { ready = true; });
  const persisting = storage.persist(current, ticket.current);
  await Promise.resolve();
  await Promise.resolve();
  assert.deepEqual(events, ['discard-start']);
  assert.equal(ready, false);
  owner.invalidate();
  finishDeletion();
  await deleting;
  await readiness;
  await persisting;
  assert.deepEqual(events, ['discard-start', 'discard-end']);
});

test('failed secure deletion makes queued and future credential operations unreadable', async () => {
  let writes = 0;
  const storage = createSerializedMobileSessionStorage({
    read: async () => null,
    write: async () => { writes += 1; },
    discard: async () => { throw new Error(); },
  });
  const deleting = storage.discard();
  const queuedPersistence = storage.persist(current, () => true);
  await assert.rejects(deleting, { kind: 'local_storage', reason: 'malformed' });
  await assert.rejects(queuedPersistence, { kind: 'local_storage', reason: 'malformed' });
  await assert.rejects(storage.ready(), { kind: 'local_storage', reason: 'malformed' });
  assert.equal(writes, 0);
});

test('retained failures transition offline without rewriting an unchanged credential', async () => {
  let storageCalls = 0;
  let state: MobileSessionState | undefined;
  const failure: SessionFailure = { kind: 'network' };
  await applyMobileSessionFailure(failure, current, {
    discardStored: async () => { storageCalls += 1; },
    transition: (next) => { state = next; },
    revoke: async () => { storageCalls += 1; },
  });
  assert.equal(storageCalls, 0);
  assert.deepEqual(state, {
    session: current,
    accessState: 'authenticated_offline' satisfies SessionAccessState,
    retryable: true,
    storageUnreadable: false,
  });
});

test('feature-local failures stay local unless the credential must be discarded', () => {
  assert.equal(shouldTransitionMobileSessionForFeatureFailure({ kind: 'network' }), false);
  assert.equal(shouldTransitionMobileSessionForFeatureFailure({ kind: 'http', status: 503 }), false);
  assert.equal(shouldTransitionMobileSessionForFeatureFailure({ kind: 'http', status: 401 }), true);
  assert.equal(shouldTransitionMobileSessionForFeatureFailure({ kind: 'http', status: 403 }), false);
  assert.equal(shouldTransitionMobileSessionForFeatureFailure({ kind: 'http', status: 404 }), false);
});
