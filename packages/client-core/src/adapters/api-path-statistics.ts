import type { PathStatistics as PathStatisticsDTO } from '@hourpaths/api-client';
import type { PathStatistics } from '../path-statistics';

export function pathStatisticsFromAPI(dto: PathStatisticsDTO): PathStatistics {
  const integer = (value: number) => {
    if (!Number.isSafeInteger(value) || value < 0) throw new Error('invalid_path_statistics');
    return value;
  };
  const date = (value: string) => {
    const parsed = new Date(value + 'T12:00:00Z');
    if (!/^\d{4}-\d{2}-\d{2}$/.test(value) || !Number.isFinite(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== value) throw new Error('invalid_path_statistics');
    return value;
  };
  const firstDate = date(dto.startDate), lastDate = date(dto.endDate);
  if (lastDate < firstDate || !Number.isInteger(dto.weekStartsOn) || dto.weekStartsOn < 1 || dto.weekStartsOn > 7) throw new Error('invalid_path_statistics');
  return { totalSeconds: integer(dto.totalSeconds), sessionCount: integer(dto.sessionCount), averageSeconds: integer(dto.averageSeconds), weekStartsOn: dto.weekStartsOn, firstDate, lastDate,
    days: dto.calendar.map(day => ({ date: date(day.date), seconds: integer(day.seconds) })) };
}
