import assert from 'node:assert/strict';
import test from 'node:test';
import { classifySessionFailure, createSessionOperationOwner, isSessionFailure, type SessionAccessState } from '@hourpaths/client-core';
import { activateExchangedSession, activatePersistedMobileSession, completeMobileSessionExchange, recoverMobileSession } from './session-exchange';

type Session = {
  token: string;
  expiresAt: string;
  nextAction: 'home' | 'onboarding' | 'duplicate_email_recovery';
};
const issued: Session = {
  token: 'issued-token',
  expiresAt: '2026-07-21T12:00:00Z',
  nextAction: 'home',
};

async function profileFailure(status: number) {
  let stored: Session | null = issued;
  let state: SessionAccessState = 'authentication_required';
  await activateExchangedSession({
    credential: issued,
    activate: async () => { throw { kind: 'http', status } as const; },
    handleFailure: async (cause, current) => {
      const failure = isSessionFailure(cause) ? cause : { kind: 'network' } as const;
      const decision = classifySessionFailure(failure);
      stored = decision.discardCredential ? null : current;
      state = decision.state;
    },
  });
  return { stored, state };
}

test('a profile 401 after exchange discards the just-issued credential', async () => {
  assert.deepEqual(await profileFailure(401), {
    stored: null,
    state: 'authentication_required',
  });
});

test('a profile 503 after exchange retains the just-issued credential offline', async () => {
  assert.deepEqual(await profileFailure(503), {
    stored: issued,
    state: 'authenticated_offline',
  });
});

test('successful exchange is acknowledged before pending profile activation', async () => {
  const events: string[] = [];
  let finishActivation!: () => void;
  const pendingActivation = new Promise<void>((resolve) => { finishActivation = resolve; });
  let persistenceCount = 0;
  let completed = false;
  const operation = completeMobileSessionExchange({
    current: () => true,
    exchange: async () => { persistenceCount += 1; events.push('exchange'); return issued; },
    acknowledge: () => { events.push('acknowledge'); },
    activate: async () => { events.push('profile'); await pendingActivation; },
    handleFailure: async () => assert.fail('profile activation must remain pending'),
    revokeSuperseded: async () => assert.fail('current exchange must not be revoked'),
  }).then(() => { completed = true; });
  await Promise.resolve();
  await Promise.resolve();
  assert.deepEqual(events, ['exchange', 'acknowledge', 'profile']);
  assert.equal(persistenceCount, 1);
  assert.equal(completed, false);
  finishActivation();
  await operation;
});

test('profile recovery reuses the authoritative credential without refreshing or persisting', async () => {
  let refreshes = 0;
  let activations = 0;
  let persistenceCount = 0;
  await recoverMobileSession({
    mode: 'profile',
    credential: issued,
    renewable: true,
    current: () => true,
    refresh: async () => { refreshes += 1; persistenceCount += 1; return issued; },
    activate: async (credential) => { activations += 1; assert.equal(credential, issued); },
    handleFailure: async () => assert.fail('profile recovery should succeed'),
    revokeSuperseded: async () => assert.fail('current profile must not be revoked'),
  });
  assert.equal(refreshes, 0);
  assert.equal(activations, 1);
  assert.equal(persistenceCount, 0);
});

test('refresh recovery persists exactly once and reports profile-only failures without rotating again', async () => {
  const replacement = { ...issued, token: 'replacement-token', expiresAt: '2026-07-21T13:00:00Z' };
  let persistenceCount = 0;
  let failureOperation = '';
  let failedCredential: Session | undefined;
  await recoverMobileSession({
    mode: 'refresh',
    credential: issued,
    renewable: true,
    current: () => true,
    refresh: async () => { persistenceCount += 1; return replacement; },
    activate: async () => { throw { kind: 'network' } as const; },
    handleFailure: async (_cause, credential, operation) => {
      failedCredential = credential;
      failureOperation = operation;
    },
    revokeSuperseded: async () => assert.fail('current refresh must not be revoked'),
  });
  assert.equal(persistenceCount, 1);
  assert.equal(failureOperation, 'profile');
  assert.equal(failedCredential, replacement);
});

test('activation adopts an already-persisted credential without another storage write', async () => {
  const events: string[] = [];
  const profile = { id: 'user-1' };
  await activatePersistedMobileSession({
    credential: issued,
    renewable: true,
    current: () => true,
    adopt: (credential, renewable) => {
      events.push(`adopt:${credential.token}:${renewable}`);
    },
    loadHome: async (credential) => {
      events.push(`profile:${credential.token}`);
      return profile;
    },
    loadOnboarding: async () => assert.fail('home activation must not load onboarding'),
    online: (loaded) => { events.push(`online:${loaded.profile.id}`); },
  });
  assert.deepEqual(events, [
    'adopt:issued-token:true',
    'profile:issued-token',
    'online:user-1',
  ]);
});

test('onboarding activation loads only the private onboarding profile', async () => {
  const onboardingSession = { ...issued, nextAction: 'onboarding' as const };
  const events: string[] = [];
  await activatePersistedMobileSession({
    credential: onboardingSession,
    renewable: true,
    current: () => true,
    adopt: (_credential, renewable) => { events.push(`adopt:${renewable}`); },
    loadHome: async () => { events.push('me'); return { id: 'must-not-load' }; },
    loadOnboarding: async () => {
      events.push('onboarding');
      return { email: 'private@example.test', displayName: 'seed' };
    },
    online: (destination) => {
      assert.deepEqual(destination, {
        kind: 'onboarding',
        profile: { email: 'private@example.test', displayName: 'seed' },
      });
      events.push('online');
    },
  });
  assert.deepEqual(events, ['adopt:false', 'onboarding', 'online']);
});

test('duplicate-email recovery cannot send its restricted credential to the Home profile route', async () => {
  const recoverySession = { ...issued, nextAction: 'duplicate_email_recovery' as const };
  let homeRequests = 0;
  await activatePersistedMobileSession({
    credential: recoverySession,
    renewable: true,
    current: () => true,
    adopt: () => {},
    loadHome: async () => { homeRequests += 1; return { id: 'must-not-load' }; },
    loadOnboarding: async () => ({ email: 'private@example.test', displayName: 'seed' }),
    online: (destination) => assert.equal(destination.kind, 'duplicate_email_recovery'),
  });
  assert.equal(homeRequests, 0);
});

test('sign-out wins over a pending refresh and revokes its late replacement', async () => {
  const owner = createSessionOperationOwner();
  const ticket = owner.issue();
  const replacement = { ...issued, token: 'late-token', expiresAt: '2026-07-21T13:00:00Z' };
  let finishRefresh!: () => void;
  const pendingRefresh = new Promise<void>((resolve) => { finishRefresh = resolve; });
  let stored: Session | null = issued;
  let active: Session | null = issued;
  const revoked: string[] = [];
  const recovery = recoverMobileSession({
    mode: 'refresh', credential: issued, renewable: true,
    current: ticket.current,
    refresh: async () => {
      await pendingRefresh;
      if (ticket.current()) stored = replacement;
      return replacement;
    },
    activate: async (credential) => { active = credential; },
    handleFailure: async () => assert.fail('supersession is not a session failure'),
    revokeSuperseded: async (credential) => { revoked.push(credential.token); },
  });
  owner.invalidate();
  stored = null;
  active = null;
  finishRefresh();
  await recovery;
  assert.equal(stored, null);
  assert.equal(active, null);
  assert.deepEqual(revoked, ['late-token']);
});

test('sign-out and replacement win over pending profile activation', async () => {
  const owner = createSessionOperationOwner();
  const oldTicket = owner.issue();
  let finishProfile!: () => void;
  const pendingProfile = new Promise<void>((resolve) => { finishProfile = resolve; });
  let activeProfile = '';
  const oldActivation = activatePersistedMobileSession({
    credential: issued,
    renewable: true,
    current: oldTicket.current,
    adopt: () => {},
    loadHome: async () => { await pendingProfile; return { id: 'old-profile' }; },
    loadOnboarding: async () => assert.fail('home activation must not load onboarding'),
    online: (destination) => { activeProfile = destination.profile.id; },
  });
  const replacementTicket = owner.issue();
  await activatePersistedMobileSession({
    credential: { ...issued, token: 'new-token' },
    renewable: true,
    current: replacementTicket.current,
    adopt: () => {},
    loadHome: async () => ({ id: 'new-profile' }),
    loadOnboarding: async () => assert.fail('home activation must not load onboarding'),
    online: (destination) => { activeProfile = destination.profile.id; },
  });
  finishProfile();
  await oldActivation;
  assert.equal(activeProfile, 'new-profile');
});

test('sign-out wins over pending exchange persistence and revokes its issued token', async () => {
  const owner = createSessionOperationOwner();
  const ticket = owner.issue();
  let finishExchange!: () => void;
  const pendingExchange = new Promise<void>((resolve) => { finishExchange = resolve; });
  let stored: Session | null = null;
  let activated = false;
  let acknowledged = false;
  const revoked: string[] = [];
  const exchange = completeMobileSessionExchange({
    current: ticket.current,
    exchange: async () => {
      await pendingExchange;
      if (ticket.current()) stored = issued;
      return issued;
    },
    acknowledge: () => { acknowledged = true; },
    activate: async () => { activated = true; },
    handleFailure: async () => assert.fail('supersession is not a session failure'),
    revokeSuperseded: async (credential) => { revoked.push(credential.token); },
  });
  owner.invalidate();
  finishExchange();
  await exchange;
  assert.equal(stored, null);
  assert.equal(activated, false);
  assert.equal(acknowledged, true);
  assert.deepEqual(revoked, ['issued-token']);
});
