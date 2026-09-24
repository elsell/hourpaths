import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const page = readFileSync(new URL('../routes/onboarding/+page.svelte', import.meta.url), 'utf8');

test('onboarding renders the server policy set and every required affirmation', () => {
  assert.match(page, /policies!?\.termsOfService\.url/);
  assert.match(page, /policies\.termsOfService\.version/);
  assert.match(page, /policies!?\.privacyPolicy\.url/);
  assert.match(page, /policies\.privacyPolicy\.version/);
  assert.match(page, /policies!?\.communityGuidelines\.url/);
  assert.match(page, /policies\.communityGuidelines\.version/);
  assert.match(page, /policies!?\.supportUrl/);
  assert.match(page, /openWebPolicyLink/);
  assert.doesNotMatch(page, /href=\{policies\./);
  for (const affirmation of ['atLeast16', 'termsAccepted', 'privacyAcknowledged', 'communityGuidelinesAccepted']) {
    assert.match(page, new RegExp(`bind:checked=\\{${affirmation}\\}`));
  }
});

test('onboarding submits all editable values and the server review token', () => {
  assert.match(page, /buildOnboardingActivationInput\(\{/);
  assert.match(page, /\.activateOnboarding\(activationInput\)/);
  for (const field of ['username', 'displayName', 'profileVisibility', 'timeZone', 'firstDayOfWeek', 'policyReviewToken']) {
    assert.match(page, new RegExp(`\\b${field},`));
  }
  assert.match(page, /activateApplicationSession/);
  assert.match(page, /replaceApplicationLocation\('\/'\)/);
});

test('device timezone is sent without asking the user and ISO weekdays remain selectable', () => {
  assert.match(page, /deviceOnboardingDefaults\(\)/);
  assert.match(page, /onboarding\.timeZoneUnavailable/);
  assert.match(page, /onboarding\.localeUnavailable/);
  assert.doesNotMatch(page, /onboarding\.timeZoneLabel/);
  assert.match(page, /\[1, 2, 3, 4, 5, 6, 7\]\.map/);
  assert.match(page, /\bvalue,/);
});

test('visibility and a trimmed nonblank display name require deliberate reviewed input', () => {
  assert.match(page, /profileVisibility: 'public' \| 'private' \| '' = ''/);
  assert.match(page, /onboarding\.visibilityChoose/);
  assert.doesNotMatch(page, /maxlength="100"/);
  assert.match(page, /autocomplete="name" required onblur=\{\(\) => displayName = displayName\.trim\(\)\}/);
  assert.match(page, /aria-describedby="username-notice username-error"/);
  assert.match(page, /id="username-error" role="alert"/);
});

test('policy changes refresh review while username conflicts stay in the form', () => {
  assert.match(page, /failure\.code === 'policy_set_changed'/);
  assert.match(page, /submitting = false;\s*try \{ await loadOnboardingReview/);
  assert.match(page, /await loadOnboardingReview/);
  assert.match(page, /failure\.code === 'username_unavailable'/);
  assert.match(page, /aria-invalid=\{onboardingErrorKey === 'onboarding\.usernameUnavailable'\}/);
});

test('onboarding keeps operation ownership across load, activation, and sign-out', () => {
  assert.match(page, /applicationSessionOperations\.issue\(\)/);
  assert.match(page, /if \(!ticket\.current\(\)\) return/);
  assert.match(page, /revokeSupersededApplicationSession/);
  assert.match(page, /revokeApplicationSession/);
});
