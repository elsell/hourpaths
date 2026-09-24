import type { Translator } from '@hourpaths/i18n';

const minutesPerHour = 60;
const minutesPerDay = 24 * minutesPerHour;
const minutesPerYear = 365 * minutesPerDay;

export type OwnershipTransferExpirationPresentation = {
  exact: string;
  relative: string;
  summary: string;
};

export function ownershipTransferExpirationPresentation(
  createdAt: string,
  expiresAt: string,
  timeZone: string,
  translator: Translator,
): OwnershipTransferExpirationPresentation | undefined {
  const created = Date.parse(createdAt);
  const expires = Date.parse(expiresAt);
  if (!Number.isFinite(created) || !Number.isFinite(expires) || expires <= created || timeZone.trim() !== timeZone || timeZone === '') return undefined;
  const durationMilliseconds = expires - created;
  if (durationMilliseconds % 60_000 !== 0) return undefined;

  let remaining = durationMilliseconds / 60_000;
  const values = [
    ['duration.years', Math.floor(remaining / minutesPerYear)],
    ['duration.days', Math.floor((remaining %= minutesPerYear) / minutesPerDay)],
    ['duration.hours', Math.floor((remaining %= minutesPerDay) / minutesPerHour)],
    ['duration.minutes', remaining % minutesPerHour],
  ] as const;
  const relative = values
    .filter(([, value]) => value > 0)
    .map(([key, value]) => translator.t(key, { count: value }))
    .join(' ');
  if (!relative) return undefined;

  try {
    const exact = translator.t('pathOwnership.exactExpiration', {
      date: translator.date(expires, { dateStyle: 'medium', timeZone }),
      time: translator.time(expires, { timeStyle: 'short', timeZone }),
    });
    return {
      exact,
      relative,
      summary: translator.t('pathOwnership.expiration', { exact, relative }),
    };
  } catch (error) {
    if (error instanceof RangeError) return undefined;
    throw error;
  }
}
