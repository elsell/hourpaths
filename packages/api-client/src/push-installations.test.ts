import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient, generatedResponse, type PushInstallationInput } from './index.js';

test('push installation and notification resolution use generated authenticated routes', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  const tokens = ['register-token', 'resolve-token', 'delete-token'] as const;
  let tokenIndex = 0;
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    requests.push(request);
    if (request.method === 'GET') {
      return Response.json({
        data: {
          id: 'notification-1',
          type: 'path_invitation_received',
          presentation: 'actionable',
          read: false,
          createdAt: '2026-07-23T12:00:00Z',
          actor: { userId: 'owner', username: 'Owner.One', displayName: 'Owner' },
          pathId: 'path-1',
          pathName: 'Piano',
          invitationId: 'invitation-1',
          offeredRole: 'participant',
        },
      });
    }
    return new Response(null, { status: 204 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const api = createSessionApiClient('https://api.example.test', () => tokens[tokenIndex++] ?? null);
  const body: PushInstallationInput = {
    provider: 'expo',
    platform: 'ios',
    locale: 'en',
    token: 'ExpoPushToken[token]',
  };
  const registered = await api.upsertPushInstallation('installation-1', body);
  const resolved = await api.getNotification('notification-1');
  const deleted = await api.deletePushInstallation('installation-1');

  assert.deepEqual(requests.map((request) => [
    request.method,
    new URL(request.url).pathname,
    request.headers.get('authorization'),
  ]), [
    ['PUT', '/v1/push-installations/installation-1', 'Bearer register-token'],
    ['GET', '/v1/notifications/notification-1', 'Bearer resolve-token'],
    ['DELETE', '/v1/push-installations/installation-1', 'Bearer delete-token'],
  ]);
  assert.deepEqual(await requests[0]!.json(), body);
  assert.equal(generatedResponse(registered).status, 204);
  assert.equal((await generatedResponse(resolved).json())?.data.id, 'notification-1');
  assert.equal(generatedResponse(deleted).status, 204);
});
