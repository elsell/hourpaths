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
export interface PathStatistics {
  totalSeconds: number; sessionCount: number; averageSeconds: number;
  weekStartsOn: number; firstDate: string; lastDate: string; days: readonly CalendarDay[];
}
