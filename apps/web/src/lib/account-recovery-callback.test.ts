import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import test from 'node:test';
import { browserAccountRecovery } from './studio/entry/adapters/browser-account-recovery';
import { browserEntryService } from './studio/entry/adapters/browser-entry-service';
import { applicationSession, applicationSessionOperations, initializeApplicationSession, persistOwnedApplicationSession, clearApplicationSession } from './auth';
import { browserSessionState } from './browser-session-state';
import { providerRecoveryIntentStorage } from './provider-auth';

function storage() { const values = new Map<string, string>(); return { getItem: (k: string) => values.get(k) ?? null, setItem: (k: string, v: string) => { values.set(k, v); }, removeItem: (k: string) => { values.delete(k); } }; }
test('recovery callback adopts only its enrollment, uses its frozen token, and revokes a late result', async () => {
  let tail: Promise<unknown> = Promise.resolve();
  const locks = { request<T>(_name: string, work: () => T | PromiseLike<T>): Promise<T> { const next = tail.then(work); tail = next.catch(() => undefined); return next; } };
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { localStorage: storage(), sessionStorage: storage(), navigator: { locks } } });
  await initializeApplicationSession();
  const source = { token: 'enrollment', nextAction: 'duplicate_email_recovery' as const, expiresAt: '2099-01-01T00:00:00.000Z' };
  const recovered = { ...source, token: 'recovered', nextAction: 'home' as const };
  let completions = 0, revoke!: () => void, arrive!: () => void, release!: () => void, defer = false;
  const arrived = new Promise<void>(r => { arrive = r; });
  const revoked = new Promise<void>(r => { revoke = r; });
  const server = createServer(async (req, res) => {
    if (req.method === 'DELETE') {
      assert.equal(req.headers.authorization, 'Bearer recovered');
      res.writeHead(204).end();
      revoke();
      return;
    }
    assert.equal(req.url, '/v1/onboarding/duplicate-email-recovery/complete');
    assert.equal(req.headers.authorization, 'Bearer enrollment');
    let body = ''; for await (const chunk of req) body += chunk;
    assert.deepEqual(JSON.parse(body), { challengeId: 'challenge', identityToken: 'signed-proof' });
    completions++;
    if (defer) { arrive(); await new Promise<void>(r => { release = r; }); }
    res.setHeader('Content-Type', 'application/json'); res.end(JSON.stringify({ data: recovered }));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  const config = { environment: 'development' as const, apiURL: `http://127.0.0.1:${(server.address() as { port: number }).port}`, oidcIssuer: 'https://id.example.test', oidcClientId: 'web' };
  async function prepare() {
    await persistOwnedApplicationSession(source, applicationSessionOperations.issue());
    const lifecycle = browserSessionState().revision();
    await providerRecoveryIntentStorage().save({ lifecycle, id: 'challenge', provider: 'apple', nonce: 'hourpaths-recovery:' + 'a'.repeat(64), expiresAt: Date.parse(source.expiresAt) });
    const state = { purpose: 'account-recovery', lifecycle, challengeId: 'challenge' };
    const service = browserEntryService(config, () => 0, {
      authenticate: async () => ({ identityToken: 'signed-proof', state }),
      link: async () => { assert.fail(); },
      recover: (proof, state, ticket) => browserAccountRecovery(config).complete(proof, state, ticket),
    });
    return service;
  }
  try {
    let service = await prepare();
    assert.equal((await service.callback()).kind, 'home');
    assert.deepEqual(applicationSession(), recovered);
    assert.equal(await providerRecoveryIntentStorage().read(), null);
    service = await prepare();
    await clearApplicationSession();
    await persistOwnedApplicationSession({ ...recovered, token: 'replacement' }, applicationSessionOperations.issue());
    await assert.rejects(service.callback());
    assert.equal(completions, 1);
    assert.equal(applicationSession()?.token, 'replacement');
    service = await prepare(); defer = true;
    const pending = service.callback(); await arrived;
    await clearApplicationSession();
    await persistOwnedApplicationSession({ ...recovered, token: 'replacement' }, applicationSessionOperations.issue());
    release(); await assert.rejects(pending); await revoked;
    assert.equal(applicationSession()?.token, 'replacement');
    assert.equal(completions, 2);
  } finally { server.close(); server.closeAllConnections(); Reflect.deleteProperty(globalThis, 'window'); }
});
