export class StatisticsUnavailable extends Error {
  constructor(readonly retryable = true) { super('statistics_unavailable'); }
}
export const ranges = ['day', 'week', 'month', 'year', 'all_time'] as const;
export type Range = typeof ranges[number];
export type CalendarUnit = 'day' | 'week' | 'month' | 'year';
export interface Selection { range: Range; anchor?: string; pathIds: readonly string[] }
export interface CalendarDay { date: string; seconds: number }
export interface Bucket { key: string; seconds: number }
export interface Statistics {
  range: Range; anchor: string; firstDate: string; lastDate: string; previous: string | null; next: string | null;
  weekStartsOn: number; bucketUnit: 'hour' | 'day' | 'month' | 'year'; seconds: number;
  paths: readonly { id: string; name: string; archived: boolean }[];
  distribution: readonly { id: string; name: string; seconds: number }[];
  buckets: readonly Bucket[]; days: readonly CalendarDay[];
}
// These are historical date labels, not UTC instants. Calendar arithmetic must
// preserve the labels attributed by the server in each occurrence time zone.
export function contributionWeeks(days: readonly CalendarDay[], weekStartsOn: number, range?: { startDate: string; endDate: string }): (CalendarDay | null)[][] {
  if (!days.length && !range) return [];
  const ordered = [...days].sort((a, b) => a.date.localeCompare(b.date));
  const values = new Map(ordered.map(day => [day.date, day.seconds]));
  const cursor = new Date((range?.startDate ?? ordered[0].date) + 'T12:00:00Z');
  const end = range?.endDate ?? ordered[ordered.length - 1].date;
  const offset = ((cursor.getUTCDay() || 7) - weekStartsOn + 7) % 7;
  const cells: (CalendarDay | null)[] = Array.from({ length: offset }, () => null);
  while (cursor.toISOString().slice(0, 10) <= end) {
    const date = cursor.toISOString().slice(0, 10);
    cells.push({ date, seconds: values.get(date) ?? 0 });
    cursor.setUTCDate(cursor.getUTCDate() + 1);
  }
  while (cells.length % 7) cells.push(null);
  return Array.from({ length: cells.length / 7 }, (_, i) => cells.slice(i * 7, i * 7 + 7));
}
export function calendarGroups(days: readonly CalendarDay[], unit: CalendarUnit, weekStartsOn: number): Bucket[] {
  const groups = new Map<string, number>();
  for (const day of days) {
    let key = day.date;
    if (unit === 'month') key = key.slice(0, 7);
    if (unit === 'year') key = key.slice(0, 4);
    if (unit === 'week') {
      const date = new Date(day.date + 'T12:00:00Z');
      date.setUTCDate(date.getUTCDate() - ((date.getUTCDay() || 7) - weekStartsOn + 7) % 7);
      key = date.toISOString().slice(0, 10);
    }
    groups.set(key, (groups.get(key) ?? 0) + day.seconds);
  }
  return [...groups].map(([key, seconds]) => ({ key, seconds })).sort((a, b) => a.key.localeCompare(b.key));
}
export function contributionLevel(seconds: number, maximum: number): number {
  return seconds <= 0 ? 0 : Math.max(1, Math.min(4, Math.ceil(seconds / Math.max(1, maximum) * 4)));
}
