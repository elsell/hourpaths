import assert from 'node:assert/strict';
import { test } from 'node:test';
import { browserSessionRecords, type SessionLocks } from './studio/session/adapters/browser-session-record';

function storage() {
  const values = new Map<string, string>();
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value); },
    removeItem: (key: string) => { values.delete(key); },
  };
}
function locks(): SessionLocks {
  let tail: Promise<unknown> = Promise.resolve();
  return { request<T>(_name: string, operation: () => T | PromiseLike<T>): Promise<T> {
    const result = tail.then(operation);
    tail = result.catch(() => undefined);
    return result;
  } };
}

test('a fresh browser context restores the durable record and a tombstone prevents legacy resurrection', async () => {
  const durable = storage(), original = storage(), mutex = locks();
  let id = 0;
  const revision = () => String(++id);
  original.setItem('hourpaths_application_session', 'opaque-session');
  const first = browserSessionRecords(durable, original, mutex, revision);
  assert.equal((await first.initialize()).value, 'opaque-session');
  assert.equal(original.getItem('hourpaths_application_session'), null);
  const reopened = browserSessionRecords(durable, storage(), mutex, revision);
  assert.equal((await reopened.initialize()).value, 'opaque-session');
  await reopened.coordinator.replace(null);
  const oldTab = storage();
  oldTab.setItem('hourpaths_application_session', 'opaque-session');
  assert.equal((await browserSessionRecords(durable, oldTab, mutex, revision).initialize()).value, null);
  assert.equal(oldTab.getItem('hourpaths_application_session'), null);
});

test('malformed durable storage never falls back to a legacy credential', async () => {
  const durable = storage(), old = storage();
  durable.setItem('hourpaths_session_record_v1', '{');
  old.setItem('hourpaths_application_session', 'old-session');
  const records = browserSessionRecords(durable, old, locks(), () => 'next');
  await assert.rejects(records.initialize());
  assert.throws(() => records.coordinator.read());
  assert.equal(old.getItem('hourpaths_application_session'), 'old-session');
  await records.discardUnreadable();
  assert.equal((await records.initialize()).value, null);
  assert.equal(old.getItem('hourpaths_application_session'), null);
});

test('a failed durable migration preserves the original credential for bounded recovery', async () => {
  const old = storage();
  old.setItem('hourpaths_application_session', 'old-session');
  const records = browserSessionRecords({ ...storage(), setItem: () => { throw new Error('quota'); } }, old, locks(), () => 'next');
  await assert.rejects(records.initialize());
  assert.equal(old.getItem('hourpaths_application_session'), 'old-session');
});
