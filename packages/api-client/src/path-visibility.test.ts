import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient } from './index';

test('setPathVisibility sends the reviewed transition and caller-owned replay key', async (context) => {
  const originalFetch = globalThis.fetch;
  let request: Request | undefined;
  globalThis.fetch = async (input, init) => {
    request = new Request(input, init);
    return Response.json({ data: { id: 'path-1', name: 'Read', visibility: 'followers', capabilities: { trackTime: true, renamePath: true, inviteMembers: true, manageMembers: true, manageGoals: true, manageLifecycle: true, manageVisibility: true, transferOwnership: true, leavePath: false } } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });
  await createSessionApiClient('https://api.example.test', () => 'session').setPathVisibility('path-1', { confirmed: true, expectedVisibility: 'private', visibility: 'followers' }, 'visibility-key-0001');
  assert.equal(request?.method, 'PUT');
  assert.equal(new URL(request!.url).pathname, '/v1/paths/path-1/visibility');
  assert.equal(request?.headers.get('idempotency-key'), 'visibility-key-0001');
  assert.deepEqual(await request?.json(), { confirmed: true, expectedVisibility: 'private', visibility: 'followers' });
});
