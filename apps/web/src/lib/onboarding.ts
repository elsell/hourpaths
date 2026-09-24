import type { OnboardingActivationInput } from '@hourpaths/api-client';
import { browserDeviceDefaultsSource, type DeviceDefaultsSource } from './device-locale';

type VisibilityChoice = OnboardingActivationInput['profileVisibility'] | '';

export function deviceOnboardingDefaults(
  source: DeviceDefaultsSource = browserDeviceDefaultsSource,
): { timeZone: string; firstDayOfWeek: number } {
  const timeZone = source.timeZone();
  try {
    if (!timeZone || Intl.DateTimeFormat('en', { timeZone }).resolvedOptions().timeZone !== timeZone) {
      throw new Error();
    }
  } catch {
    throw new Error('device time zone unavailable');
  }
  const locale = source.locale();
  try {
    if (!locale || !new Intl.Locale(locale).region) throw new Error();
  } catch {
    throw new Error('device locale unavailable');
  }
  const localeFirstDay = source.firstDay(locale);
  if (!Number.isInteger(localeFirstDay) || localeFirstDay! < 1 || localeFirstDay! > 7) {
    throw new Error('device locale unavailable');
  }
  return { timeZone, firstDayOfWeek: localeFirstDay! };
}

export function buildOnboardingActivationInput(input: {
  username: string;
  displayName: string;
  profileVisibility: VisibilityChoice;
  timeZone: string;
  firstDayOfWeek: number;
  policyReviewToken: string;
  atLeast16: boolean;
  termsAccepted: boolean;
  privacyAcknowledged: boolean;
  communityGuidelinesAccepted: boolean;
}): OnboardingActivationInput {
  const username = input.username.trim();
  const displayName = input.displayName.trim();
  if (!input.profileVisibility || !username || !displayName
    || !Number.isInteger(input.firstDayOfWeek) || input.firstDayOfWeek < 1 || input.firstDayOfWeek > 7) {
    throw new Error('invalid onboarding profile');
  }
  return { ...input, username, displayName, profileVisibility: input.profileVisibility };
}
