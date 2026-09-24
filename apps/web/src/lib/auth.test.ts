import assert from 'node:assert/strict';
import test from 'node:test';
import { createSessionOperationOwner } from '@hourpaths/client-core';
import { activateApplicationSession, applicationDestination, applicationSessionExpired, clearApplicationSession, declineApplicationRecovery, exchangeSessionFailureMessage, persistOwnedApplicationSession, readApplicationSession, revokeApplicationSession, revokeSupersededApplicationSession, webSessionFailure } from './auth';

const unreadableSessionFailure = { kind: 'local_storage', reason: 'malformed' } as const;
const runtimeConfig = {
  environment: 'development',
  apiURL: 'http://api.example.test',
  oidcIssuer: 'http://identity.example.test',
  oidcClientId: 'hourpaths-web',
} as const;

test('session exchange actions map only to closed application destinations', () => {
  assert.equal(applicationDestination('home'), '/');
  assert.equal(applicationDestination('onboarding'), '/onboarding');
  assert.equal(applicationDestination('duplicate_email_recovery'), '/account-recovery');
});

function storageWith(raw: string | null) {
  let current = raw;
  return {
    storage: {
      getItem: () => current,
      removeItem: () => { current = null; },
    },
    current: () => current,
  };
}

test('invalid browser session records are discarded as unreadable', () => {
  for (const raw of [
    '{',
    '{}',
    'null',
    JSON.stringify({ token: null, expiresAt: '2026-07-21T12:00:00Z' }),
    JSON.stringify({ token: 123, expiresAt: '2026-07-21T12:00:00Z' }),
    JSON.stringify({ token: {}, expiresAt: '2026-07-21T12:00:00Z' }),
    JSON.stringify({ token: 'token', expiresAt: null }),
    JSON.stringify({ token: 'token', expiresAt: 123 }),
  ]) {
    const fixture = storageWith(raw);
    assert.throws(
      () => readApplicationSession(fixture.storage),
      (failure: unknown) => {
        assert.deepEqual(failure, unreadableSessionFailure);
        return true;
      },
      raw,
    );
    assert.equal(fixture.current(), null, `${raw} must be removed`);
  }
});

test('an absent browser session remains an ordinary signed-out state', () => {
  const fixture = storageWith(null);
  assert.equal(readApplicationSession(fixture.storage), null);
});

test('browser storage access failures surface as unreadable local sessions', () => {
  const securityError = new Error('browser storage is unavailable');
  securityError.name = 'SecurityError';
  assert.throws(
    () => readApplicationSession({
      getItem: () => { throw securityError; },
      removeItem: () => { throw securityError; },
    }),
    (failure: unknown) => {
      assert.deepEqual(failure, unreadableSessionFailure);
      return true;
    },
  );
});

test('failed malformed-session removal preserves the typed recovery failure', () => {
  const securityError = new Error('browser storage removal is unavailable');
  securityError.name = 'SecurityError';
  assert.throws(
    () => readApplicationSession({
      getItem: () => '{',
      removeItem: () => { throw securityError; },
    }),
    (failure: unknown) => {
      assert.deepEqual(failure, unreadableSessionFailure);
      return true;
    },
  );
});

test('presentation cleanup does not rethrow an unavailable storage boundary', () => {
  const securityError = new Error('browser storage removal is unavailable');
  securityError.name = 'SecurityError';
  assert.doesNotThrow(() => clearApplicationSession({
    removeItem: () => { throw securityError; },
  }));
});

test('sign out resolves locally without storage reads or remote revocation when no session is owned', async () => {
  const securityError = new Error('browser storage is unavailable');
  securityError.name = 'SecurityError';
  let revocations = 0;
  await assert.doesNotReject(() => revokeApplicationSession(
    runtimeConfig,
    null,
    { removeItem: () => { throw securityError; } },
    async () => { revocations += 1; },
  ));
  assert.equal(revocations, 0);
});

test('sign out resolves after local disposal without waiting for remote revocation', async () => {
  let removals = 0;
  let revocations = 0;
  let resolved = false;
  const pendingRevocation = new Promise<void>(() => {});
  void revokeApplicationSession(
    runtimeConfig,
    { token: 'session-token', expiresAt: '2026-07-21T12:00:00Z' },
    { removeItem: () => { removals += 1; } },
    async () => { revocations += 1; await pendingRevocation; },
  ).then(() => { resolved = true; });
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(removals, 1);
  assert.equal(revocations, 1);
  assert.equal(resolved, true);
});

test('the web presentation surfaces unreadable local session state', () => {
  assert.deepEqual(webSessionFailure({ kind: 'local_storage', reason: 'malformed' }), {
    accessState: 'local_session_unreadable',
    discardCredential: true,
    retryable: false,
    message: 'errors.localSessionUnreadable',
  });
});

test('first-exchange 401 keeps disposal status authoritative but presents its semantic identity code', () => {
  const failure = { kind: 'http', status: 401, code: 'invalid_credential' } as const;
  assert.deepEqual(webSessionFailure(failure), {
    accessState: 'authentication_required',
    discardCredential: true,
    retryable: false,
    message: 'errors.sessionExpired',
  });
  assert.equal(exchangeSessionFailureMessage(failure), 'errors.authenticationRejected');
});

test('late refresh and exchange persistence cannot survive browser sign-out', async () => {
  for (const operation of ['refresh', 'exchange']) {
    const owner = createSessionOperationOwner();
    const ticket = owner.issue();
    let stored = 'old-token';
    let ui = 'old-token';
    const revoked: string[] = [];
    let finish!: () => void;
    const pending = new Promise<void>((resolve) => { finish = resolve; });
    const completion = (async () => {
      await pending;
      persistOwnedApplicationSession(
        { token: `${operation}-replacement`, expiresAt: '2026-07-21T13:00:00Z' },
        ticket,
        { setItem: (_key, value) => { stored = JSON.parse(value).token; } },
      );
      if (ticket.current()) ui = `${operation}-replacement`;
      else revokeSupersededApplicationSession(
        runtimeConfig,
        { token: `${operation}-replacement`, expiresAt: '2026-07-21T13:00:00Z' },
        async (_config, token) => { revoked.push(token); },
      );
    })();
    owner.invalidate();
    stored = '';
    ui = '';
    finish();
    await completion;
    assert.equal(stored, '', operation);
    assert.equal(ui, '', operation);
    assert.deepEqual(revoked, [`${operation}-replacement`], operation);
  }
});

test('late browser profile response cannot overwrite a replacement owner', async () => {
  const owner = createSessionOperationOwner();
  const oldTicket = owner.issue();
  let profile = 'old';
  let finish!: () => void;
  const pending = new Promise<void>((resolve) => { finish = resolve; });
  const oldProfile = (async () => {
    await pending;
    if (oldTicket.current()) profile = 'late-old';
  })();
  owner.issue();
  profile = 'replacement';
  finish();
  await oldProfile;
  assert.equal(profile, 'replacement');
});

test('declining recovery persists the onboarding transition for the current browser owner', () => {
  const owner = createSessionOperationOwner();
  const writes: Array<[string, string]> = [];
  const result = declineApplicationRecovery(
    { token: 'recovery-token', expiresAt: '2026-07-21T13:00:00Z', nextAction: 'duplicate_email_recovery' },
    owner.issue(),
    { setItem: (key, value) => { writes.push([key, value]); } },
  );
  assert.deepEqual(result, {
    token: 'recovery-token',
    expiresAt: '2026-07-21T13:00:00Z',
    nextAction: 'onboarding',
  });
  assert.deepEqual(JSON.parse(writes[0][1]), result);
});

test('declining recovery cannot persist for a superseded browser owner', () => {
  const owner = createSessionOperationOwner();
  const stale = owner.issue();
  let writes = 0;
  owner.invalidate();
  assert.equal(declineApplicationRecovery(
    { token: 'recovery-token', expiresAt: '2026-07-21T13:00:00Z', nextAction: 'duplicate_email_recovery' },
    stale,
    { setItem: () => { writes += 1; } },
  ), null);
  assert.equal(writes, 0);
});

test('declining recovery types browser persistence failures as unreadable local storage', () => {
  const owner = createSessionOperationOwner();
  const storageError = new Error();
  storageError.name = 'SecurityError';
  assert.throws(
    () => declineApplicationRecovery(
      { token: 'recovery-token', expiresAt: '2026-07-21T13:00:00Z', nextAction: 'duplicate_email_recovery' },
      owner.issue(),
      { setItem: () => { throw storageError; } },
    ),
    (failure: unknown) => {
      assert.deepEqual(failure, unreadableSessionFailure);
      return true;
    },
  );
});

test('restricted browser session expiry is determined from an injected instant', () => {
  const session = {
    token: 'recovery-token',
    expiresAt: '2026-07-21T13:00:00Z',
    nextAction: 'duplicate_email_recovery' as const,
  };
  assert.equal(applicationSessionExpired(session, Date.parse('2026-07-21T12:59:59.999Z')), false);
  assert.equal(applicationSessionExpired(session, Date.parse('2026-07-21T13:00:00Z')), true);
});

test('activation adopts the already-issued home credential with one owned persistence write', async () => {
  const owner = createSessionOperationOwner();
  const writes: Array<[string, string]> = [];
  const result = await activateApplicationSession(
    { token: 'onboarding-token', expiresAt: '2026-07-21T13:00:00Z', nextAction: 'onboarding' },
    async () => ({
      ok: true,
      status: 200,
      json: async () => ({ data: { token: 'active-token', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'home' } }),
    }),
    owner.issue(),
    { setItem: (key, value) => { writes.push([key, value]); } },
  );
  assert.deepEqual(result, {
    session: { token: 'active-token', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'home' },
    adopted: true,
  });
  assert.equal(writes.length, 1);
  assert.deepEqual(JSON.parse(writes[0][1]), result.session);
});

test('activation never persists or adopts a credential after sign-out wins ownership', async () => {
  const owner = createSessionOperationOwner();
  const ticket = owner.issue();
  let writes = 0;
  owner.invalidate();
  const result = await activateApplicationSession(
    { token: 'onboarding-token', expiresAt: '2026-07-21T13:00:00Z', nextAction: 'onboarding' },
    async () => ({
      ok: true,
      status: 200,
      json: async () => ({ data: { token: 'late-active-token', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'home' } }),
    }),
    ticket,
    { setItem: () => { writes += 1; } },
  );
  assert.deepEqual(result, {
    session: { token: 'late-active-token', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'home' },
    adopted: false,
  });
  assert.equal(writes, 0);
});

test('activation rejects a non-home or malformed replacement without persistence', async () => {
  for (const data of [
    { token: 'token', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'onboarding' },
    { token: '', expiresAt: '2026-07-21T14:00:00Z', nextAction: 'home' },
  ]) {
    let writes = 0;
    await assert.rejects(
      activateApplicationSession(
        { token: 'onboarding-token', expiresAt: '2026-07-21T13:00:00Z', nextAction: 'onboarding' },
        async () => ({ ok: true, status: 200, json: async () => ({ data }) }),
        createSessionOperationOwner().issue(),
        { setItem: () => { writes += 1; } },
      ),
      (failure: unknown) => {
        assert.deepEqual(failure, { kind: 'http', status: 502 });
        return true;
      },
    );
    assert.equal(writes, 0);
  }
});

test('activation preserves actionable onboarding problem codes', async () => {
  for (const code of ['username_unavailable', 'policy_set_changed']) {
    await assert.rejects(
      activateApplicationSession(
        { token: 'onboarding-token', expiresAt: '2026-07-21T13:00:00Z', nextAction: 'onboarding' },
        async () => ({ ok: false, status: 409, problem: { code }, json: async () => undefined }),
        createSessionOperationOwner().issue(),
		{ setItem: () => assert.fail() },
      ),
      (failure: unknown) => {
        assert.deepEqual(failure, { kind: 'http', status: 409, code });
        return true;
      },
    );
  }
});
