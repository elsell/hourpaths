import { createSessionApiClient, type StatsSummary } from '@hourpaths/api-client';
import { ranges, StatisticsUnavailable, type Range, type Statistics } from '../domain/statistics';
import type { StatisticsRepository } from '../ports/statistics-repository';
function date(value: string): string {
  const parsed = new Date(value + 'T12:00:00Z');
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || !Number.isFinite(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== value) throw new StatisticsUnavailable();
  return value;
}
function seconds(value: number): number {
  if (!Number.isSafeInteger(value) || value < 0) throw new StatisticsUnavailable();
  return value;
}
function bucketKey(value: string, unit: string): string {
  if (unit === 'day') date(value);
  else if (unit === 'month') date(value + '-01');
  else if (unit === 'year') date(value + '-01-01');
  else if (unit === 'hour') {
    if (!/^\d{4}-\d{2}-\d{2}T\d{2}$/.test(value) || Number(value.slice(11)) > 23) throw new StatisticsUnavailable();
    date(value.slice(0, 10));
  }
  return value;
}
export function statisticsFromAPI(dto: StatsSummary): Statistics {
  if (!ranges.includes(dto.range as Range) || !['hour', 'day', 'month', 'year'].includes(dto.bucketUnit) || !Number.isInteger(dto.weekStartsOn) || dto.weekStartsOn < 1 || dto.weekStartsOn > 7) throw new StatisticsUnavailable();
  const firstDate = date(dto.startDate), lastDate = date(dto.endDate);
  if (lastDate < firstDate) throw new StatisticsUnavailable();
  return {
    range: dto.range as Range, anchor: date(dto.anchor), firstDate, lastDate,
    previous: dto.previousAnchor ? date(dto.previousAnchor) : null, next: dto.nextAnchor ? date(dto.nextAnchor) : null,
    weekStartsOn: dto.weekStartsOn, bucketUnit: dto.bucketUnit as Statistics['bucketUnit'], seconds: seconds(dto.totalSeconds),
    paths: dto.availablePaths.map(path => ({ id: path.id, name: path.name, archived: path.archived })),
    distribution: dto.distribution.map(path => ({ id: path.pathId, name: path.name, seconds: seconds(path.seconds) })),
    buckets: dto.buckets.map(bucket => ({ key: bucketKey(bucket.key, dto.bucketUnit), seconds: seconds(bucket.seconds) })),
    days: dto.calendar.map(day => ({ date: date(day.date), seconds: seconds(day.seconds) })),
  };
}
export function apiStatisticsRepository(baseURL: string, token: () => string | null, rejected: () => void): StatisticsRepository {
  return { async load(selection, signal) {
    try {
    const result = await createSessionApiClient(baseURL, token, signal).stats({ range: selection.range, anchor: selection.anchor, pathIds: selection.pathIds.length ? selection.pathIds.join(',') : undefined });
    if (result.response.status === 401) rejected();
    if (!result.response.ok || !result.data) throw new StatisticsUnavailable(result.response.status === 429 || result.response.status >= 500);
    return statisticsFromAPI(result.data.data);
    } catch (error) {
      if (signal?.aborted || error instanceof StatisticsUnavailable) throw error;
      throw new StatisticsUnavailable(true);
    }
  } };
}
