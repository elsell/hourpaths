import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import test from 'node:test';
import type { SessionExchangeCredential } from '@hourpaths/client-core';
import { continueMobileNewAccount } from './recovery-decline';

const recovery = {
  token: 'restricted-token',
  expiresAt: '2026-07-21T18:00:00Z',
  nextAction: 'duplicate_email_recovery' as const,
};

test('declining duplicate recovery persists onboarding before changing the mobile destination', async () => {
  const events: string[] = [];
  let persisted: SessionExchangeCredential | undefined;
  let activated: SessionExchangeCredential | undefined;

  const replacement = await continueMobileNewAccount({
    credential: recovery,
    current: () => true,
    decline: async () => { events.push('decline'); },
    persist: async (next) => { events.push('persist'); persisted = next; },
    activate: async (next) => { events.push('activate'); activated = next; },
  });

  assert.deepEqual(events, ['decline', 'persist', 'activate']);
  assert.deepEqual(replacement, {
    token: recovery.token,
    expiresAt: recovery.expiresAt,
    nextAction: 'onboarding',
  });
  assert.deepEqual(persisted, replacement);
  assert.deepEqual(activated, replacement);
});

test('failed secure persistence leaves duplicate recovery visible', async () => {
  let activated = false;
  const storageFailure = { kind: 'local_storage', reason: 'malformed' } as const;

  await assert.rejects(
    continueMobileNewAccount({
      credential: recovery,
      current: () => true,
      decline: async () => {},
      persist: async () => { throw storageFailure; },
      activate: async () => { activated = true; },
    }),
    storageFailure,
  );
  assert.equal(activated, false);
});

test('a rejected server decline cannot persist or open onboarding', async () => {
  let persisted = false;
  let activated = false;
  const rejection = { kind: 'http', status: 503 } as const;

  await assert.rejects(continueMobileNewAccount({
    credential: recovery,
    current: () => true,
    decline: async () => { throw rejection; },
    persist: async () => { persisted = true; },
    activate: async () => { activated = true; },
  }), rejection);
  assert.equal(persisted, false);
  assert.equal(activated, false);
});

test('a superseded decline cannot change the mobile destination', async () => {
  let current = true;
  let activated = false;

  const result = await continueMobileNewAccount({
    credential: recovery,
    current: () => current,
    decline: async () => {},
    persist: async () => { current = false; },
    activate: async () => { activated = true; },
  });

  assert.equal(result, null);
  assert.equal(activated, false);
});

test('the recovery presentation wires the tested persisted transition', () => {
  const app = readFileSync(`${import.meta.dirname}/../app/index.tsx`, 'utf8');
  const screenPath = `${import.meta.dirname}/ui/duplicate-email-recovery-screen.tsx`;
  const screen = existsSync(screenPath) ? readFileSync(screenPath, 'utf8') : '';
  assert.match(app, /continueMobileNewAccount\(\{/);
  assert.match(app, /declineDuplicateEmailRecovery\(\)/);
  assert.match(app, /persist: \(replacement\) => serializedSessionStorage\.persist\(replacement, ticket\.current\)/);
  assert.match(app, /<DuplicateEmailRecoveryScreen/);
  assert.match(app, /onContinue=\{\(\) => void continueCreatingNewAccount\(\)\}/);
  assert.match(app, /onReturnToSignIn=\{\(\) => void clearSession\(\)\}/);
  assert.doesNotMatch(app, /destination\?\.kind === 'duplicate_email_recovery' \? <View style=\{\{/);
  assert.match(screen, /<ScreenHeader/);
  assert.match(screen, /<NativePrimaryButton/);
  assert.match(screen, /systemImage="rectangle\.portrait\.and\.arrow\.right"/);
  assert.match(screen, /duplicateEmailRecovery\.returnToSignIn/);
  assert.match(screen, /duplicateEmailRecovery\.decline/);
  assert.match(screen, /<StatusBanner/);
  assert.match(screen, /busy=\{declining\}/);
  assert.doesNotMatch(screen, /<Button/);
  const handler = /async function continueCreatingNewAccount[\s\S]*?\n  }/.exec(app)?.[0];
  assert.ok(handler);
  assert.match(handler, /setDestination\(\{ kind: 'onboarding', profile: onboardingProfile \}\)/);
  assert.doesNotMatch(handler, /\.profile\(\)|\.onboarding\(\)|activate\(replacement/);
});
