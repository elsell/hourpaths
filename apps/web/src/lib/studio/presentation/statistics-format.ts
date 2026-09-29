import type { MessageKey, Translator } from '@hourpaths/i18n';
import type { Range, Statistics } from '../analytics/domain/statistics';
export const rangeLabels: Record<Range, MessageKey> = { day: 'stats.range.day', week: 'stats.range.week', month: 'stats.range.month', year: 'stats.range.year', all_time: 'stats.range.allTime' };
export function calendarLabel(key: string, unit: string, i18n: Translator): string {
  if (unit === 'year') return i18n.number(Number(key.slice(0, 4)), { useGrouping: false });
  if (unit === 'hour') return i18n.time(Date.UTC(2000, 0, 1, Number(key.slice(11, 13))), { hour: 'numeric', timeZone: 'utc' });
  const date = new Date((unit === 'month' ? key.slice(0, 7) + '-01' : key.slice(0, 10)) + 'T12:00:00Z');
  return i18n.date(date, { timeZone: 'utc', ...(unit === 'month' ? { month: 'short', year: 'numeric' } : { month: 'short', day: 'numeric' }) });
}
export function periodLabel(data: Statistics, i18n: Translator): string {
  if (data.range === 'all_time') return i18n.t('stats.range.allTime');
  const first = new Date(data.firstDate + 'T12:00:00Z');
  if (data.range === 'day') return i18n.date(first, { dateStyle: 'long', timeZone: 'utc' });
  if (data.range === 'month') return i18n.date(first, { month: 'long', year: 'numeric', timeZone: 'utc' });
  if (data.range === 'year') return i18n.date(first, { year: 'numeric', timeZone: 'utc' });
  return i18n.t('stats.period', { start: i18n.date(first, { month: 'short', day: 'numeric', timeZone: 'utc' }), end: i18n.date(new Date(data.lastDate + 'T12:00:00Z'), { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'utc' }) });
}
