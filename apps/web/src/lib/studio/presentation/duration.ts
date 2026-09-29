import type { Translator } from '@hourpaths/i18n';
export function duration(i18n: Translator, seconds: number): string {
  const whole = Math.max(0, Math.floor(seconds));
  if (whole >= 3600) return i18n.t('studio.durationHours', { hours: Math.floor(whole / 3600), minutes: Math.floor(whole % 3600 / 60) });
  if (whole % 60 === 0) return i18n.t('studio.durationMinutes', { minutes: Math.floor(whole / 60) });
  return i18n.t('studio.duration', { minutes: Math.floor(whole / 60), seconds: whole % 60 });
}
