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
