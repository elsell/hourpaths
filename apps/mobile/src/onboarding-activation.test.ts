import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionOperationOwner, type SessionFailure } from '@hourpaths/client-core';
import {
  completeMobileOnboardingActivation,
  type MobileOnboardingActivationResponse,
} from './onboarding-activation';

const untrustedServerDetail = ['untrusted', 'server prose'].join(' ');

const activeResponse: MobileOnboardingActivationResponse = {
  ok: true,
  status: 200,
  json: async () => ({
    data: {
      token: 'active-token',
      expiresAt: '2026-07-22T12:00:00Z',
      nextAction: 'home',
    },
  }),
};

test('activation persists and adopts the already-issued active credential exactly once', async () => {
  const events: string[] = [];
  const outcome = await completeMobileOnboardingActivation({
    request: async () => activeResponse,
    current: () => true,
    persist: async (credential) => { events.push(`persist:${credential.token}`); },
    adopt: async (credential) => { events.push(`adopt:${credential.token}`); },
    handleAdoptionFailure: async () => assert.fail('Home adoption must succeed'),
    refreshReview: async () => assert.fail('current review must remain valid'),
    revokeSuperseded: async () => assert.fail('current activation must not be revoked'),
  });

  assert.deepEqual(outcome, { kind: 'activated' });
  assert.deepEqual(events, ['persist:active-token', 'adopt:active-token']);
});

test('policy changes refresh the server review and keep onboarding actionable', async () => {
  const refreshed = { policyReviewToken: 'fresh-review' };
  const outcome = await completeMobileOnboardingActivation({
    request: async () => ({
      ok: false,
      status: 409,
      problem: { code: 'policy_set_changed', detail: untrustedServerDetail },
      json: async () => undefined,
    }),
    current: () => true,
    persist: async () => assert.fail('a rejected activation has no credential'),
    adopt: async () => assert.fail('a rejected activation cannot reach Home'),
    handleAdoptionFailure: async () => assert.fail('a rejected activation cannot fail Home adoption'),
    refreshReview: async () => refreshed,
    revokeSuperseded: async () => assert.fail('a rejected activation has no credential'),
  });

  assert.deepEqual(outcome, { kind: 'policy_set_changed', profile: refreshed });
});

test('an unavailable username remains a local actionable conflict without refreshing policy', async () => {
  const outcome = await completeMobileOnboardingActivation({
    request: async () => ({
      ok: false,
      status: 409,
      problem: { code: 'username_unavailable' },
      json: async () => undefined,
    }),
    current: () => true,
    persist: async () => assert.fail('a rejected activation has no credential'),
    adopt: async () => assert.fail('a rejected activation cannot reach Home'),
    handleAdoptionFailure: async () => assert.fail('a rejected activation cannot fail Home adoption'),
    refreshReview: async () => assert.fail('a username conflict does not stale policy review'),
    revokeSuperseded: async () => assert.fail('a rejected activation has no credential'),
  });

  assert.deepEqual(outcome, { kind: 'username_unavailable' });
});

test('sign-out wins over a pending activation response and revokes its late active credential', async () => {
  const owner = createSessionOperationOwner();
  const ticket = owner.issue();
  let finishRequest!: () => void;
  const pendingRequest = new Promise<void>((resolve) => { finishRequest = resolve; });
  const events: string[] = [];
  const operation = completeMobileOnboardingActivation({
    request: async () => { await pendingRequest; return activeResponse; },
    current: ticket.current,
    persist: async () => { if (ticket.current()) events.push('persist'); },
    adopt: async () => { events.push('adopt'); },
    handleAdoptionFailure: async () => assert.fail('superseded activation cannot fail adoption'),
    refreshReview: async () => assert.fail('successful activation does not refresh review'),
    revokeSuperseded: async (credential) => { events.push(`revoke:${credential.token}`); },
  });

  owner.invalidate();
  finishRequest();
  assert.deepEqual(await operation, { kind: 'superseded' });
  assert.deepEqual(events, ['revoke:active-token']);
});

test('secure-store failure revokes the issued active credential without adopting it', async () => {
  const events: string[] = [];
  await assert.rejects(
    completeMobileOnboardingActivation({
      request: async () => activeResponse,
      current: () => true,
      persist: async () => { throw { kind: 'local_storage', reason: 'malformed' } satisfies SessionFailure; },
      adopt: async () => { events.push('adopt'); },
      handleAdoptionFailure: async () => assert.fail('persistence failed before adoption'),
      refreshReview: async () => assert.fail('successful activation does not refresh review'),
      revokeSuperseded: async (credential) => { events.push(`revoke:${credential.token}`); },
    }),
    (failure: SessionFailure) => failure.kind === 'local_storage',
  );
  assert.deepEqual(events, ['revoke:active-token']);
});

for (const failure of [
  { kind: 'network' } as const,
  { kind: 'http', status: 503 } as const,
]) {
  test(`a ${failure.kind === 'network' ? 'network' : '503'} Home adoption failure is handled against the active credential`, async () => {
    const events: string[] = [];
    const outcome = await completeMobileOnboardingActivation({
      request: async () => activeResponse,
      current: () => true,
      persist: async (credential) => { events.push(`persist:${credential.token}`); },
      adopt: async () => { throw failure; },
      handleAdoptionFailure: async (cause, credential) => {
        assert.equal(cause, failure);
        events.push(`recover-profile:${credential.token}`);
        return true;
      },
      refreshReview: async () => assert.fail('activation succeeded'),
      revokeSuperseded: async () => assert.fail('retryable active credentials remain recoverable'),
    });

    assert.deepEqual(outcome, { kind: 'adoption_failed' });
    assert.deepEqual(events, ['persist:active-token', 'recover-profile:active-token']);
  });
}

test('a 401 Home adoption failure is disposed against the active credential', async () => {
  const events: string[] = [];
  const unauthorized = { kind: 'http', status: 401 } as const;
  const outcome = await completeMobileOnboardingActivation({
    request: async () => activeResponse,
    current: () => true,
    persist: async (credential) => { events.push(`persist:${credential.token}`); },
    adopt: async () => { throw unauthorized; },
    handleAdoptionFailure: async (cause, credential) => {
      assert.equal(cause, unauthorized);
      events.push(`dispose:${credential.token}`);
      return true;
    },
    refreshReview: async () => assert.fail('activation succeeded'),
    revokeSuperseded: async () => assert.fail('the failure handler owns active disposal'),
  });

  assert.deepEqual(outcome, { kind: 'adoption_failed' });
  assert.deepEqual(events, ['persist:active-token', 'dispose:active-token']);
});

test('supersession during a throwing Home adoption late-revokes the active credential', async () => {
  const owner = createSessionOperationOwner();
  const ticket = owner.issue();
  const events: string[] = [];
  const outcome = await completeMobileOnboardingActivation({
    request: async () => activeResponse,
    current: ticket.current,
    persist: async (credential) => { events.push(`persist:${credential.token}`); },
    adopt: async () => { owner.invalidate(); throw { kind: 'network' } satisfies SessionFailure; },
    handleAdoptionFailure: async () => assert.fail('superseded operations cannot recover UI state'),
    refreshReview: async () => assert.fail('activation succeeded'),
    revokeSuperseded: async (credential) => { events.push(`revoke:${credential.token}`); },
  });

  assert.deepEqual(outcome, { kind: 'superseded' });
  assert.deepEqual(events, ['persist:active-token', 'revoke:active-token']);
});

test('supersession before the failure handler takes ownership late-revokes the active credential', async () => {
  const events: string[] = [];
  const outcome = await completeMobileOnboardingActivation({
    request: async () => activeResponse,
    current: () => true,
    persist: async () => {},
    adopt: async () => { throw { kind: 'network' } satisfies SessionFailure; },
    handleAdoptionFailure: async () => false,
    refreshReview: async () => assert.fail('activation succeeded'),
    revokeSuperseded: async (credential) => { events.push(`revoke:${credential.token}`); },
  });
  assert.deepEqual(outcome, { kind: 'superseded' });
  assert.deepEqual(events, ['revoke:active-token']);
});

test('a failure-handler throw late-revokes the active credential before propagating', async () => {
  const events: string[] = [];
  const handlerFailure = new Error('handler failed');
  await assert.rejects(completeMobileOnboardingActivation({
    request: async () => activeResponse,
    current: () => true,
    persist: async () => {},
    adopt: async () => { throw { kind: 'network' } satisfies SessionFailure; },
    handleAdoptionFailure: async () => { throw handlerFailure; },
    refreshReview: async () => assert.fail('activation succeeded'),
    revokeSuperseded: async (credential) => { events.push(`revoke:${credential.token}`); },
  }), handlerFailure);
  assert.deepEqual(events, ['revoke:active-token']);
});

test('the native adapter routes adoption failures through profile recovery with the new credential', async () => {
  const { readFile } = await import('node:fs/promises');
  const app = await readFile('app/index.tsx', 'utf8');
  assert.match(app, /handleAdoptionFailure: async \(cause, credential\)/);
  assert.match(app, /handleSessionFailure\(cause, credential, 'profile', ticket\)/);
});

test('malformed success responses fail closed without persistence or adoption', async () => {
  await assert.rejects(
    completeMobileOnboardingActivation({
      request: async () => ({
        ok: true,
        status: 200,
        json: async () => ({ data: { token: 'token', expiresAt: 'bad', nextAction: 'onboarding' } }),
      }),
      current: () => true,
      persist: async () => assert.fail('malformed credentials must not persist'),
      adopt: async () => assert.fail('malformed credentials must not activate'),
      handleAdoptionFailure: async () => assert.fail('malformed credentials must not reach Home'),
      refreshReview: async () => assert.fail('malformed credentials are not stale policy'),
      revokeSuperseded: async () => assert.fail('no valid credential exists'),
    }),
    (failure: SessionFailure) => failure.kind === 'http' && failure.status === 502,
  );
});
