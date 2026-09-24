import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { fileURLToPath, URL } from 'node:url';

test('onboarding owns its account-exit action instead of inheriting a detached shell button', async () => {
  const app = await readFile(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');
  const onboarding = await readFile(fileURLToPath(new URL('./ui/onboarding-form.tsx', import.meta.url)), 'utf8');

  assert.match(app, /<OnboardingForm[\s\S]*onSignOut=\{\(\) => void clearSession\(\)\}/);
  assert.match(onboarding, /onSignOut:\s*\(\) => void/);
  assert.match(onboarding, /leadingAction=\{\{[\s\S]*auth\.signOut[\s\S]*onPress:\s*onSignOut/);
  assert.doesNotMatch(
    app,
    /destination && destination\.kind !== 'home'[\s\S]*<ActionButton/,
  );
});

test('the shell retains locked Home recovery props after paused or rejected activation loads', async () => {
  const app = await readFile(fileURLToPath(new URL('../app/index.tsx', import.meta.url)), 'utf8');

  assert.match(app, /setOnboardingHomeRecovery\(\{ sessionToken: credential\.token, status: 'loading' \}\)/);
  assert.match(app, /failure\.kind === 'network' \? 'offline' : 'error'/);
  assert.match(app, /if \(next\.session\) setOnboardingHomeRecovery/);
  assert.match(app, /if \(onboardingHomeRecovery \|\| onboardingActivations\.blocked\(\)\) return/);
  assert.match(app, /busy=\{activatingOnboarding \|\| onboardingHomeRecovery !== null\}/);
  assert.match(app, /homeRecoveryStatus=\{onboardingHomeRecovery\?\.status\}/);
  assert.match(app, /onRetryHome=\{\(\) => void retryOnboardingHome\(\)\}/);
  assert.match(app, /async function retryOnboardingHome[\s\S]*mode: 'profile'/);
  assert.match(app, /if \(onboardingHomeRecovery\?\.status === 'loading'\) return/);
});
