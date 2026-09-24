import assert from 'node:assert/strict';
import test from 'node:test';
import { buildOnboardingActivationInput, deviceOnboardingDefaults } from './onboarding';

const newYorkTimeZone = ['America', 'New_York'].join('/');
const utcTimeZone = ['U', 'TC'].join('');

test('device onboarding defaults use a trustworthy IANA zone and ISO locale weekday', () => {
  const defaults = deviceOnboardingDefaults({
    timeZone: () => newYorkTimeZone,
    locale: () => 'en-US',
    firstDay: () => 7,
  });
  assert.deepEqual(defaults, { timeZone: newYorkTimeZone, firstDayOfWeek: 7 });
});

test('same-language browser regions wire distinct ISO week starts', () => {
  const firstDay = (locale: string) => locale === 'en-US' ? 7 : locale === 'en-GB' ? 1 : undefined;
  assert.equal(deviceOnboardingDefaults({ timeZone: () => utcTimeZone, locale: () => 'en-US', firstDay }).firstDayOfWeek, 7);
  assert.equal(deviceOnboardingDefaults({ timeZone: () => utcTimeZone, locale: () => 'en-GB', firstDay }).firstDayOfWeek, 1);
});

test('device onboarding defaults reject an absent or non-IANA timezone', () => {
  for (const timeZone of ['', 'not/a-zone']) {
    assert.throws(() => deviceOnboardingDefaults({
      timeZone: () => timeZone,
      locale: () => 'en-US',
      firstDay: () => 1,
    }), /device time zone unavailable/);
  }
});

test('device onboarding defaults fail closed without a regional locale or trustworthy week info', () => {
  for (const source of [
    { timeZone: () => utcTimeZone, locale: () => '', firstDay: () => 1 },
    { timeZone: () => utcTimeZone, locale: () => 'en', firstDay: () => 1 },
    { timeZone: () => utcTimeZone, locale: () => 'en-US', firstDay: () => undefined },
  ]) assert.throws(() => deviceOnboardingDefaults(source), /device locale unavailable/);
});

test('activation payload trims reviewed names and preserves all ISO weekdays', () => {
  for (let firstDayOfWeek = 1; firstDayOfWeek <= 7; firstDayOfWeek += 1) {
    assert.deepEqual(buildOnboardingActivationInput({
      username: '  reviewed.name  ',
      displayName: '  Reviewed Name  ',
      profileVisibility: 'private',
      timeZone: newYorkTimeZone,
      firstDayOfWeek,
      policyReviewToken: 'review-token',
      atLeast16: true,
      termsAccepted: true,
      privacyAcknowledged: true,
      communityGuidelinesAccepted: true,
    }), {
      username: 'reviewed.name',
      displayName: 'Reviewed Name',
      profileVisibility: 'private',
      timeZone: newYorkTimeZone,
      firstDayOfWeek,
      policyReviewToken: 'review-token',
      atLeast16: true,
      termsAccepted: true,
      privacyAcknowledged: true,
      communityGuidelinesAccepted: true,
    });
  }
});

test('activation payload requires a deliberate visibility and valid profile values', () => {
  const valid = {
    username: 'reviewed.name', displayName: 'Reviewed Name', profileVisibility: 'public' as const,
    timeZone: utcTimeZone, firstDayOfWeek: 1, policyReviewToken: 'review-token', atLeast16: true,
    termsAccepted: true, privacyAcknowledged: true, communityGuidelinesAccepted: true,
  };
  for (const invalid of [
    { ...valid, profileVisibility: '' as const },
    { ...valid, displayName: ' ' },
    { ...valid, firstDayOfWeek: 0 },
    { ...valid, firstDayOfWeek: 8 },
  ]) assert.throws(() => buildOnboardingActivationInput(invalid), /invalid onboarding profile/);
});
