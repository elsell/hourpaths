import assert from 'node:assert/strict';
import test from 'node:test';
import { classifySessionFailure, createSessionOperationOwner, isSessionFailure, type SessionAccessState } from '@hourpaths/client-core';
import { restoreStoredSession } from './session-restoration';

test('cold launch restores a valid session offline with OIDC discovery unavailable and API unavailable', async () => {
  let state: SessionAccessState = 'authentication_required';
  let retainedToken = '';
  let discarded = false;
  let refreshCalled = false;
  await restoreStoredSession({
    read: async () => JSON.stringify({ token: 'stored-token', expiresAt: '2026-07-20T12:00:00Z', nextAction: 'onboarding' }),
    now: () => Date.parse('2026-07-19T12:00:00Z'),
    refreshLeadMs: 60_000,
    refresh: async (current) => { refreshCalled = true; return current; },
    expiryAdvanced: () => true,
    current: () => true,
    revokeSuperseded: async () => assert.fail('current restoration must not be revoked'),
    activate: async () => { throw { kind: 'network' } as const; },
    handleFailure: async (cause, current) => {
      const decision = classifySessionFailure(isSessionFailure(cause) ? cause : { kind: 'network' });
      state = decision.state;
      retainedToken = current.token;
      discarded = decision.discardCredential;
    },
    handleUnreadable: async () => { state = 'local_session_unreadable'; discarded = true; },
  });
  assert.equal(state, 'authenticated_offline');
  assert.equal(retainedToken, 'stored-token');
  assert.equal(discarded, false);
  assert.equal(refreshCalled, false);
});

test('cold launch restores the persisted onboarding destination without probing the Home route', async () => {
  let restoredAction = '';
  await restoreStoredSession({
    read: async () => JSON.stringify({
      token: 'stored-token',
      expiresAt: '2026-07-20T12:00:00Z',
      nextAction: 'onboarding',
    }),
    now: () => Date.parse('2026-07-19T12:00:00Z'),
    refreshLeadMs: 60_000,
    refresh: async () => assert.fail('fresh session must not refresh'),
    expiryAdvanced: () => true,
    current: () => true,
    revokeSuperseded: async () => assert.fail('current restoration must not be revoked'),
    activate: async (session) => { restoredAction = session.nextAction; },
    handleFailure: async () => assert.fail('valid onboarding storage must restore'),
    handleUnreadable: async () => assert.fail('valid onboarding storage must be readable'),
  });
  assert.equal(restoredAction, 'onboarding');
});

test('near-expiry restricted sessions remain nonrenewable instead of probing the active refresh route', async () => {
  for (const nextAction of ['onboarding', 'duplicate_email_recovery'] as const) {
    let refreshCalls = 0;
    let renewable: boolean | undefined;
    await restoreStoredSession({
      read: async () => JSON.stringify({
        token: 'restricted-token',
        expiresAt: '2026-07-20T12:00:30Z',
        nextAction,
      }),
      now: () => Date.parse('2026-07-20T12:00:00Z'),
      refreshLeadMs: 60_000,
      refresh: async (current) => { refreshCalls += 1; return current; },
      expiryAdvanced: () => true,
      current: () => true,
      revokeSuperseded: async () => assert.fail('current restricted session must not be revoked'),
      activate: async (_session, canRenew) => { renewable = canRenew; },
      handleFailure: async () => assert.fail('valid restricted session must activate'),
      handleUnreadable: async () => assert.fail('valid restricted session must be readable'),
    });
    assert.equal(refreshCalls, 0, nextAction);
    assert.equal(renewable, false, nextAction);
  }
});

test('cold launch rejects a credential whose destination is absent or unknown', async () => {
  for (const stored of [
    { token: 'stored-token', expiresAt: '2026-07-20T12:00:00Z' },
    { token: 'stored-token', expiresAt: '2026-07-20T12:00:00Z', nextAction: '/v1/me' },
  ]) {
    let unreadable = false;
    await restoreStoredSession({
      read: async () => JSON.stringify(stored),
      now: () => Date.parse('2026-07-19T12:00:00Z'),
      refreshLeadMs: 60_000,
      refresh: async (current) => current,
      expiryAdvanced: () => true,
      current: () => true,
      revokeSuperseded: async () => assert.fail('invalid storage has no credential to revoke'),
      activate: async () => assert.fail('unknown destinations must fail closed'),
      handleFailure: async () => assert.fail('invalid storage is not a request failure'),
      handleUnreadable: async () => { unreadable = true; },
    });
    assert.equal(unreadable, true, JSON.stringify(stored));
  }
});

test('cold launch discards non-string secure-storage credentials as unreadable', async () => {
  for (const stored of [
    { token: 123, expiresAt: '2026-07-20T12:00:00Z' },
    { token: {}, expiresAt: '2026-07-20T12:00:00Z' },
    { token: 'stored-token', expiresAt: 123 },
  ]) {
    let unreadable = false;
    let activated = false;
    await restoreStoredSession({
      read: async () => JSON.stringify(stored),
      now: () => Date.parse('2026-07-19T12:00:00Z'),
      refreshLeadMs: 60_000,
      refresh: async (current) => current,
      expiryAdvanced: () => true,
      current: () => true,
      revokeSuperseded: async () => assert.fail('malformed restoration has no credential to revoke'),
      activate: async () => { activated = true; },
      handleFailure: async () => assert.fail('malformed storage must not reach session failure handling'),
      handleUnreadable: async () => { unreadable = true; },
    });
    assert.equal(unreadable, true, JSON.stringify(stored));
    assert.equal(activated, false, JSON.stringify(stored));
  }
});

test('authoritative disposal wins over a pending cold-start refresh', async () => {
  const owner = createSessionOperationOwner();
  const ticket = owner.issue();
  const replacement = { token: 'late-restored-token', expiresAt: '2026-07-20T13:00:00Z', nextAction: 'home' as const };
  let finishRefresh!: () => void;
  let markRefreshStarted!: () => void;
  const pendingRefresh = new Promise<void>((resolve) => { finishRefresh = resolve; });
  const refreshStarted = new Promise<void>((resolve) => { markRefreshStarted = resolve; });
  let activated = false;
  const revoked: string[] = [];
  const restoration = restoreStoredSession({
    read: async () => JSON.stringify({ token: 'stored-token', expiresAt: '2026-07-20T12:00:00Z', nextAction: 'home' }),
    now: () => Date.parse('2026-07-20T11:59:30Z'),
    refreshLeadMs: 60_000,
    refresh: async () => { markRefreshStarted(); await pendingRefresh; return replacement; },
    expiryAdvanced: () => true,
    current: ticket.current,
    revokeSuperseded: async (credential) => { revoked.push(credential.token); },
    activate: async () => { activated = true; },
    handleFailure: async () => assert.fail('supersession is not a restoration failure'),
    handleUnreadable: async () => assert.fail('valid storage must remain readable'),
  });
  await refreshStarted;
  owner.invalidate();
  finishRefresh();
  await restoration;
  assert.equal(activated, false);
  assert.deepEqual(revoked, ['late-restored-token']);
});

test('a newer owner wins over late unreadable cold-start storage', async () => {
  const owner = createSessionOperationOwner();
  const oldTicket = owner.issue();
  let finishRead!: (value: string) => void;
  const pendingRead = new Promise<string>((resolve) => { finishRead = resolve; });
  let unreadableRecovery = false;
  let activeOwner = 'old';
  const restoration = restoreStoredSession({
    read: async () => pendingRead,
    now: () => Date.parse('2026-07-20T11:59:30Z'),
    refreshLeadMs: 60_000,
    refresh: async (current) => current,
    expiryAdvanced: () => true,
    current: oldTicket.current,
    revokeSuperseded: async () => assert.fail('malformed storage has no credential to revoke'),
    activate: async () => assert.fail('malformed storage cannot activate'),
    handleFailure: async () => assert.fail('malformed storage is not a session request failure'),
    handleUnreadable: async () => { unreadableRecovery = true; activeOwner = ''; },
  });
  owner.issue();
  activeOwner = 'replacement';
  finishRead('{');
  await restoration;
  assert.equal(unreadableRecovery, false);
  assert.equal(activeOwner, 'replacement');
});
