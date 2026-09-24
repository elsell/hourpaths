import assert from 'node:assert/strict';
import test from 'node:test';
import { activeTimerSeconds, classifySessionFailure, createManualActivityFormState, createSessionOperationOwner, declineDuplicateEmailRecovery, exchangeSessionCredential, isValidSessionCredential, manualActivityParticipantNow, overallProgress, overrideManualActivityOccurrence, publicEndpointConfig, publicEnvironmentConfig, publicStringConfig, refreshSessionCredential, retainedSessionExpiry, serializeManualActivityForm, sessionFailureFromResponse, sessionRetryDelay, updateManualActivityDuration, validateSessionCredential, validateSessionMutation, type SessionFailure } from './index.js';

test('active timer duration excludes previously accumulated activity', () => {
  assert.equal(activeTimerSeconds('2026-07-21T12:00:00Z', Date.parse('2026-07-21T12:01:01.900Z')), 61);
  assert.equal(activeTimerSeconds(undefined, Date.now()), 0);
  assert.equal(activeTimerSeconds('invalid', Date.now()), 0);
});

test('overall progress is omitted when the path has no overall target', () => {
  assert.equal(overallProgress(60), undefined);
});

test('overall progress preserves zero, partial, and uncapped completed time', () => {
  assert.deepEqual(overallProgress(0, { targetSeconds: 120 }), {
    accumulatedSeconds: 0,
    targetSeconds: 120,
    visualSeconds: 0,
    completed: false,
  });
  assert.deepEqual(overallProgress(60, { targetSeconds: 120 }), {
    accumulatedSeconds: 60,
    targetSeconds: 120,
    visualSeconds: 60,
    completed: false,
  });
  assert.deepEqual(overallProgress(150, { targetSeconds: 120 }), {
    accumulatedSeconds: 150,
    targetSeconds: 120,
    visualSeconds: 120,
    completed: true,
  });
});

test('network, 429, and 503 preserve a valid credential as authenticated_offline', () => {
  for (const failure of [{ kind: 'network' } as const, { kind: 'http', status: 429 } as const, { kind: 'http', status: 503 } as const]) {
    assert.deepEqual(classifySessionFailure(failure), {
      state: 'authenticated_offline', discardCredential: false, retryable: true,
    });
  }
});

test('401 and expired credentials require authentication and are discarded', () => {
  for (const failure of [{ kind: 'http', status: 401 } as const, { kind: 'expired' } as const]) {
    assert.deepEqual(classifySessionFailure(failure), {
      state: 'authentication_required', discardCredential: true, retryable: false,
    });
  }
});

test('non-401 HTTP failures preserve credentials and only transient statuses retry', () => {
  for (const status of [400, 403, 404]) {
    assert.deepEqual(classifySessionFailure({ kind: 'http', status }), {
      state: 'authenticated_offline', discardCredential: false, retryable: false,
    });
  }
  assert.deepEqual(classifySessionFailure({ kind: 'http', status: 408 }), {
    state: 'authenticated_offline', discardCredential: false, retryable: true,
  });
});

test('known RFC 9457 problem codes are retained without trusting presentation text', () => {
  for (const [status, code] of [
    [401, 'invalid_credential'],
    [409, 'idempotency_conflict'],
    [503, 'authorization_pending'],
    [503, 'authorization_dead_lettered'],
    [503, 'authorization_policy_not_configured'],
  ] as const) {
    assert.deepEqual(
      sessionFailureFromResponse(status, { code, title: 'Hostile title', detail: '<script>hostile</script>' }),
      { kind: 'http', status, code },
    );
  }
  for (const problem of [null, {}, { code: 7 }, { code: 'unknown_server_code' }, 'not a problem']) {
    assert.deepEqual(sessionFailureFromResponse(400, problem), { kind: 'http', status: 400 });
  }
});

test('status remains authoritative when an untrusted problem code disagrees', () => {
  assert.deepEqual(
    classifySessionFailure(sessionFailureFromResponse(500, { code: 'unauthenticated' })),
    { state: 'authenticated_offline', discardCredential: false, retryable: true },
  );
  assert.deepEqual(
    classifySessionFailure(sessionFailureFromResponse(401, { code: 'internal_error' })),
    { state: 'authentication_required', discardCredential: true, retryable: false },
  );
});

test('malformed local storage is classified separately and discarded', () => {
  assert.deepEqual(classifySessionFailure({ kind: 'local_storage', reason: 'malformed' }), {
    state: 'local_session_unreadable', discardCredential: true, retryable: false,
  });
});

const current = { token: 'old-token', expiresAt: '2026-07-18T12:00:00Z' };
const replacement = { token: 'new-token', expiresAt: '2026-07-18T13:00:00Z' };

test('session exchange preserves the closed next action through persistence', async () => {
  for (const nextAction of ['home', 'onboarding', 'duplicate_email_recovery'] as const) {
    const saved: unknown[] = [];
    const exchanged = await exchangeSessionCredential(
      async () => ({
        ok: true,
        status: 200,
        json: async () => ({ data: { ...replacement, nextAction } }),
      }),
      async (value) => { saved.push(value); },
    );
    assert.deepEqual(exchanged, { ...replacement, nextAction });
    assert.deepEqual(saved, [{ ...replacement, nextAction }]);
  }
});

test('session exchange rejects missing or unknown next actions before persistence', async () => {
  for (const data of [replacement, { ...replacement, nextAction: 'unexpected' }]) {
    const saved: unknown[] = [];
    await assert.rejects(
      exchangeSessionCredential(
        async () => ({ ok: true, status: 200, json: async () => ({ data }) }),
        async (value) => { saved.push(value); },
      ),
      (failure: SessionFailure) => {
        assert.deepEqual(failure, { kind: 'http', status: 502 });
        return true;
      },
    );
    assert.deepEqual(saved, []);
  }
});

test('refresh retains a stored next action while rotating the credential', async () => {
  const onboarding = { ...current, nextAction: 'onboarding' as const };
  const saved: unknown[] = [];
  const refreshed = await refreshSessionCredential(
    onboarding,
    async () => ({ ok: true, status: 200, json: async () => ({ data: replacement }) }),
    async (value) => { saved.push(value); },
  );
  assert.deepEqual(refreshed, { ...replacement, nextAction: 'onboarding' });
  assert.deepEqual(saved, [{ ...replacement, nextAction: 'onboarding' }]);
});

test('stored session validation rejects an action outside the generated contract', () => {
  assert.equal(isValidSessionCredential({ ...current, nextAction: 'onboarding' }), true);
  assert.equal(isValidSessionCredential({ ...current, nextAction: 'unexpected' }), false);
});

test('declining duplicate-email recovery retains the credential while opening onboarding', () => {
  const recovery = {
    token: 'recovery-token',
    expiresAt: '2026-07-21T13:00:00Z',
    nextAction: 'duplicate_email_recovery' as const,
  };
  assert.deepEqual(declineDuplicateEmailRecovery(recovery), {
    token: recovery.token,
    expiresAt: recovery.expiresAt,
    nextAction: 'onboarding',
  });
});

test('duplicate-email recovery decline is a closed transition', () => {
  for (const nextAction of ['home', 'onboarding'] as const) {
    assert.throws(
      () => declineDuplicateEmailRecovery({ ...current, nextAction }),
      /duplicate_email_recovery/,
    );
  }
});

test('refresh adapters classify network, 401, 429, and 503 without overwriting the credential', async () => {
  for (const [name, request, expected] of [
    ['network', async () => { throw new Error('offline'); }, { kind: 'network' }],
    ['401', async () => ({ ok: false, status: 401, json: async () => ({}) }), { kind: 'http', status: 401 }],
    ['429', async () => ({ ok: false, status: 429, json: async () => ({}) }), { kind: 'http', status: 429 }],
    ['503', async () => ({ ok: false, status: 503, json: async () => ({}) }), { kind: 'http', status: 503 }],
  ] as const) {
    const saved: unknown[] = [];
    await assert.rejects(
      refreshSessionCredential(current, request, async (value) => { saved.push(value); }),
      (failure: SessionFailure) => { assert.deepEqual(failure, expected, name); return true; },
    );
    assert.deepEqual(saved, [], `${name} must not replace storage`);
  }
});

test('refresh adapters retain a generated semantic problem code', async () => {
  await assert.rejects(
    refreshSessionCredential(
      current,
      async () => ({
        ok: false,
        status: 503,
        problem: { code: 'authorization_pending', detail: 'do not render me' },
        json: async () => ({}),
      }),
      async () => assert.fail('a failed refresh must not persist'),
    ),
    (failure: SessionFailure) => {
      assert.deepEqual(failure, { kind: 'http', status: 503, code: 'authorization_pending' });
      return true;
    },
  );
});

test('refresh adapters atomically persist a validated replacement credential', async () => {
  const saved: unknown[] = [];
  const result = await refreshSessionCredential(
    current,
    async () => ({ ok: true, status: 200, json: async () => ({ data: replacement }) }),
    async (value) => { saved.push(value); },
  );
  assert.deepEqual(result, replacement);
  assert.deepEqual(saved, [replacement]);
});

test('exchange and refresh classify credential persistence failures as unreadable local storage', async () => {
  const storageFailure = new Error('credential storage is unavailable');
  storageFailure.name = 'SecurityError';
  const request = async () => ({
    ok: true,
    status: 200,
    json: async () => ({ data: { ...replacement, nextAction: 'home' as const } }),
  });
  for (const operation of ['exchange', 'refresh'] as const) {
    await assert.rejects(
      operation === 'exchange'
        ? exchangeSessionCredential(request, async () => { throw storageFailure; })
        : refreshSessionCredential(current, request, async () => { throw storageFailure; }),
      (failure: SessionFailure) => {
        assert.deepEqual(failure, { kind: 'local_storage', reason: 'malformed' }, operation);
        return true;
      },
    );
  }
});

test('exchange and refresh reject non-string credential fields before persistence', async () => {
  for (const malformed of [
    { token: 123, expiresAt: '2026-07-18T13:00:00Z' },
    { token: 'new-token', expiresAt: 123 },
  ]) {
    for (const operation of ['exchange', 'refresh'] as const) {
      const saved: unknown[] = [];
      const request = async () => ({
        ok: true,
        status: 200,
        json: async () => ({ data: malformed }),
      });
      await assert.rejects(
        operation === 'exchange'
          ? exchangeSessionCredential(request, async (value) => { saved.push(value); })
          : refreshSessionCredential(current, request, async (value) => { saved.push(value); }),
        (failure: SessionFailure) => {
          assert.deepEqual(failure, { kind: 'http', status: 502 }, operation);
          return true;
        },
      );
      assert.deepEqual(saved, [], `${operation} must not persist malformed credentials`);
    }
  }
});

test('a profile outage after rotation leaves the replacement credential persisted for recovery', async () => {
  let saved = current;
  const next = await refreshSessionCredential(
    current,
    async () => ({ ok: true, status: 200, json: async () => ({ data: replacement }) }),
    async (value) => { saved = value; },
  );
  await assert.rejects(
    validateSessionCredential(next, async () => { throw new Error('offline'); }),
    (failure: SessionFailure) => failure.kind === 'network',
  );
  assert.deepEqual(saved, replacement);
});

test('session mutations accept no-content success and retain typed failure boundaries', async () => {
  await assert.doesNotReject(validateSessionMutation(async () => ({
    ok: true,
    status: 204,
    json: async () => undefined,
  })));
  await assert.rejects(
    validateSessionMutation(async () => ({
      ok: false,
      status: 409,
      problem: { code: 'conflict' },
      json: async () => undefined,
    })),
    { kind: 'http', status: 409, code: 'conflict' },
  );
  await assert.rejects(
    validateSessionMutation(async () => { throw new Error(); }),
    { kind: 'network' },
  );
});

test('profile-outage retry timing follows the retained rotated credential', () => {
  const oldExpiry = '2026-07-18T12:05:00Z';
  const replacement = { token: 'replacement', expiresAt: '2026-07-18T13:00:00Z' };
  assert.equal(retainedSessionExpiry(replacement, oldExpiry), replacement.expiresAt);
  assert.equal(retainedSessionExpiry(null, oldExpiry), oldExpiry);
});

test('retry backoff is bounded by five minutes and credential expiry', () => {
  const now = Date.parse('2026-07-18T12:00:00Z');
  assert.equal(sessionRetryDelay(0, '2026-07-18T13:00:00Z', now), 30_000);
  assert.equal(sessionRetryDelay(20, '2026-07-18T13:00:00Z', now), 300_000);
  assert.equal(sessionRetryDelay(20, '2026-07-18T12:01:00Z', now), 60_000);
  assert.equal(sessionRetryDelay(0, '2026-07-18T11:59:00Z', now), 0);
});

test('production endpoint configuration fails closed for unsafe actual bundle values', () => {
  for (const value of [undefined, 'http://api.example.com', 'https://LOCALHOST/path', 'https://sub.localhost/path', 'https://127.0.0.2', 'https://[::1]', 'https://user:pass@example.com', 'https://api.example.com?target=dev', 'https://api.example.com/#dev']) {
    assert.throws(() => publicEndpointConfig(value, 'http://localhost:8080', 'EXPO_PUBLIC_API_URL', true));
  }
  assert.equal(publicEndpointConfig('https://api.example.com/', 'http://localhost:8080', 'API', true), 'https://api.example.com');
  assert.equal(publicEndpointConfig(undefined, 'http://localhost:8080', 'API', false), 'http://localhost:8080');
});

test('unknown deployment environment fails closed', () => {
  assert.equal(publicEnvironmentConfig(undefined, 'PUBLIC_APP_ENV'), 'development');
  assert.equal(publicEnvironmentConfig('production', 'PUBLIC_APP_ENV'), 'production');
  assert.throws(() => publicEnvironmentConfig('prod', 'PUBLIC_APP_ENV'));
});

test('missing production string configuration fails closed', () => {
  assert.equal(publicStringConfig(undefined, 'local-client', 'PUBLIC_OIDC_CLIENT_ID', false), 'local-client');
  assert.equal(publicStringConfig('production-client', 'local-client', 'PUBLIC_OIDC_CLIENT_ID', true), 'production-client');
  assert.throws(() => publicStringConfig(undefined, 'local-client', 'PUBLIC_OIDC_CLIENT_ID', true));
  assert.throws(() => publicStringConfig('   ', 'local-client', 'PUBLIC_OIDC_CLIENT_ID', true));
});

test('session operation tickets are invalidated by sign-out and newer ownership', () => {
  const owner = createSessionOperationOwner();
  const first = owner.issue();
  assert.equal(first.current(), true);
  const replacement = owner.issue();
  assert.equal(first.current(), false);
  assert.equal(replacement.current(), true);
  owner.invalidate();
  assert.equal(replacement.current(), false);
});

test('manual activity defaults to the participant-local date and start time', () => {
  const now = manualActivityParticipantNow('2026-07-23T04:00:30Z', 'America/New_York');
  assert.deepEqual(
    createManualActivityFormState(now),
    {
      localDate: '2026-07-23',
      localTime: '00:00:30',
      durationSeconds: '',
      occurrenceTouched: false,
    },
  );
});

test('duration changes re-anchor an untouched occurrence to end at injected local now', () => {
  const initialNow = manualActivityParticipantNow('2026-07-23T04:00:30Z', 'America/New_York');
  const initial = createManualActivityFormState(initialNow);
  const first = updateManualActivityDuration(
    initial,
    '90',
    initialNow,
  );
  assert.deepEqual(first, {
    localDate: '2026-07-22',
    localTime: '23:59:00',
    durationSeconds: '90',
    occurrenceTouched: false,
  });
  assert.deepEqual(
    updateManualActivityDuration(first, '120', manualActivityParticipantNow('2026-07-23T04:02:30Z', 'America/New_York')),
    {
      localDate: '2026-07-23',
      localTime: '00:00:30',
      durationSeconds: '120',
      occurrenceTouched: false,
    },
  );
});

test('an explicit date or start override is preserved when duration changes', () => {
  const initial = createManualActivityFormState(manualActivityParticipantNow('2026-07-23T16:00:00Z', 'America/New_York'));
  for (const occurrence of [
    { localDate: '2026-07-20' },
    { localTime: '09:30:00' },
    { localDate: '2026-07-20', localTime: '09:30:00' },
  ]) {
    const overridden = overrideManualActivityOccurrence(initial, occurrence);
    const changed = updateManualActivityDuration(
      overridden,
      '3600',
      manualActivityParticipantNow('2026-07-23T17:00:00Z', 'America/New_York'),
    );
    assert.equal(changed.localDate, occurrence.localDate ?? initial.localDate);
    assert.equal(changed.localTime, occurrence.localTime ?? initial.localTime);
    assert.equal(changed.occurrenceTouched, true);
  }
});

test('manual activity serialization requires a positive whole-second duration', () => {
  const now = manualActivityParticipantNow('2026-07-23T16:01:00Z', 'America/New_York');
  const initial = createManualActivityFormState(now);
  for (const duration of ['', '0', '-1', '1.5', 'seconds', '9007199254740992']) {
    const state = updateManualActivityDuration(initial, duration, now);
    assert.deepEqual(serializeManualActivityForm(state, now), { ok: false, reason: 'duration' });
  }
});

test('manual activity serialization rejects an end after injected local now', () => {
  const initial = createManualActivityFormState(manualActivityParticipantNow('2026-07-23T16:00:00Z', 'America/New_York'));
  const state = updateManualActivityDuration(
    overrideManualActivityOccurrence(initial, { localTime: '12:00:00' }),
    '60',
    manualActivityParticipantNow('2026-07-23T16:00:00Z', 'America/New_York'),
  );
  assert.deepEqual(
    serializeManualActivityForm(state, manualActivityParticipantNow('2026-07-23T16:00:59Z', 'America/New_York')),
    { ok: false, reason: 'future_end' },
  );
});

test('manual activity serialization returns local fields when the end is not future', () => {
  const initial = createManualActivityFormState(manualActivityParticipantNow('2026-07-23T16:00:00Z', 'America/New_York'));
  const state = updateManualActivityDuration(
    overrideManualActivityOccurrence(initial, { localDate: '2026-07-22', localTime: '09:30:00' }),
    '3600',
    manualActivityParticipantNow('2026-07-23T16:00:00Z', 'America/New_York'),
  );
  assert.deepEqual(
    serializeManualActivityForm(state, manualActivityParticipantNow('2026-07-23T16:00:00Z', 'America/New_York')),
    {
      ok: true,
      fields: { localDate: '2026-07-22', localTime: '09:30:00', durationSeconds: 3600 },
    },
  );
});

test('participant-local now follows the repeated hour from an authoritative instant', () => {
  assert.deepEqual(
    manualActivityParticipantNow('2026-11-01T05:59:59Z', 'America/New_York', 2_000),
    {
      localDate: '2026-11-01',
      localTime: '01:00:01',
      currentInstant: '2026-11-01T06:00:01.000Z',
      timeZone: 'America/New_York',
    },
  );
});

test('duration re-anchoring subtracts elapsed time through a repeated hour', () => {
  const now = manualActivityParticipantNow('2026-11-01T06:30:00Z', 'America/New_York');
  const state = updateManualActivityDuration(createManualActivityFormState(now), '3600', now);
  assert.equal(state.localDate, '2026-11-01');
  assert.equal(state.localTime, '01:30:00');
  assert.deepEqual(serializeManualActivityForm(state, now), {
    ok: true,
    fields: { localDate: '2026-11-01', localTime: '01:30:00', durationSeconds: 3600 },
  });
});

test('future validation resolves a nonexistent start forward through the gap', () => {
  const now = manualActivityParticipantNow('2026-03-08T07:30:01Z', 'America/New_York');
  const state = updateManualActivityDuration(
    overrideManualActivityOccurrence(createManualActivityFormState(now), { localTime: '02:30:00' }),
    '1',
    now,
  );
  assert.deepEqual(serializeManualActivityForm(state, now), {
    ok: true,
    fields: { localDate: '2026-03-08', localTime: '02:30:00', durationSeconds: 1 },
  });
});
