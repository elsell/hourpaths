import type { StatsSummary } from '@hourpaths/api-client';
import type { MessageKey, Translator } from '@hourpaths/i18n';
import { defaultPathAppearance, pathPalette, type PathAppearance } from './path-appearance';

export const statsRanges = ['day', 'week', 'month', 'year', 'all_time'] as const;
export type StatsRange = typeof statsRanges[number];
export type StatsSelection = { range: StatsRange; anchor?: string; pathIds: readonly string[] };
export type StatsState = {
  status: 'idle' | 'loading' | 'ready' | 'error';
  selection: StatsSelection;
  data?: StatsSummary;
  paths?: StatsSummary['availablePaths'];
  refreshing: boolean;
};
export const initialStatsSelection = (): StatsSelection => ({ range: 'week', pathIds: [] });
export const statsRangeKeys: Record<StatsRange, MessageKey> = {
  day: 'stats.range.day', week: 'stats.range.week', month: 'stats.range.month', year: 'stats.range.year', all_time: 'stats.range.allTime',
};
export function statsDuration(seconds: number, i18n: Translator): string {
  const value = Math.max(0, Math.floor(seconds));
  if (value < 60) return i18n.t('duration.compactSeconds', { seconds: i18n.number(value) });
  if (value < 3600) return i18n.t('duration.minutes', { minutes: i18n.number(Math.floor(value / 60)) });
  return i18n.t('duration.compactHoursMinutes', { hours: i18n.number(Math.floor(value / 3600)), minutes: i18n.number(Math.floor(value % 3600 / 60)) });
}
export function statsDateLabel(key: string, unit: string, i18n: Translator): string {
  if (unit === 'year') return i18n.number(Number(key.slice(0, 4)), { useGrouping: false });
  const date = new Date(`${key.slice(0, 10)}T12:00:00Z`);
  if (unit === 'hour') {
    const hour = Number(key.slice(11, 13));
    return i18n.time(Date.UTC(2000, 0, 1, hour || 0), { hour: 'numeric', timeZone: 'UTC' });
  }
  const safeDate = unit === 'month' ? new Date(`${key.slice(0, 7)}-01T12:00:00Z`) : date;
  return i18n.date(safeDate, { timeZone: 'UTC', ...(unit === 'month' ? { month: 'short', year: 'numeric' } as const : { month: 'short', day: 'numeric', year: 'numeric' } as const) });
}
export function statsPeriodLabel(data: StatsSummary, i18n: Translator): string {
  if (data.range === 'all_time') return i18n.t('stats.range.allTime');
  const first = new Date(`${data.startDate}T12:00:00Z`);
  if (data.range === 'day') return i18n.date(first, { dateStyle: 'long', timeZone: 'UTC' });
  if (data.range === 'month') return i18n.date(first, { month: 'long', year: 'numeric', timeZone: 'UTC' });
  if (data.range === 'year') return i18n.date(first, { year: 'numeric', timeZone: 'UTC' });
  return i18n.t('stats.period', { start: i18n.date(first, { month: 'short', day: 'numeric', timeZone: 'UTC' }), end: i18n.date(new Date(`${data.endDate}T12:00:00Z`), { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' }) });
}
export function statsPathTone(id: string, appearance?: (id: string) => PathAppearance) {
  return pathPalette[(appearance?.(id) ?? defaultPathAppearance(id)).color];
}

export type StatsCalendarUnit = 'day' | 'week' | 'month' | 'year';
export const statsCalendarUnits: readonly StatsCalendarUnit[] = ['day', 'week', 'month', 'year'];
export const statsCalendarKeys: Record<StatsCalendarUnit, MessageKey> = {
  day: 'stats.calendar.day', week: 'stats.calendar.week', month: 'stats.calendar.month', year: 'stats.calendar.year',
};
// These are historical calendar labels, not UTC instants. UTC arithmetic here
// preserves the dates already attributed by the server in each occurrence zone.
export function statsCalendarGroups(days: readonly { date: string; seconds: number }[], unit: StatsCalendarUnit, weekStartsOn = 1): { key: string; seconds: number }[] {
  const groups = new Map<string, number>();
  for (const day of days) {
    let key = day.date;
    if (unit === 'month') key = key.slice(0, 7);
    else if (unit === 'year') key = key.slice(0, 4);
    else if (unit === 'week') {
      const date = new Date(`${day.date}T12:00:00Z`);
      const weekday = date.getUTCDay() || 7;
      date.setUTCDate(date.getUTCDate() - ((weekday - weekStartsOn + 7) % 7));
      key = date.toISOString().slice(0, 10);
    }
    groups.set(key, (groups.get(key) ?? 0) + day.seconds);
  }
  return [...groups].map(([key, seconds]) => ({ key, seconds })).sort((a, b) => a.key.localeCompare(b.key));
}

// Layout historical labels without reinterpreting their occurrence time zones.
export function statsContributionWeeks(days: readonly { date: string; seconds: number }[], weekStartsOn = 1): ({ date: string; seconds: number } | null)[][] {
  if (!days.length) return [];
  const ordered = [...days].sort((a, b) => a.date.localeCompare(b.date));
  const values = new Map(ordered.map(day => [day.date, day.seconds]));
  const cursor = new Date(`${ordered[0]!.date}T12:00:00Z`);
  const last = ordered[ordered.length - 1]!.date;
  const offset = ((cursor.getUTCDay() || 7) - weekStartsOn + 7) % 7;
  const cells: ({ date: string; seconds: number } | null)[] = Array.from({ length: offset }, () => null);
  while (cursor.toISOString().slice(0, 10) <= last) {
    const date = cursor.toISOString().slice(0, 10);
    cells.push({ date, seconds: values.get(date) ?? 0 });
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  while (cells.length % 7) cells.push(null);
  return Array.from({ length: cells.length / 7 }, (_, index) => cells.slice(index * 7, index * 7 + 7));
}
export function statsContributionLevel(seconds: number, maximum: number): number {
  return seconds <= 0 ? 0 : Math.max(1, Math.min(4, Math.ceil(seconds / Math.max(1, maximum) * 4)));
}

export function statsWeekdayLabel(index: number, weekStartsOn: number, i18n: Translator) {
  return i18n.date(new Date(Date.UTC(2026, 0, 5 + weekStartsOn - 1 + index)), { weekday: 'short', timeZone: 'UTC' });
}
export function statsContributionMonthLabel(week: ReturnType<typeof statsContributionWeeks>[number], index: number, i18n: Translator) {
  const day = week.find(day => day && (index === 0 || day.date.endsWith('-01')));
  return day ? i18n.date(new Date(`${day.date}T12:00:00Z`), { month: 'short', timeZone: 'UTC' }) : '';
}
