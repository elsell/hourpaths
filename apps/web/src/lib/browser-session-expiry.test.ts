import assert from 'node:assert/strict';
import { test } from 'node:test';
import { applicationSession, applicationSessionOperations, clearApplicationSession } from './auth';
import { browserSessionStore } from './studio/session/adapters/browser-session';

test('cold expiry replaces a verified credential with a stoppable owner reference; logout clears it', async () => {
  const values = new Map<string, string>();
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value); },
    removeItem: (key: string) => { values.delete(key); },
  };
  const original = Object.getOwnPropertyDescriptor(globalThis, 'window');
  const originalNavigator = Object.getOwnPropertyDescriptor(globalThis, 'navigator');
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { sessionStorage: storage, localStorage: storage, navigator: { locks: { request: async (_name: string, operation: () => unknown) => operation() } } } });
  Object.defineProperty(globalThis, 'navigator', { configurable: true, value: { locks: { request: async (_name: string, operation: () => unknown) => operation() } } });
  try {
    storage.setItem('hourpaths_application_session', JSON.stringify({
      token: 'expired', expiresAt: new Date(1000).toISOString(), ownerId: 'alice', nextAction: 'home'
    }));
    const store = browserSessionStore(() => 1000);
    await store.initialize?.();
    assert.equal(store.read(), null);
    assert.equal(store.retainedOwner?.(), 'alice');
    assert.equal(JSON.parse(JSON.parse(storage.getItem('hourpaths_session_record_v1')!).value).token, undefined);
    await store.clear();
    assert.equal(store.retainedOwner?.(), null);
    await applicationSessionOperations.issue().persist(JSON.stringify({
      token: 'unverified', expiresAt: new Date(1000).toISOString(), nextAction: 'home'
    }));
    await store.initialize?.();
    assert.equal(store.read(), null);
    assert.equal(store.retainedOwner?.(), null);
    const active = { token: 'alice-old', expiresAt: new Date(9000).toISOString(), nextAction: 'home', ownerId: 'alice' };
    await applicationSessionOperations.issue().persist(JSON.stringify(active));
    assert.equal(applicationSession()?.token, active.token);
    const durableKey = 'hourpaths_session_record_v1';
    const before = JSON.parse(storage.getItem(durableKey)!);
    storage.setItem(durableKey, JSON.stringify({ ...before, revision: 'other-tab-rotation', value: JSON.stringify({ ...active, token: 'alice-rotated' }) }));
    await clearApplicationSession(undefined, active.token);
    assert.equal(applicationSession(), null);
    await applicationSessionOperations.issue().persist(JSON.stringify(active));
    assert.equal(applicationSession()?.token, active.token);
    const next = JSON.parse(storage.getItem(durableKey)!);
    storage.setItem(durableKey, JSON.stringify({ ...next, revision: 'bob-sign-in', family: 'bob-family', value: JSON.stringify({ ...active, token: 'bob', ownerId: 'bob' }) }));
    await clearApplicationSession(undefined, active.token);
    assert.equal(applicationSession()?.token, 'bob');
    await applicationSessionOperations.issue().persist(JSON.stringify({
      token: 'onboarding', expiresAt: new Date(1000).toISOString(), ownerId: 'alice', nextAction: 'onboarding'
    }));
    await store.initialize?.();
    assert.equal(store.read(), null);
    assert.equal(store.retainedOwner?.(), null);
  } finally {
    if (originalNavigator) Object.defineProperty(globalThis, 'navigator', originalNavigator);
    else Reflect.deleteProperty(globalThis, 'navigator');
    if (original) Object.defineProperty(globalThis, 'window', original);
    else Reflect.deleteProperty(globalThis, 'window');
  }
});
