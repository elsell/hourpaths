import assert from 'node:assert/strict';
import test from 'node:test';
import { createApiClient, createSessionApiClient } from './index.js';

test('authentication preserves headers serialized from the OpenAPI contract', async (context) => {
  const originalFetch = globalThis.fetch;
  let captured: Request | undefined;
  globalThis.fetch = async (input, init) => {
    captured = new Request(input, init);
    return new Response(JSON.stringify({ data: { id: 'user-1', email: 'person@example.test', displayName: 'Person' } }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const result = await createApiClient('https://api.example.test', () => 'application-session').GET('/v1/me');

  assert.equal(result.response.status, 200);
  assert.equal(captured?.headers.get('authorization'), 'Bearer application-session');
});

test('the credential provider is authoritative without discarding generated request semantics', async (context) => {
  const originalFetch = globalThis.fetch;
  const captured: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    captured.push(request);
    if (request.method === 'POST') {
      return new Response(JSON.stringify({ data: { id: 'path-1', name: 'Piano', visibility: 'private' } }), {
        status: 201,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    return new Response(JSON.stringify({ data: { id: 'user-1', email: 'person@example.test', displayName: 'Person' } }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const credentials = ['current-one', null, 'current-two'] as const;
  let requestIndex = 0;
  const client = createApiClient('https://api.example.test', () => credentials[requestIndex++]);
  const idempotencyKey = '0123456789abcdef';

  await client.POST('/v1/paths', {
    params: { header: { Authorization: 'Bearer stale-caller', 'Idempotency-Key': idempotencyKey } },
    body: { name: 'Piano', visibility: 'private' },
  });
  await client.GET('/v1/me', { params: { header: { Authorization: 'Bearer stale-caller' } } });
  await client.GET('/v1/me');

  assert.equal(requestIndex, 3, 'the provider must be consulted for every request');
  assert.equal(captured.length, 3);
  assert.equal(captured[0]?.method, 'POST');
  assert.equal(captured[0]?.headers.get('authorization'), 'Bearer current-one');
  assert.equal(captured[0]?.headers.get('idempotency-key'), idempotencyKey);
  assert.equal(captured[0]?.headers.get('content-type'), 'application/json');
  assert.deepEqual(await captured[0]?.json(), { name: 'Piano', visibility: 'private' });
  assert.equal(captured[1]?.headers.get('authorization'), null, 'an absent credential must remove a caller bearer');
  assert.equal(captured[2]?.headers.get('authorization'), 'Bearer current-two');
});

test('credential lookup failure performs no network request', async (context) => {
  const originalFetch = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => {
    calls += 1;
    return new Response(null, { status: 204 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const unavailable = new Error('secure storage unavailable');
  await assert.rejects(
    createApiClient('https://api.example.test', async () => { throw unavailable; }).GET('/v1/me'),
    unavailable,
  );
  assert.equal(calls, 0);
});

test('interaction settings and exact feed event wrappers preserve typed boundary inputs', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  globalThis.fetch = async (input, init) => {
    requests.push(new Request(input, init));
    return new Response(JSON.stringify({ data: { commentsEnabled: false, reactionsEnabled: true } }), { status: 200, headers: { 'Content-Type': 'application/json' } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });
  const client = createSessionApiClient('https://api.example.test', () => 'token');
  await client.getInteractionSettings();
  await client.updateInteractionSettings({ commentsEnabled: false, reactionsEnabled: true }, 'interaction-key-01');
  await client.getPracticeFeedEvent('achievement:old');
  assert.equal(new URL(requests[0]!.url).pathname, '/v1/me/interaction-settings');
  assert.equal(requests[1]!.method, 'PUT');
  assert.equal(requests[1]!.headers.get('idempotency-key'), 'interaction-key-01');
  assert.deepEqual(await requests[1]!.json(), { commentsEnabled: false, reactionsEnabled: true });
  assert.equal(new URL(requests[2]!.url).pathname, '/v1/social/feed/achievement%3Aold');
});
