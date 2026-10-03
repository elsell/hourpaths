import assert from 'node:assert/strict';
import { test } from 'node:test';
import { SessionRecordCoordinator, type SessionRecord } from './studio/session/application/session-record';
import { DurableSessionOperations } from './studio/session/application/session-operations';

function origin() {
  let record: SessionRecord | null = null;
  let tail: Promise<unknown> = Promise.resolve();
  let revision = 0;
  const port = {
    read: () => record,
    write: (value: SessionRecord) => { record = value; },
    exclusive<T>(operation: () => T): Promise<T> {
      const result = tail.then(operation);
      tail = result.catch(() => undefined);
      return result;
    },
  };
  return () => new SessionRecordCoordinator(port, () => String(++revision));
}

test('logout from an empty origin fences an already pending sign-in in another tab', async () => {
  const tab = origin(), signingIn = tab(), signingOut = tab();
  const before = signingIn.read();
  await signingOut.replace(null);
  assert.equal(await signingIn.commit(before.revision, 'late-credential'), null);
  assert.equal(signingIn.read().value, null);
});

test('two tabs cannot both replace the same credential revision', async () => {
  const tab = origin(), first = tab(), second = tab();
  const initial = await first.replace('original');
  const results = await Promise.all([
    first.commit(initial.revision, 'replacement'),
    second.commit(initial.revision, 'stale-response'),
  ]);
  assert.equal(results.filter(Boolean).length, 1);
  assert.equal(second.read().value, 'replacement');
  await second.replace('another-account');
  assert.equal(await first.commit(initial.revision, 'original-owner'), null);
  assert.equal(first.read().value, 'another-account');
});

test('pending local persistence rechecks cancellation inside the shared lock', async () => {
  const records = origin()();
  const operations = new DurableSessionOperations(records);
  const ticket = operations.issue();
  const pending = ticket.persist('late-response');
  operations.invalidate();
  assert.equal(await pending, false);
  assert.equal(records.read().value, null);
});

test('a committed sign-in remains current only until another tab replaces the account', async () => {
  const tab = origin(), records = tab(), other = tab();
  const operations = new DurableSessionOperations(records);
  const ticket = operations.issue();
  assert.equal(await ticket.persist('alice'), true);
  assert.equal(ticket.current(), true);
  await other.replace('bob');
  assert.equal(ticket.current(), false);
  assert.equal(await ticket.persist('late-alice-profile'), false);
  assert.equal(records.read().value, 'bob');
});

test('a redirect callback uses its original sign-in revision rather than the account present on return', async () => {
  const records = origin()();
  const intent = records.read().revision;
  await records.replace(null);
  const callback = new DurableSessionOperations(records).issue(intent);
  assert.equal(callback.current(), false);
  assert.equal(await callback.persist('provider-response'), false);
  assert.equal(records.read().value, null);
});

test('logout clears a queued rotation of its family but preserves a replacement account', async () => {
  const tab = origin(), first = tab(), other = tab();
  const original = await first.replace('alice');
  const rotating = other.commit(original.revision, 'rotated-alice');
  const logout = first.discardFamily(original.family, null);
  await Promise.all([rotating, logout]);
  assert.equal(first.read().value, null);
  const alice = await first.replace('alice');
  const signingIn = new DurableSessionOperations(other).issue(alice.revision, true).persist('bob');
  const staleLogout = first.discardFamily(alice.family, null);
  await Promise.all([signingIn, staleLogout]);
  assert.equal(first.read().value, 'bob');
});

test('deletion clears a captured owner across rotation but preserves an account switched before the lock', async () => {
  const tab = origin(), deletion = tab(), other = tab();
  await other.replace(JSON.stringify({ ownerId: 'alice', token: 'old' }));
  const rotation = other.commit(other.read().revision, JSON.stringify({ ownerId: 'alice', token: 'rotated' }));
  const removal = deletion.discardOwner('alice');
  await rotation;
  assert.ok(await removal);
  assert.equal(other.read().value, null);
  await other.replace(JSON.stringify({ ownerId: 'alice', token: 'old' }));
  const switchAccount = other.replace(JSON.stringify({ ownerId: 'bob', token: 'replacement' }));
  const lateRemoval = deletion.discardOwner('alice');
  await switchAccount;
  assert.equal(await lateRemoval, null);
  assert.equal(JSON.parse(other.read().value!).ownerId, 'bob');
  await other.replace(JSON.stringify({ ownerId: 'alice', kind: 'retained_account' }));
  assert.ok(await deletion.discardOwner('alice'));
  assert.equal(other.read().value, null);
});
