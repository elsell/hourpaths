import assert from 'node:assert/strict';
import test from 'node:test';
import { browserEntryService, type ProviderCallbackPort } from './studio/entry/adapters/browser-entry-service';
import { applicationSession, applicationSessionOperations, initializeApplicationSession, persistOwnedApplicationSession, clearApplicationSession, pauseApplicationSession } from './auth';
import { browserSessionState } from './browser-session-state';
import { providerLinkIntentStorage } from './provider-auth';

function storage() {
  const values = new Map<string, string>();
  return { getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
}
const config = { environment: 'development', apiURL: 'http://api.example.test', oidcIssuer: 'http://identity.example.test', oidcClientId: 'web' } as const;

test('provider callbacks preserve account ownership and clean up account-bound intent', async () => {
  const localStorage = storage(), sessionStorage = storage();
  let tail: Promise<unknown> = Promise.resolve();
  const locks = { request<T>(_name: string, operation: () => T | PromiseLike<T>): Promise<T> { const next = tail.then(operation); tail = next.catch(() => undefined); return next; } };
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { localStorage, sessionStorage, navigator: { locks } } });
  await initializeApplicationSession();
  const session = { token: 'application-session', ownerId: 'account-a', nextAction: 'home' as const, expiresAt: '2099-01-01T00:00:00.000Z' };
  await persistOwnedApplicationSession(session, applicationSessionOperations.issue());
  const pending = providerLinkIntentStorage();
  const intent = { owner: session.ownerId, id: 'challenge-a', provider: 'apple' as const, nonce: `hourpaths-link:${'a'.repeat(64)}`, expiresAt: Date.parse(session.expiresAt) };
  await pending.save(intent);
  let completed = 0;
  const callbacks: ProviderCallbackPort = {
    async authenticate() { return { identityToken: 'verified-provider-proof', state: { purpose: 'identity-link', owner: intent.owner, challengeId: intent.id } }; },
    async link(token) { assert.equal(token, 'verified-provider-proof'); assert.equal(applicationSession()?.token, session.token); completed++; },
  };
  assert.equal((await browserEntryService(config, () => 0, callbacks).callback()).kind, 'home');
  assert.equal(completed, 1);
  assert.equal(applicationSession()?.token, session.token);
  // Linking does not need a sign-in intent; ordinary sign-in still does.
  await assert.rejects(browserEntryService(config, () => 0, { ...callbacks, async authenticate() { return { identityToken: 'proof', state: undefined }; } }).callback());
  assert.equal(applicationSession()?.token, session.token);
  // A late provider response after logout cannot link or replace the account.
  await assert.rejects(browserEntryService(config, () => 0, { ...callbacks, async authenticate() { await clearApplicationSession(); return callbacks.authenticate(); } }).callback());
  assert.equal(completed, 1);
  assert.equal(await pending.read(), null);
  await persistOwnedApplicationSession(session, applicationSessionOperations.issue());
  assert.equal(applicationSession()?.token, session.token);
  await pending.save(intent);
  await pauseApplicationSession(session.ownerId, undefined, session.token);
  assert.equal(await pending.read(), null);
  await pending.save({ ...intent, owner: 'replacement-account' });
  await browserSessionState().discardOwner(session.ownerId);
  assert.equal((await pending.read())?.owner, 'replacement-account');
  await browserSessionState().discardOwner('replacement-account');
  assert.equal(await pending.read(), null);
  delete (globalThis as { window?: unknown }).window;
});
