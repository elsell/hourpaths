import assert from 'node:assert/strict';
import { once } from 'node:events';
import { createServer } from 'node:http';
import test from 'node:test';
import { browserEntryService } from './studio/entry/adapters/browser-entry-service';
import { EntryFailure } from './studio/entry/domain/entry';

// Controlled browser storage and a real HTTP boundary exercise the extracted entry flow.
test('entry maps policy review, preserves transient credentials, and never adopts a disposed activation', async () => {
  const now = Date.parse('2030-01-01T00:00:00Z');
  const original = { token: 'onboarding-credential', expiresAt: new Date(now + 60_000).toISOString(), nextAction: 'onboarding' };
  const replacement = { ...original, token: 'activated-credential', nextAction: 'home' };
  const records = new Map<string, string>();
  const key = 'hourpaths_application_session';
  const store = { getItem: (name: string) => records.get(name) ?? null, setItem: (name: string, value: string) => { records.set(name, value); }, removeItem: (name: string) => { records.delete(name); } };
  const before = Object.getOwnPropertyDescriptor(globalThis, 'window');
  Object.defineProperty(globalThis, 'window', { configurable: true, value: { sessionStorage: store } });
  let failure = 0, code = '', deferred = false, finish!: () => void, started!: () => void, revoked!: () => void;
  const pending = new Promise<void>(resolve => { started = resolve; });
  const revocation = new Promise<void>(resolve => { revoked = resolve; });
  let body: Record<string, unknown> | undefined;
  const policy = { url: 'http://localhost:5173/legal/policy', version: 'v1' };
  const server = createServer(async (request, response) => {
    response.setHeader('Content-Type', 'application/json');
    if (request.method === 'DELETE') {
      assert.equal(request.headers.authorization, 'Bearer activated-credential');
      response.statusCode = 204; response.end(); revoked(); return;
    }
    assert.equal(request.headers.authorization, 'Bearer onboarding-credential');
    if (request.method === 'POST') { let text = ''; for await (const chunk of request) text += chunk; body = JSON.parse(text); if (deferred) { started(); await new Promise<void>(resolve => { finish = resolve; }); } }
    response.statusCode = failure || 200;
    response.end(JSON.stringify(failure ? { code, status: failure } : { data: request.method === 'POST' ? replacement : { email: 'person@example.test', displayName: '', usernameSuggestion: 'person', policyReviewToken: 'policy-1', policies: { termsOfService: policy, privacyPolicy: policy, communityGuidelines: policy, supportUrl: policy.url } } }));
  });
  server.listen(0, '127.0.0.1'); await once(server, 'listening');
  const port = (server.address() as { port: number }).port;
  const config = { environment: 'development' as const, apiURL: `http://127.0.0.1:${port}`, oidcIssuer: 'https://identity.example.test', oidcClientId: 'client' };
  const input = { username: 'person', displayName: 'person', visibility: 'private' as const, 
    timeZone: 'UTC',
    firstDayOfWeek: 1, reviewToken: 'policy-1', age: true, terms: true, privacy: true, guidelines: true };
  try {
    store.setItem(key, JSON.stringify(original));
    const service = browserEntryService(config, () => now); await service.restore();
    const review = await service.review(); assert.equal(review.token, 'policy-1'); assert.equal(review.displayName, ''); assert.equal(review.policies.privacy.version, 'v1');
    assert.throws(() => service.openPolicy(review.policies.privacy.url), (error: unknown) => error instanceof EntryFailure && error.kind === 'unavailable');
    failure = 503; code = 'unavailable'; await assert.rejects(service.review(), (error: unknown) => error instanceof EntryFailure && error.kind === 'unavailable'); assert.ok(store.getItem(key));
    failure = 403; code = 'forbidden'; await assert.rejects(service.review(), (error: unknown) => error instanceof EntryFailure && error.kind === 'forbidden'); assert.ok(store.getItem(key));
    failure = 409; code = 'username_unavailable'; await assert.rejects(service.activate(input), (error: unknown) => error instanceof EntryFailure && error.kind === 'username');
    code = 'policy_set_changed'; await assert.rejects(service.activate(input), (error: unknown) => error instanceof EntryFailure && error.kind === 'policy');
    failure = 0; deferred = true;
    const late = service.activate(input); await pending; service.dispose(); finish();
    await assert.rejects(late, (error: unknown) => error instanceof EntryFailure && error.kind === 'superseded'); await revocation;
    assert.deepEqual(JSON.parse(store.getItem(key)!), original);
    assert.deepEqual(body, { username: 'person', displayName: 'person', profileVisibility: 'private', 
    timeZone: 'UTC',
    firstDayOfWeek: 1, policyReviewToken: 'policy-1', atLeast16: true, termsAccepted: true, privacyAcknowledged: true, communityGuidelinesAccepted: true });
    const rejected = browserEntryService(config, () => now); await rejected.restore(); failure = 401; code = 'invalid_credential';
    await assert.rejects(rejected.review(), (error: unknown) => error instanceof EntryFailure && error.kind === 'expired'); assert.equal(store.getItem(key), null);
    store.setItem(key, JSON.stringify(original)); const expired = browserEntryService(config, () => now + 60_000);
    await assert.rejects(expired.restore(), (error: unknown) => error instanceof EntryFailure && error.kind === 'expired'); assert.equal(store.getItem(key), null);
  } finally { server.close(); server.closeAllConnections(); if (before) Object.defineProperty(globalThis, 'window', before); else Reflect.deleteProperty(globalThis, 'window'); }
});
