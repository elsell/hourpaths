import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient, type HomePreferencesUpdate } from './index';

test('Home preference client uses the authenticated self-scoped idempotent contract', async () => {
  let captured: Request | undefined;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (request, init) => {
    captured = new Request(request, init);
    return Response.json({ data: { orderMethod: 'manual', revision: 1, pinnedPathIds: ['path-2'], manualPathIds: ['path-2', 'path-1'], updatedAt: '2026-08-04T12:00:00Z' } });
  };
  try {
    const body: HomePreferencesUpdate = { expectedRevision: 0, orderMethod: 'manual', pinnedPathIds: ['path-2'], manualPathIds: ['path-2', 'path-1'] };
    await createSessionApiClient('https://api.example.test', () => 'session-token').updateHomePreferences(body, 'home-order-key-0001');
  } finally { globalThis.fetch = originalFetch; }
  assert.equal(new URL(captured!.url).pathname, '/v1/me/home-preferences');
  assert.equal(captured!.method, 'PUT');
  assert.equal(captured!.headers.get('authorization'), 'Bearer session-token');
  assert.equal(captured!.headers.get('idempotency-key'), 'home-order-key-0001');
  assert.deepEqual(await captured!.clone().json(), { expectedRevision: 0, orderMethod: 'manual', pinnedPathIds: ['path-2'], manualPathIds: ['path-2', 'path-1'] });
});

test('Home can refresh one authoritative Path projection after recorded activity changes', async () => {
  let captured: Request | undefined;
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (request, init) => {
    captured = new Request(request, init);
    return Response.json({ data: { id: 'path-1', name: 'Reading', visibility: 'private' } });
  };
  try {
    await createSessionApiClient('https://api.example.test', () => 'session-token').path('path-1');
  } finally { globalThis.fetch = originalFetch; }
  assert.equal(new URL(captured!.url).pathname, '/v1/paths/path-1');
  assert.equal(captured!.method, 'GET');
  assert.equal(captured!.headers.get('authorization'), 'Bearer session-token');
});
