import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient, generatedResponse, type ConfiguredTimeZoneUpdate } from './index.js';

test('configured time-zone operations use generated authenticated contracts and atomic mutation identity', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    requests.push(request);
    return Response.json({ data: {
      timeZone: request.method === 'GET' ? 'America/New_York' : 'Europe/Paris',
      effectiveAt: '2026-08-03T14:30:00Z',
      changed: request.method === 'PUT',
    } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const client = createSessionApiClient('https://api.example.test', () => 'application-token');
  const update: ConfiguredTimeZoneUpdate = {
    reviewedTimeZone: 'America/New_York', proposedTimeZone: 'Europe/Paris', confirmed: true,
  };
  const current = await client.configuredTimeZone();
  const changed = await client.updateConfiguredTimeZone(update, 'time-zone-key-0001');

  assert.deepEqual(requests.map(({ method, url }) => [method, new URL(url).pathname]), [
    ['GET', '/v1/me/time-zone'], ['PUT', '/v1/me/time-zone'],
  ]);
  assert.equal(requests[0]?.headers.get('authorization'), 'Bearer application-token');
  assert.equal(requests[1]?.headers.get('idempotency-key'), 'time-zone-key-0001');
  assert.deepEqual(await requests[1]?.json(), update);
  assert.equal((await generatedResponse(current).json())?.data.changed, false);
  assert.equal((await generatedResponse(changed).json())?.data.timeZone, 'Europe/Paris');
});
