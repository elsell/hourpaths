import type { Selection, Statistics, PathStatistics } from '../domain/statistics';
export interface StatisticsRepository {
  path(pathId: string, participantId: string, signal?: AbortSignal): Promise<PathStatistics>; load(selection: Selection, signal?: AbortSignal): Promise<Statistics> }
