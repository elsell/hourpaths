import type { Translator } from '@hourpaths/i18n';

export function formatCompactDuration(totalSeconds: number, translator: Translator): string {
  const seconds = Math.max(0, Math.floor(totalSeconds));
  if (seconds < 60) {
    return translator.t('duration.seconds', { seconds: translator.number(seconds) });
  }

  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) {
    return translator.t('duration.minutesSeconds', {
      minutes: translator.number(minutes),
      seconds: translator.number(seconds % 60),
    });
  }

  return translator.t('duration.hoursMinutes', {
    hours: translator.number(Math.floor(minutes / 60)),
    minutes: translator.number(minutes % 60),
  });
}


export function formatGoalDuration(totalSeconds: number, translator: Translator): string {
  const seconds = Math.max(0, Math.floor(totalSeconds));
  if (seconds < 60) return translator.t('duration.compactSeconds', { seconds: translator.number(seconds) });
  if (seconds >= 3600) {
    const hours = translator.number(Math.floor(seconds / 3600));
    const minutes = Math.floor(seconds % 3600 / 60);
    return minutes ? translator.t('duration.compactHoursMinutes', { hours, minutes: translator.number(minutes) })
      : translator.t('duration.compactHours', { hours });
  }
  return translator.t('duration.minutes', { minutes: translator.number(Math.floor(seconds / 60)) });
}

export function formatSessionClock(totalSeconds: number, translator: Translator): string {
  const seconds = Math.max(0, Math.floor(Number.isFinite(totalSeconds) ? totalSeconds : 0));
  const hours = Math.floor(seconds / 3600);
  const twoDigits = (value: number) => translator.number(value, { minimumIntegerDigits: 2, useGrouping: false });
  return translator.t(hours ? 'duration.clockHours' : 'duration.clockMinutes', {
    hours: translator.number(hours, { useGrouping: false }),
    minutes: hours ? twoDigits(Math.floor(seconds % 3600 / 60)) : translator.number(Math.floor(seconds / 60)),
    seconds: twoDigits(seconds % 60),
  });
}
