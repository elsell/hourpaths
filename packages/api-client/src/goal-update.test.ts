import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient, generatedResponse } from './index.js';

test('goal update uses the dedicated generated route with current credentials and replay key', async (context) => {
  const originalFetch = globalThis.fetch;
  let captured: Request | undefined;
  globalThis.fetch = async (input, init) => {
    captured = new Request(input, init);
    return Response.json({
      data: {
        path: {
          id: 'path-1',
          name: 'Read',
          visibility: 'private',
          intervalGoal: {
            targetSeconds: 30,
            recurrence: 'daily',
            alignment: { hour: 6 },
          },
        },
        accumulatedSeconds: 45,
        intervalProgress: { accumulatedSeconds: 45, targetSeconds: 30 },
      },
    });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const body = {
    confirmed: true as const,
    expectedGoals: {
      intervalGoal: {
        targetSeconds: 60,
        recurrence: 'weekly' as const,
        alignment: { isoWeekday: 1 },
      },
      overallTarget: { targetSeconds: 3_600 },
    },
    intervalGoal: {
      targetSeconds: 30,
      recurrence: 'daily' as const,
      alignment: { hour: 6 },
    },
  };
  const result = await createSessionApiClient('https://api.example.test', () => 'current-token')
    .updatePathGoals('path-1', body, 'path-goals-key-0001');

  assert.ok(captured);
  assert.equal(captured.method, 'PUT');
  assert.equal(new URL(captured.url).pathname, '/v1/paths/path-1/goals');
  assert.equal(captured.headers.get('authorization'), 'Bearer current-token');
  assert.equal(captured.headers.get('idempotency-key'), 'path-goals-key-0001');
  assert.deepEqual(await captured.json(), body);
  assert.deepEqual(await generatedResponse(result).json(), {
    data: {
      path: {
        id: 'path-1',
        name: 'Read',
        visibility: 'private',
        intervalGoal: {
          targetSeconds: 30,
          recurrence: 'daily',
          alignment: { hour: 6 },
        },
      },
      accumulatedSeconds: 45,
      intervalProgress: { accumulatedSeconds: 45, targetSeconds: 30 },
    },
  });
});

test('goal update binds removal to the reviewed goal configuration', async (context) => {
  const originalFetch = globalThis.fetch;
  let captured: Request | undefined;
  globalThis.fetch = async (input, init) => {
    captured = new Request(input, init);
    return Response.json({
      data: {
        path: { id: 'path-1', name: 'Read', visibility: 'private' },
        accumulatedSeconds: 45,
      },
    });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  await createSessionApiClient('https://api.example.test', () => 'current-token')
    .updatePathGoals('path-1', {
      confirmed: true,
      expectedGoals: { overallTarget: { targetSeconds: 45 } },
    }, 'path-goals-key-0002');

  assert.ok(captured);
  assert.deepEqual(await captured.json(), {
    confirmed: true,
    expectedGoals: { overallTarget: { targetSeconds: 45 } },
  });
});
