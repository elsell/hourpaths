import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionApiClient, generatedResponse } from './index.js';
import type { components } from './schema.js';
import type { paths } from './schema.js';

type ExchangeProblem = paths['/v1/sessions']['post']['responses']['default']['content']['application/problem+json'];
type ActivationBody = components['schemas']['OnboardingActivationInputBody'];
type ActivationProblem = paths['/v1/onboarding/activation']['post']['responses']['default']['content']['application/problem+json'];

const activationBody: ActivationBody = {
  username: 'person',
  displayName: 'Person',
  profileVisibility: 'private',
  timeZone: 'America/New_York',
  firstDayOfWeek: 1,
  atLeast16: true,
  termsAccepted: true,
  privacyAcknowledged: true,
  communityGuidelinesAccepted: true,
  policyReviewToken: 'review-token',
};

test('session operations use the generated contract and the current credential', async (context) => {
  const originalFetch = globalThis.fetch;
  const requests: Request[] = [];
  globalThis.fetch = async (input, init) => {
    const request = new Request(input, init);
    requests.push(request);
    if (request.url.endsWith('/v1/sessions')) {
      return Response.json({ data: { token: 'application-token', expiresAt: '2026-07-21T12:00:00Z', nextAction: 'onboarding' } });
    }
    if (request.url.endsWith('/v1/session/refresh')) {
      return Response.json({ data: { token: 'rotated-token', expiresAt: '2026-07-21T13:00:00Z' } });
    }
    if (request.url.endsWith('/v1/me')) {
      return Response.json({ data: { id: 'user-1', email: 'person@example.test', displayName: 'Person' } });
    }
    if (request.url.endsWith('/v1/onboarding')) {
      return Response.json({ data: { email: 'person@example.test', displayName: 'Person' } });
    }
    if (request.url.endsWith('/v1/onboarding/activation')) {
      return Response.json({ data: { token: 'active-token', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'home' } });
    }
    if (new URL(request.url).pathname === '/v1/paths' && request.method === 'GET') {
      return Response.json({ data: [], meta: {} });
    }
    if (new URL(request.url).pathname === '/v1/paths' && request.method === 'POST') {
      return Response.json({ data: { id: 'path-1', name: 'Piano', visibility: 'private' } }, { status: 201 });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/timer' && request.method === 'GET') {
      return Response.json({ data: { running: false, accumulatedSeconds: 90 } });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/timer' && request.method === 'POST') {
      return Response.json({ data: { running: true, accumulatedSeconds: 90, timer: { id: 'timer-1', pathId: 'path-1', startedAt: '2026-07-21T12:00:00Z', occurrenceTimeZone: 'America/New_York' } } });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/timer/timer-1' && request.method === 'DELETE') {
      return Response.json({ data: { running: false, accumulatedSeconds: 151, saved: true, subsecond: false } });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/activities' && request.method === 'POST') {
      return Response.json({ data: { activity: { id: 'activity-1', pathId: 'path-1', participantId: 'user-1', startedAt: '2026-07-21T13:00:00Z', endedAt: '2026-07-21T14:00:00Z', occurrenceTimeZone: 'Etc/UTC', durationSeconds: 3600, createdAt: '2026-07-21T14:00:00Z', updatedAt: '2026-07-21T14:00:00Z' }, version: 1, accumulatedSeconds: 3600 } }, { status: 201 });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/activities' && request.method === 'GET') {
      return Response.json({ data: [{ activity: { id: 'activity-1' }, version: 2 }], meta: {} });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/activities/manual-defaults' && request.method === 'GET') {
      return Response.json({ data: { localDate: '2026-07-21', localStartTime: '14:00:00', timeZone: 'Etc/UTC', currentInstant: '2026-07-21T14:00:00Z' } });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/activities/activity-1' && request.method === 'PUT') {
      return Response.json({ data: { activity: { id: 'activity-1' }, version: 2, accumulatedSeconds: 1800 } });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/activities/activity-1' && request.method === 'GET') {
      return Response.json({ data: { activity: { id: 'activity-1' }, version: 2 } });
    }
    if (new URL(request.url).pathname === '/v1/paths/path-1/activities/activity-1/revisions') {
      return Response.json({ data: [{ id: 'activity-1', version: 1 }], meta: {} });
    }
    return new Response(null, { status: 204 });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  let token: string | null = null;
  const api = createSessionApiClient('https://api.example.test', () => token);
  const exchange = await api.exchange('provider-id-token');
  token = 'application-token';
  const refresh = await api.refresh();
  token = 'rotated-token';
  const profile = await api.profile();
  const onboarding = await api.onboarding();
  const activation = await api.activateOnboarding(activationBody);
  const paths = await api.paths();
  const pathBody = {
    name: 'Piano',
    intervalGoal: {
      targetSeconds: 1800,
      recurrence: 'monthly' as const,
      alignment: { day: 15 },
    },
    overallTarget: { targetSeconds: 360000 },
  };
  const createdPath = await api.createPath(pathBody, '0123456789abcdef');
  const currentTimer = await api.currentTimer('path-1');
  const startedTimer = await api.startTimer('path-1', 'start-key-0000001');
  const stoppedTimer = await api.stopTimer('path-1', 'timer-1', 'stop-key-00000001');
  const manualBody = { localDate: '2026-07-21', localStartTime: '13:00:00', durationSeconds: 3600, note: 'practice' };
  const manualDefaults = await api.manualActivityDefaults('path-1');
  const createdActivity = await api.createManualActivity('path-1', manualBody, 'manual-create-key');
  const updatedActivity = await api.updateActivity('path-1', 'activity-1', { ...manualBody, durationSeconds: 1800 }, 'manual-update-key');
  const activity = await api.activity('path-1', 'activity-1');
  const activities = await api.activities('path-1', 'activity-page-cursor', 'participant-2');
  const revisions = await api.activityRevisions('path-1', 'activity-1', 'revision-page-cursor');
  const decline = await api.declineDuplicateEmailRecovery();
  await api.revoke();

  assert.deepEqual(requests.map(({ method, url }) => [method, new URL(url).pathname]), [
    ['POST', '/v1/sessions'],
    ['POST', '/v1/session/refresh'],
    ['GET', '/v1/me'],
    ['GET', '/v1/onboarding'],
    ['POST', '/v1/onboarding/activation'],
    ['GET', '/v1/paths'],
    ['POST', '/v1/paths'],
    ['GET', '/v1/paths/path-1/timer'],
    ['POST', '/v1/paths/path-1/timer'],
    ['DELETE', '/v1/paths/path-1/timer/timer-1'],
    ['GET', '/v1/paths/path-1/activities/manual-defaults'],
    ['POST', '/v1/paths/path-1/activities'],
    ['PUT', '/v1/paths/path-1/activities/activity-1'],
    ['GET', '/v1/paths/path-1/activities/activity-1'],
    ['GET', '/v1/paths/path-1/activities'],
    ['GET', '/v1/paths/path-1/activities/activity-1/revisions'],
    ['POST', '/v1/onboarding/duplicate-email-recovery/decline'],
    ['DELETE', '/v1/session'],
  ]);
  assert.equal(new URL(requests[5]!.url).searchParams.get('limit'), '25');
  assert.equal(new URL(requests[14]!.url).searchParams.get('limit'), '25');
  assert.equal(new URL(requests[14]!.url).searchParams.get('cursor'), 'activity-page-cursor');
  assert.equal(new URL(requests[14]!.url).searchParams.get('participantId'), 'participant-2');
  assert.equal(new URL(requests[15]!.url).searchParams.get('limit'), '25');
  assert.equal(new URL(requests[15]!.url).searchParams.get('cursor'), 'revision-page-cursor');
  assert.deepEqual(await requests[0]?.json(), { identityToken: 'provider-id-token' });
  assert.equal(requests[0]?.headers.get('authorization'), null);
  assert.equal(requests[1]?.headers.get('authorization'), 'Bearer application-token');
  assert.equal(requests[2]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[3]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[4]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[5]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[6]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[6]?.headers.get('idempotency-key'), '0123456789abcdef');
  assert.deepEqual(await requests[6]?.json(), pathBody);
  assert.equal(requests[7]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[8]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[8]?.headers.get('idempotency-key'), 'start-key-0000001');
  assert.equal(requests[9]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[9]?.headers.get('idempotency-key'), 'stop-key-00000001');
  assert.equal(requests[11]?.headers.get('idempotency-key'), 'manual-create-key');
  assert.deepEqual(await requests[11]?.json(), manualBody);
  assert.equal(requests[12]?.headers.get('idempotency-key'), 'manual-update-key');
  assert.equal(requests[16]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.equal(requests[17]?.headers.get('authorization'), 'Bearer rotated-token');
  assert.deepEqual(await requests[4]?.json(), activationBody);
  assert.deepEqual(await generatedResponse(exchange).json(), { data: { token: 'application-token', expiresAt: '2026-07-21T12:00:00Z', nextAction: 'onboarding' } });
  assert.deepEqual(await generatedResponse(refresh).json(), { data: { token: 'rotated-token', expiresAt: '2026-07-21T13:00:00Z' } });
  assert.deepEqual(await generatedResponse(profile).json(), { data: { id: 'user-1', email: 'person@example.test', displayName: 'Person' } });
  assert.deepEqual(await generatedResponse(onboarding).json(), { data: { email: 'person@example.test', displayName: 'Person' } });
  assert.deepEqual(await generatedResponse(activation).json(), { data: { token: 'active-token', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'home' } });
  assert.deepEqual(await generatedResponse(paths).json(), { data: [], meta: {} });
  assert.deepEqual(await generatedResponse(createdPath).json(), { data: { id: 'path-1', name: 'Piano', visibility: 'private' } });
  assert.deepEqual(await generatedResponse(currentTimer).json(), { data: { running: false, accumulatedSeconds: 90 } });
  assert.equal((await generatedResponse(startedTimer).json())?.data.running, true);
  assert.deepEqual(await generatedResponse(stoppedTimer).json(), { data: { running: false, accumulatedSeconds: 151, saved: true, subsecond: false } });
  assert.equal((await generatedResponse(manualDefaults).json())?.data.timeZone, 'Etc/UTC');
  assert.equal((await generatedResponse(manualDefaults).json())?.data.currentInstant, '2026-07-21T14:00:00Z');
  assert.equal((await generatedResponse(createdActivity).json())?.data.version, 1);
  assert.equal((await generatedResponse(updatedActivity).json())?.data.version, 2);
  assert.equal((await generatedResponse(activity).json())?.data.version, 2);
  assert.equal((await generatedResponse(activities).json())?.data[0]?.version, 2);
  assert.equal((await generatedResponse(revisions).json())?.data[0]?.version, 1);
  assert.equal(generatedResponse(decline).status, 204);
});

test('activation preserves stable username and policy-set problem codes from the generated contract', async (context) => {
  const originalFetch = globalThis.fetch;
  const problems: ActivationProblem[] = [
    { type: 'about:blank', code: 'username_unavailable' },
    { type: 'about:blank', code: 'policy_set_changed' },
  ];
  let attempt = 0;
  globalThis.fetch = async () => new Response(JSON.stringify(problems[attempt++]), {
    status: 409,
    headers: { 'Content-Type': 'application/problem+json' },
  });
  context.after(() => { globalThis.fetch = originalFetch; });

  const api = createSessionApiClient('https://api.example.test', () => 'onboarding-token');
  for (const problem of problems) {
    const response = generatedResponse(await api.activateOnboarding(activationBody));
    assert.equal(response.status, 409);
    assert.deepEqual(response.problem, problem);
  }
});

test('generated problem bodies cross the adapter without trusting their text', async (context) => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => new Response(JSON.stringify({
    code: 'authorization_pending',
    title: 'Hostile title',
    detail: '<script>do not render me</script>',
  }), { status: 503, headers: { 'Content-Type': 'application/problem+json' } });
  context.after(() => { globalThis.fetch = originalFetch; });

  const response = generatedResponse(await createSessionApiClient('https://api.example.test', () => 'token').refresh());

  assert.equal(response.ok, false);
  assert.equal(response.status, 503);
  assert.deepEqual(response.problem, {
    code: 'authorization_pending',
    title: 'Hostile title',
    detail: '<script>do not render me</script>',
  });
});

test('session exchange exposes an actual generated authentication problem code', async (context) => {
  const generatedProblem: ExchangeProblem = { type: 'about:blank', code: 'invalid_credential' };
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => new Response(JSON.stringify(generatedProblem), {
    status: 401,
    headers: { 'Content-Type': 'application/problem+json' },
  });
  context.after(() => { globalThis.fetch = originalFetch; });

  const response = generatedResponse(await createSessionApiClient('https://api.example.test', () => null).exchange('rejected-token'));
  assert.equal(response.status, 401);
  assert.deepEqual(response.problem, generatedProblem);
});

test('activity deletion uses the generated route with current credentials and caller-owned replay key', async (context) => {
  const originalFetch = globalThis.fetch;
  let captured: Request | undefined;
  globalThis.fetch = async (input, init) => {
    captured = new Request(input, init);
    return Response.json({ data: { accumulatedSeconds: 42, sessionCount: 3, unreadNotificationCount: 4, removedFeedEventIds: ['achievement:a', 'practice:activity-1'] } });
  };
  context.after(() => { globalThis.fetch = originalFetch; });

  const result = await createSessionApiClient('https://api.example.test', () => 'current-token')
    .deleteActivity('path-1', 'activity-1', 'activity-delete-key');

  assert.ok(captured);
  assert.equal(captured.method, 'DELETE');
  assert.equal(new URL(captured.url).pathname, '/v1/paths/path-1/activities/activity-1');
  assert.equal(captured.headers.get('authorization'), 'Bearer current-token');
  assert.equal(captured.headers.get('idempotency-key'), 'activity-delete-key');
  assert.deepEqual(await generatedResponse(result).json(), { data: { accumulatedSeconds: 42, sessionCount: 3, unreadNotificationCount: 4, removedFeedEventIds: ['achievement:a', 'practice:activity-1'] } });
});
