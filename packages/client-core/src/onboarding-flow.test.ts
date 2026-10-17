import test from 'node:test';
import assert from 'node:assert/strict';
import { advanceOnboardingStep, previousOnboardingStep, recoverOnboardingStep } from './onboarding-flow';

test('guided setup cannot skip invalid identity or preferences, and recovers to the relevant task', () => {
  assert.equal(advanceOnboardingStep('identity', false, false), 'identity');
  let step = advanceOnboardingStep('identity', true, false);
  assert.equal(step, 'preferences');
  assert.equal(advanceOnboardingStep(step, false, false), 'preferences');
  step = advanceOnboardingStep(step, true, false);
  assert.equal(step, 'review');
  assert.equal(previousOnboardingStep(step, true), 'review'); // Activation owns the UI.
  assert.equal(previousOnboardingStep(step, false), 'preferences');
  assert.equal(recoverOnboardingStep(step, 'username'), 'identity');
  assert.equal(recoverOnboardingStep('preferences', 'policy'), 'review');
  assert.equal(recoverOnboardingStep(step, 'timeZone'), 'preferences');
  assert.equal(recoverOnboardingStep(step, 'unavailable'), 'review');
  assert.equal(advanceOnboardingStep('review', true, false), 'review'); // Only activation can enter photo setup.
  assert.equal(previousOnboardingStep('photo', false), 'photo'); // Never resubmit an already activated account.
});
